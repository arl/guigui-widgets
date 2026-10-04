package shortcut

import (
	"testing"

	"github.com/hajimehoshi/ebiten/v2"
)

func TestParseRoundTrip(t *testing.T) {
	t.Parallel()
	for k := ebiten.Key(0); k <= ebiten.KeyMax; k++ {
		if !bindable(k) {
			continue
		}
		want := Key(k)
		got, err := Parse(want.String())
		if err != nil {
			t.Fatalf("Parse(%q): %v", want, err)
		}
		if got != want {
			t.Fatalf("Parse(%q) = %+v, want %+v", want, got, want)
		}
	}
}

func TestParseModifiers(t *testing.T) {
	t.Parallel()
	base := Key(ebiten.KeyO)
	tests := []struct {
		name string
		in   string
		want Chord
	}{
		{name: "bare", in: "o", want: base},
		{name: "canonical", in: "O", want: base},
		{name: "ctrl", in: "Ctrl+O", want: Chord{Key: ebiten.KeyO, Ctrl: true, valid: true}},
		{name: "alias order", in: "shift+cmd+o", want: Chord{Key: ebiten.KeyO, Shift: true, Meta: true, valid: true}},
		{name: "spaces", in: " Ctrl + Alt + O ", want: Chord{Key: ebiten.KeyO, Ctrl: true, Alt: true, valid: true}},
		{name: "control word", in: "control+option+O", want: Chord{Key: ebiten.KeyO, Ctrl: true, Alt: true, valid: true}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, err := Parse(tt.in)
			if err != nil {
				t.Fatalf("Parse(%q): %v", tt.in, err)
			}
			if got != tt.want {
				t.Fatalf("Parse(%q) = %+v, want %+v", tt.in, got, tt.want)
			}
			if got.String() != tt.want.String() {
				t.Fatalf("String = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseRejects(t *testing.T) {
	t.Parallel()
	for _, in := range []string{"", "-", "Shift", "Ctrl", "ControlLeft", "NotAKey", "Ctrl+", "+O", "O+Ctrl"} {
		if _, err := Parse(in); err == nil {
			t.Errorf("Parse(%q) succeeded, want error", in)
		}
	}
}

func TestKeyAIsBindable(t *testing.T) {
	t.Parallel()
	// ebiten.KeyA is numeric zero. The zero Chord must not mean KeyA.
	if (Chord{}).String() != "" {
		t.Fatal("zero Chord string is not empty")
	}
	got := Key(ebiten.KeyA)
	if got.String() != "A" || !got.valid {
		t.Fatalf("Key(A) = %+v", got)
	}
}

func TestMatchIsExact(t *testing.T) {
	t.Parallel()
	c, err := Parse("Ctrl+O")
	if err != nil {
		t.Fatal(err)
	}
	if !c.match(ebiten.KeyO, true, false, false, false) {
		t.Fatal("Ctrl+O did not match")
	}
	if c.match(ebiten.KeyO, true, false, true, false) {
		t.Fatal("Ctrl+Shift+O matched Ctrl+O")
	}
	if c.match(ebiten.KeyO, false, false, false, false) {
		t.Fatal("bare O matched Ctrl+O")
	}
	if c.match(ebiten.KeyP, true, false, false, false) {
		t.Fatal("Ctrl+P matched Ctrl+O")
	}
}

type mem map[string]string

func (m *mem) Get(id string) (string, bool) {
	if m == nil || *m == nil {
		return "", false
	}
	v, ok := (*m)[id]
	return v, ok
}

func (m *mem) Set(id, value string) {
	if value == "" {
		if m == nil || *m == nil {
			return
		}
		delete(*m, id)
		if len(*m) == 0 {
			*m = nil
		}
		return
	}
	if *m == nil {
		*m = mem{}
	}
	(*m)[id] = value
}

func testActions() []Action {
	return []Action{
		{ID: "shader.prev", Label: "Previous shader", Default: Key(ebiten.KeyO)},
		{ID: "shader.next", Label: "Next shader", Default: Key(ebiten.KeyP)},
		{ID: "emulation.pause", Label: "Pause", Default: Key(ebiten.KeySpace)},
	}
}

func TestEffectiveFallsBack(t *testing.T) {
	t.Parallel()
	actions := testActions()
	var store mem
	c, ok := effective(actions[1], &store)
	if !ok || c.String() != "P" {
		t.Fatalf("missing override = %s, %v", c, ok)
	}
	store.Set("shader.next", "not-a-key")
	c, ok = effective(actions[1], &store)
	if !ok || c.String() != "P" {
		t.Fatalf("invalid override = %s, %v", c, ok)
	}
	store.Set("shader.next", Unbound)
	if _, ok = effective(actions[1], &store); ok {
		t.Fatal("unbound action is still effective")
	}
	store.Set("shader.next", "ctrl+bracketright")
	c, ok = effective(actions[1], &store)
	if !ok || c.String() != "Ctrl+BracketRight" {
		t.Fatalf("override = %s, %v", c, ok)
	}
}

func TestAssignStealsAndResetRestores(t *testing.T) {
	prev := Actions()
	t.Cleanup(func() { Set(prev) })
	Set(testActions())

	var store mem
	next, err := Parse("O")
	if err != nil {
		t.Fatal(err)
	}
	Assign(&store, "shader.next", next)
	if got := Display(&store, "shader.next"); got != "O" {
		t.Fatalf("next display = %q", got)
	}
	if got := Display(&store, "shader.prev"); got != "Not set" {
		t.Fatalf("prev display = %q, want stolen", got)
	}
	if store["shader.prev"] != Unbound {
		t.Fatalf("stolen stored value = %q", store["shader.prev"])
	}

	Reset(&store, "shader.prev")
	if Display(&store, "shader.prev") != "O" {
		t.Fatalf("prev after reset = %q", Display(&store, "shader.prev"))
	}
	if Display(&store, "shader.next") != "Not set" {
		t.Fatalf("next after reset = %q, want the default key taken back", Display(&store, "shader.next"))
	}
	var ran []string
	actions := testActions()
	actions[0].Run = func() { ran = append(ran, "prev") }
	actions[1].Run = func() { ran = append(ran, "next") }
	if !dispatch(actions, &store, func(c Chord) bool { return c.String() == "O" }) {
		t.Fatal("dispatch missed O")
	}
	if len(ran) != 1 || ran[0] != "prev" {
		t.Fatalf("ran = %v, want prev", ran)
	}

	Assign(&store, "shader.next", Key(ebiten.KeyP))
	if _, ok := store["shader.next"]; ok {
		t.Fatalf("default chord stored an override: %v", store)
	}
	if Display(&store, "shader.next") != "P" {
		t.Fatalf("display after reset = %q", Display(&store, "shader.next"))
	}
}

func TestDispatchUnknownIDIgnored(t *testing.T) {
	t.Parallel()
	store := mem{"gone": "F1"}
	if dispatch(testActions(), &store, func(Chord) bool { return true }) != true {
		t.Fatal("expected the first real action")
	}
}

func TestDispatchSkipsTakenChord(t *testing.T) {
	t.Parallel()
	actions := testActions()
	var ran string
	actions[0].Run = func() { ran = "prev" }
	actions[1].Run = func() { ran = "next" }
	store := mem{"shader.next": "O"}
	dispatch(actions, &store, func(c Chord) bool { return c.String() == "O" })
	if ran != "prev" {
		t.Fatalf("ran = %q, want prev (earlier action keeps the chord)", ran)
	}
}

func TestSetRejectsDuplicate(t *testing.T) {
	prev := Actions()
	t.Cleanup(func() { Set(prev) })
	defer func() {
		if recover() == nil {
			t.Fatal("expected panic")
		}
	}()
	Set([]Action{{ID: "a", Default: Key(ebiten.KeyA)}, {ID: "a", Default: Key(ebiten.KeyB)}})
}
