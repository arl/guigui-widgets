// Package shortcut provides keyboard shortcut management for guigui/ebiten
// application.
//
// Register every action with [Set]. The chord on the action is the default. A
// [Store] holds the user's replacements: a missing id keeps the default, and
// the value [Unbound] clears the chord. [Dispatch] runs the first matching
// action. If two actions still share a chord, the earlier one wins.
package shortcut

import (
	"fmt"
	"strings"
	"sync"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// Unbound is the stored chord text that means "this action has no key".
const Unbound = "-"

// Chord is one key plus the modifiers that must be held with it.
// The zero Chord is not a binding. [ebiten.KeyA] is numeric zero, so a
// missing valid flag is what marks an empty chord.
type Chord struct {
	Key   ebiten.Key
	Ctrl  bool
	Alt   bool
	Shift bool
	Meta  bool
	valid bool
}

// Key returns a chord for a single key and no modifiers.
func Key(k ebiten.Key) Chord {
	if !bindable(k) {
		panic(fmt.Sprintf("shortcut: %s cannot be a chord key", k))
	}
	return Chord{Key: k, valid: true}
}

// FromPress builds a chord from a key and the modifiers held with it.
// Modifier keys themselves are not chords.
func FromPress(key ebiten.Key, ctrl, alt, shift, meta bool) (Chord, bool) {
	if !bindable(key) {
		return Chord{}, false
	}
	return Chord{Key: key, Ctrl: ctrl, Alt: alt, Shift: shift, Meta: meta, valid: true}, true
}

// Parse reads a chord such as "O" or "Ctrl+Shift+O".
// Modifier words are Ctrl/Control, Alt/Option, Shift, and Meta/Cmd/Super.
// The key is an ebiten key name, case-insensitive. Modifier keys cannot be the key.
func Parse(s string) (Chord, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == Unbound {
		return Chord{}, fmt.Errorf("shortcut: empty chord")
	}
	parts := strings.Split(s, "+")
	var c Chord
	for i, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			return Chord{}, fmt.Errorf("shortcut: empty chord part")
		}
		if i < len(parts)-1 {
			switch strings.ToLower(part) {
			case "ctrl", "control":
				c.Ctrl = true
			case "alt", "opt", "option":
				c.Alt = true
			case "shift":
				c.Shift = true
			case "meta", "cmd", "command", "super", "win":
				c.Meta = true
			default:
				return Chord{}, fmt.Errorf("shortcut: unknown modifier %q", part)
			}
			continue
		}
		key, ok := keyByName()[strings.ToLower(part)]
		if !ok || !bindable(key) {
			return Chord{}, fmt.Errorf("shortcut: unknown key %q", part)
		}
		c.Key = key
		c.valid = true
	}
	if !c.valid {
		return Chord{}, fmt.Errorf("shortcut: missing key")
	}
	return c, nil
}

// String returns the canonical chord text. An invalid chord is empty.
func (c Chord) String() string {
	if !c.valid {
		return ""
	}
	var b strings.Builder
	writeMod := func(on bool, name string) {
		if !on {
			return
		}
		if b.Len() > 0 {
			b.WriteByte('+')
		}
		b.WriteString(name)
	}
	writeMod(c.Ctrl, "Ctrl")
	writeMod(c.Alt, "Alt")
	writeMod(c.Shift, "Shift")
	writeMod(c.Meta, "Meta")
	if b.Len() > 0 {
		b.WriteByte('+')
	}
	b.WriteString(c.Key.String())
	return b.String()
}

func (c Chord) pressed() bool {
	if !c.valid || !inpututil.IsKeyJustPressed(c.Key) {
		return false
	}
	return c.match(
		c.Key,
		ebiten.IsKeyPressed(ebiten.KeyControl),
		ebiten.IsKeyPressed(ebiten.KeyAlt),
		ebiten.IsKeyPressed(ebiten.KeyShift),
		ebiten.IsKeyPressed(ebiten.KeyMeta),
	)
}

func (c Chord) match(key ebiten.Key, ctrl, alt, shift, meta bool) bool {
	return c.valid && c.Key == key && c.Ctrl == ctrl && c.Alt == alt && c.Shift == shift && c.Meta == meta
}

func bindable(key ebiten.Key) bool {
	switch key {
	case ebiten.KeyAlt, ebiten.KeyAltLeft, ebiten.KeyAltRight,
		ebiten.KeyControl, ebiten.KeyControlLeft, ebiten.KeyControlRight,
		ebiten.KeyMeta, ebiten.KeyMetaLeft, ebiten.KeyMetaRight,
		ebiten.KeyShift, ebiten.KeyShiftLeft, ebiten.KeyShiftRight:
		return false
	default:
		return key >= 0 && key <= ebiten.KeyMax && key.String() != ""
	}
}

