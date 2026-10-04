package shortcut

import (
	"image"
	"slices"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// Editor is a widget for the management of keyboard shortcuts. It lists
// registered shortcut actions and allows the user to capture new chords.
type Editor struct {
	guigui.DefaultWidget

	store Store

	hint    basicwidget.Text
	table   basicwidget.Table[string]
	capture basicwidget.Popup
	page    shortcutCapture

	capturing string

	onRow          func(id string)
	onCaptureClose func(context *guigui.Context, reason basicwidget.PopupCloseReason)

	layoutItems []guigui.LinearLayoutItem
}

func (e *Editor) Bind(v any) {
	store, ok := v.(Store)
	if !ok {
		e.store = nil
		return
	}
	e.store = store
}

func (e *Editor) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	w.WriteString(e.capturing)
	w.WriteBool(e.capture.IsOpen())
	for _, a := range Actions() {
		w.WriteString(a.ID)
		raw := ""
		if e.store != nil {
			raw, _ = e.store.Get(a.ID)
		}
		w.WriteString(raw)
	}
}

func (e *Editor) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	if e.store == nil {
		return nil
	}
	adder.AddWidget(&e.hint)
	adder.AddWidget(&e.table)
	adder.AddWidget(&e.capture)

	e.hint.SetValue("Select a shortcut, then press a key.")
	e.hint.SetOpacity(0.7)

	actions := Actions()
	u := basicwidget.UnitSize(context)
	e.table.SetColumns([]basicwidget.TableColumn{
		{HeaderText: "Action", Width: guigui.FlexibleSize(1), MinWidth: 4 * u},
		{HeaderText: "Key", Width: guigui.FlexibleSize(2), MinWidth: 6 * u},
	})
	e.table.SetItemHeight(u)
	e.table.SetUnfocusedSelectionVisible(true)
	rows := make([]basicwidget.TableRow[string], 0, len(actions))
	for _, a := range actions {
		rows = append(rows, basicwidget.TableRow[string]{
			Value: a.ID,
			Cells: []basicwidget.TableCell{
				{Text: a.Label},
				{Text: Display(e.store, a.ID)},
			},
		})
	}
	e.table.SetItems(rows)
	if e.onRow == nil {
		e.onRow = func(id string) {
			if e.capture.IsOpen() || e.capturing != "" {
				return
			}
			e.table.SelectItemByIndex(-1)
			e.openCapture(id)
		}
	}
	e.table.OnItemSelected(func(context *guigui.Context, index int) {
		item, ok := e.table.ItemByIndex(index)
		if !ok {
			return
		}
		e.onRow(item.Value)
	})

	e.capture.SetContent(&e.page)
	e.capture.SetModal(true)
	e.capture.SetBackgroundDark(true)
	e.capture.SetCloseByClickingOutside(true)
	e.capture.SetAnimated(false)
	e.page.label = e.captureLabel()
	e.page.def = e.captureDefault()
	e.page.onAssign = e.assign
	e.page.onReset = e.reset
	e.page.onClear = e.clear
	e.page.onCancel = e.cancel
	if e.onCaptureClose == nil {
		e.onCaptureClose = func(context *guigui.Context, reason basicwidget.PopupCloseReason) {
			if reason != basicwidget.PopupCloseReasonFuncCall {
				e.capturing = ""
			}
		}
	}
	e.capture.OnClose(e.onCaptureClose)
	return nil
}

func (e *Editor) captureLabel() string {
	for _, a := range Actions() {
		if a.ID == e.capturing {
			return a.Label
		}
	}
	return e.capturing
}

func (e *Editor) captureDefault() string {
	for _, a := range Actions() {
		if a.ID == e.capturing {
			if text := a.Default.String(); text != "" {
				return text
			}
			return "none"
		}
	}
	return ""
}

func (e *Editor) openCapture(id string) {
	e.capturing = id
	e.page.label = e.captureLabel()
	e.page.def = e.captureDefault()
	e.capture.SetOpen(true)
}

func (e *Editor) assign(chord Chord) {
	if e.store == nil || e.capturing == "" {
		return
	}
	Assign(e.store, e.capturing, chord)
	e.capturing = ""
	e.capture.SetOpen(false)
}

func (e *Editor) reset() {
	if e.store == nil || e.capturing == "" {
		return
	}
	Reset(e.store, e.capturing)
	e.capturing = ""
	e.capture.SetOpen(false)
}

func (e *Editor) clear() {
	if e.store == nil || e.capturing == "" {
		return
	}
	Unbind(e.store, e.capturing)
	e.capturing = ""
	e.capture.SetOpen(false)
}

func (e *Editor) cancel() {
	e.capturing = ""
	e.capture.SetOpen(false)
}

