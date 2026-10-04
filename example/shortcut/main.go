// Command shortcut demonstrates shortcut registration, dispatch, and editing.
package main

import (
	"fmt"
	"image"
	"os"
	"slices"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"

	"github.com/arl/guigui-widgets/shortcut"
)

type bindings map[string]string

func (b *bindings) Get(id string) (string, bool) {
	if b == nil || *b == nil {
		return "", false
	}
	value, ok := (*b)[id]
	return value, ok
}

func (b *bindings) Set(id, value string) {
	if b == nil {
		return
	}
	if value == "" {
		delete(*b, id)
		return
	}
	if *b == nil {
		*b = bindings{}
	}
	(*b)[id] = value
}

type root struct {
	guigui.DefaultWidget

	bindings bindings
	message  string

	background basicwidget.Background
	title      basicwidget.Text
	intro      basicwidget.Text
	status     basicwidget.Text
	editor     shortcut.Editor

	layoutItems []guigui.LinearLayoutItem
}

func (r *root) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	context.SetButtonInputReceptive(r, true)
	adder.AddWidget(&r.background)
	adder.AddWidget(&r.title)
	adder.AddWidget(&r.intro)
	adder.AddWidget(&r.status)
	adder.AddWidget(&r.editor)

	r.title.SetValue("Keyboard shortcuts")
	var titleStyle basicwidget.TextStyle
	titleStyle.SetBold(true)
	r.title.SetBaseStyle(&titleStyle)
	r.title.SetScale(1.5)

	r.intro.SetValue("Press a registered shortcut, or select a row to change its binding.")
	r.intro.SetWrapMode(basicwidget.WrapModeNormal)
	r.status.SetValue(r.message)
	r.editor.Bind(&r.bindings)
	return nil
}

func (r *root) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	w.WriteString(r.message)
}

func (r *root) HandleButtonInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if shortcut.Dispatch(&r.bindings) {
		return guigui.HandleInputByWidget(r)
	}
	return guigui.HandleInputResult{}
}

func (r *root) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&r.background, widgetBounds.Bounds())

	u := basicwidget.UnitSize(context)
	r.layoutItems = slices.Delete(r.layoutItems, 0, len(r.layoutItems))
	r.layoutItems = append(r.layoutItems,
		guigui.LinearLayoutItem{Widget: &r.title},
		guigui.LinearLayoutItem{Widget: &r.intro},
		guigui.LinearLayoutItem{Widget: &r.status},
		guigui.LinearLayoutItem{Widget: &r.editor},
	)
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     r.layoutItems,
		Gap:       u / 2,
		Padding:   guigui.Padding{Start: u / 2, Top: u / 2, End: u / 2, Bottom: u / 2},
	}).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func main() {
	app := &root{message: "Try H or C."}
	shortcut.Set([]shortcut.Action{
		{
			ID:      "hello",
			Label:   "Say hello",
			Default: shortcut.Key(ebiten.KeyH),
			Run:     func() { app.message = "Hello!" },
		},
		{
			ID:      "count",
			Label:   "Count activations",
			Default: shortcut.Key(ebiten.KeyC),
			Run: func() {
				app.message += " +1"
			},
		},
	})

	if err := guigui.Run(app, &guigui.RunOptions{
		Title:         "Shortcuts",
		WindowSize:    image.Pt(720, 480),
		WindowMinSize: image.Pt(560, 360),
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