var keyByName = sync.OnceValue(func() map[string]ebiten.Key {
	m := make(map[string]ebiten.Key)
	for k := ebiten.Key(0); k <= ebiten.KeyMax; k++ {
		name := k.String()
		if name == "" {
			continue
		}
		m[strings.ToLower(name)] = k
	}
	return m
})

// Action is one command the user can bind to a chord.
type Action struct {
	ID      string
	Label   string
	Default Chord
	Enabled func() bool
	Run     func()
}

// Store is the user's chord overrides, keyed by action id.
// Set(id, "") deletes the override. Set(id, [Unbound]) clears the chord.
type Store interface {
	Get(id string) (string, bool)
	Set(id, value string)
}

var current []Action

// Set replaces the process-wide action list. Call it once at startup.
// Duplicate ids, empty ids, and two actions with the same default chord panic.
func Set(actions []Action) {
	seenID := map[string]bool{}
	seenChord := map[Chord]string{}
	for _, a := range actions {
		if a.ID == "" || seenID[a.ID] {
			panic(fmt.Sprintf("shortcut: duplicate or empty action id %q", a.ID))
		}
		seenID[a.ID] = true
		if !a.Default.valid {
			continue
		}
		if prev, ok := seenChord[a.Default]; ok {
			panic(fmt.Sprintf("shortcut: %s and %s share the default chord %s", a.ID, prev, a.Default))
		}
		seenChord[a.Default] = a.ID
	}
	current = append([]Action(nil), actions...)
}

// Actions returns the registered actions.
func Actions() []Action {
	return current
}

// Assign stores chord for id. Another action that already uses chord becomes unbound.
// A chord equal to the action default removes the override.
func Assign(store Store, id string, chord Chord) {
	if store == nil || !chord.valid {
		return
	}
	actions := Actions()
	target, ok := find(actions, id)
	if !ok {
		return
	}
	for _, a := range actions {
		if a.ID == id {
			continue
		}
		got, bound := effective(a, store)
		if bound && got == chord {
			store.Set(a.ID, Unbound)
		}
	}
	if target.Default == chord {
		store.Set(id, "")
		return
	}
	store.Set(id, chord.String())
}

// Reset gives id its default chord again.
// If another action uses that chord, Reset takes it back.
func Reset(store Store, id string) {
	if store == nil {
		return
	}
	a, ok := find(Actions(), id)
	if !ok {
		return
	}
	if !a.Default.valid {
		store.Set(id, Unbound)
		return
	}
	Assign(store, id, a.Default)
}

// Unbind clears the chord for id. The action does not fall back to its default.
func Unbind(store Store, id string) {
	if store == nil {
		return
	}
	store.Set(id, Unbound)
}

// Display is the chord text shown for id. A cleared action is "Not set".
// Text that does not parse is shown unchanged.
func Display(store Store, id string) string {
	return display(Actions(), store, id)
}

func display(actions []Action, store Store, id string) string {
	a, ok := find(actions, id)
	raw, has := "", false
	if store != nil {
		raw, has = store.Get(id)
	}
	if !has {
		if !ok || !a.Default.valid {
			return "Not set"
		}
		return a.Default.String()
	}
	if raw == Unbound {
		return "Not set"
	}
	c, err := Parse(raw)
	if err != nil {
		return raw
	}
	return c.String()
}

// Dispatch runs the first action whose chord was just pressed.
// It returns true when an action ran.
func Dispatch(store Store) bool {
	if store == nil {
		return false
	}
	return dispatch(Actions(), store, Chord.pressed)
}

func dispatch(actions []Action, store Store, pressed func(Chord) bool) bool {
	seen := map[Chord]bool{}
	for _, a := range actions {
		if a.Enabled != nil && !a.Enabled() {
			continue
		}
		c, ok := effective(a, store)
		if !ok || seen[c] {
			continue
		}
		seen[c] = true
		if !pressed(c) {
			continue
		}
		if a.Run != nil {
			a.Run()
		}
		return true
	}
	return false
}

func effective(a Action, store Store) (Chord, bool) {
	raw, has := "", false
	if store != nil {
		raw, has = store.Get(a.ID)
	}
	if !has {
		return a.Default, a.Default.valid
	}
	if raw == Unbound {
		return Chord{}, false
	}
	c, err := Parse(raw)
	if err != nil {
		return a.Default, a.Default.valid
	}
	return c, true
}

func find(actions []Action, id string) (Action, bool) {
	for _, a := range actions {
		if a.ID == id {
			return a, true
		}
	}
	return Action{}, false
}