func (e *Editor) layout(context *guigui.Context) guigui.LinearLayout {
	u := basicwidget.UnitSize(context)
	tableH := u*(1+len(Actions())) + 2*basicwidget.RoundedCornerRadius(context)
	e.layoutItems = slices.Delete(e.layoutItems, 0, len(e.layoutItems))
	e.layoutItems = append(
		e.layoutItems,
		guigui.LinearLayoutItem{Widget: &e.hint},
		guigui.LinearLayoutItem{Widget: &e.table, Size: guigui.FixedSize(tableH)},
	)
	return guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Gap:       u / 2,
		Items:     e.layoutItems,
		Padding:   guigui.Padding{Start: u / 4, Top: u / 4, End: u / 4, Bottom: u / 4},
	}
}

func (e *Editor) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	b := widgetBounds.Bounds()
	e.layout(context).LayoutWidgets(context, b, layouter)
	u := basicwidget.UnitSize(context)
	app := context.AppBounds()
	w, h := 16*u, 6*u
	x := app.Min.X + (app.Dx()-w)/2
	y := app.Min.Y + (app.Dy()-h)/2
	e.capture.SetBackgroundBounds(app)
	layouter.LayoutWidget(&e.capture, image.Rect(x, y, x+w, y+h))
}

func (e *Editor) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return e.layout(context).Measure(context, constraints)
}

type shortcutCapture struct {
	guigui.DefaultWidget

	title   basicwidget.Text
	hint    basicwidget.Text
	defBtn  basicwidget.Button
	noneBtn basicwidget.Button

	label string
	def   string

	onAssign func(Chord)
	onReset  func()
	onClear  func()
	onCancel func()

	onDefault func(context *guigui.Context)
	onNone    func(context *guigui.Context)

	rowItems []guigui.LinearLayoutItem
}

func (c *shortcutCapture) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	context.SetButtonInputReceptive(c, true)
	adder.AddWidget(&c.title)
	adder.AddWidget(&c.hint)
	adder.AddWidget(&c.defBtn)
	adder.AddWidget(&c.noneBtn)

	label := c.label
	if label == "" {
		label = "shortcut"
	}
	c.title.SetValue("Press a key for " + label)
	c.title.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
	c.title.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	var style basicwidget.TextStyle
	style.SetBold(true)
	c.title.SetBaseStyle(&style)

	def := c.def
	if def == "" {
		def = "none"
	}
	c.hint.SetValue("Default is " + def + ". Escape cancels.")
	c.hint.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
	c.hint.SetOpacity(0.7)

	c.defBtn.SetText("Default")
	c.noneBtn.SetText("None")
	if c.onDefault == nil {
		c.onDefault = func(context *guigui.Context) {
			if c.onReset != nil {
				c.onReset()
			}
		}
	}
	if c.onNone == nil {
		c.onNone = func(context *guigui.Context) {
			if c.onClear != nil {
				c.onClear()
			}
		}
	}
	c.defBtn.OnDown(c.onDefault)
	c.noneBtn.OnDown(c.onNone)
	return nil
}

func (c *shortcutCapture) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	c.rowItems = slices.Delete(c.rowItems, 0, len(c.rowItems))
	c.rowItems = append(
		c.rowItems,
		guigui.LinearLayoutItem{Widget: &c.defBtn, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &c.noneBtn, Size: guigui.FlexibleSize(1)},
	)
	row := &guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Gap:       u / 2,
		Items:     c.rowItems,
	}
	guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Gap:       u / 4,
		Padding:   guigui.Padding{Start: u / 2, Top: u / 2, End: u / 2, Bottom: u / 2},
		Items: []guigui.LinearLayoutItem{
			{Widget: &c.title, Size: guigui.FlexibleSize(1)},
			{Widget: &c.hint},
			{Layout: row, Size: guigui.FixedSize(u)},
		},
	}.LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func (c *shortcutCapture) HandleButtonInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if c.onCancel != nil {
			c.onCancel()
		}
		return guigui.HandleInputByWidget(c)
	}
	keys := inpututil.AppendJustPressedKeys(nil)
	for _, key := range keys {
		if key == ebiten.KeyEscape {
			continue
		}
		chord, ok := FromPress(
			key,
			ebiten.IsKeyPressed(ebiten.KeyControl),
			ebiten.IsKeyPressed(ebiten.KeyAlt),
			ebiten.IsKeyPressed(ebiten.KeyShift),
			ebiten.IsKeyPressed(ebiten.KeyMeta),
		)
		if !ok {
			continue
		}
		if c.onAssign != nil {
			c.onAssign(chord)
		}
		return guigui.HandleInputByWidget(c)
	}
	return guigui.AbortHandlingInputByWidget(c)
}
