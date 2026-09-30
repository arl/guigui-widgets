package docked

import (
	"image"
	"image/color"
	"slices"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// Pane is a data-only descriptor for a single collapsible panel within a
// PaneView. It is not a widget — the PaneView owns the rendering and input.
type Pane struct {
	Title       string
	Content     guigui.Widget
	Expanded    bool
	MinBodySize int
	MaxBodySize int
}

func NewPane(title string, content guigui.Widget) *Pane {
	return &Pane{
		Title:       title,
		Content:     content,
		Expanded:    true,
		MinBodySize: 50,
		MaxBodySize: 10000,
	}
}

type paneHeader struct {
	guigui.DefaultWidget

	pane  *Pane
	label basicwidget.Text
}

func (h *paneHeader) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&h.label)
	h.label.SetValue(h.pane.Title)
	h.label.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	h.label.SetHorizontalAlign(basicwidget.HorizontalAlignStart)
	return nil
}

func (h *paneHeader) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	// Leave room for the expand/collapse indicator on the left.
	b := widgetBounds.Bounds()
	b.Min.X += u
	layouter.LayoutWidget(&h.label, b)
}

func (h *paneHeader) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) && widgetBounds.IsHitAtCursor() {
		h.pane.Expanded = !h.pane.Expanded
		guigui.RequestRebuild()
		return guigui.HandleInputByWidget(h)
	}
	return guigui.HandleInputResult{}
}

func (h *paneHeader) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	return ebiten.CursorShapePointer, true
}

func (h *paneHeader) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	fixedW, _ := constraints.FixedWidth()
	u := basicwidget.UnitSize(context)
	return image.Pt(fixedW, paneHeaderHeight(u))
}

func (h *paneHeader) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	b := widgetBounds.Bounds()
	u := basicwidget.UnitSize(context)

	var bg color.RGBA
	if context.ColorMode() == ebiten.ColorModeLight {
		bg = color.RGBA{0xe0, 0xe0, 0xe0, 0xff}
	} else {
		bg = color.RGBA{0x30, 0x30, 0x30, 0xff}
	}
	vector.FillRect(dst, float32(b.Min.X), float32(b.Min.Y), float32(b.Dx()), float32(b.Dy()), bg, false)

	// Separator at the bottom of the header.
	var sep color.RGBA
	if context.ColorMode() == ebiten.ColorModeLight {
		sep = color.RGBA{0xaa, 0xaa, 0xaa, 0xff}
	} else {
		sep = color.RGBA{0x55, 0x55, 0x55, 0xff}
	}
	vector.StrokeLine(dst, float32(b.Min.X), float32(b.Max.Y), float32(b.Max.X), float32(b.Max.Y), 1, sep, false)

	// Expand/collapse indicator: a small triangle.
	cx := b.Min.X + u/2
	cy := b.Min.Y + b.Dy()/2
	var arrowClr color.RGBA
	if context.ColorMode() == ebiten.ColorModeLight {
		arrowClr = color.RGBA{0x55, 0x55, 0x55, 0xff}
	} else {
		arrowClr = color.RGBA{0xaa, 0xaa, 0xaa, 0xff}
	}
	if h.pane.Expanded {
		// "▼" — down-pointing triangle (filled rect for simplicity).
		r := float32(u / 10)
		vector.FillRect(dst, float32(cx)-r, float32(cy)-r, r*2, r*2, arrowClr, false)
	} else {
		// "▶" — right-pointing triangle.
		r := float32(u / 10)
		ax, ay := float32(cx)-r, float32(cy)-r
		bx, by := float32(cx)+r, float32(cy)
		cx2, cy2 := float32(cx)-r, float32(cy)+r
		vector.StrokeLine(dst, ax, ay, bx, by, 1.5, arrowClr, false)
		vector.StrokeLine(dst, cx2, cy2, bx, by, 1.5, arrowClr, false)
		vector.StrokeLine(dst, ax, ay, cx2, cy2, 1.5, arrowClr, false)
	}
}

// PaneView is a vertical stack of collapsible panes with headers, similar to
// dockview's PaneView. Each pane has a header that is always visible; clicking
// it toggles the body content.
type PaneView struct {
	guigui.DefaultWidget

	panes []*Pane

	headers     guigui.WidgetSlice[*paneHeader]
	layoutItems []guigui.LinearLayoutItem
}

func paneHeaderHeight(u int) int { return max(24, u*3/4) }

func (pv *PaneView) Len() int { return len(pv.panes) }

func (pv *PaneView) AddPane(pane *Pane) {
	pv.panes = append(pv.panes, pane)
}

func (pv *PaneView) RemovePane(index int) *Pane {
	if index < 0 || index >= len(pv.panes) {
		return nil
	}
	pane := pv.panes[index]
	pv.panes = append(pv.panes[:index], pv.panes[index+1:]...)
	return pane
}

func (pv *PaneView) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	pv.headers.SetLen(len(pv.panes))
	for i, pane := range pv.panes {
		h := pv.headers.At(i)
		h.pane = pane
		adder.AddWidget(h)
		// Content is only added when expanded — collapsed panes skip the body.
		if pane.Expanded {
			adder.AddWidget(pane.Content)
		}
	}
	return nil
}

func (pv *PaneView) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	pv.layoutItems = slices.Delete(pv.layoutItems, 0, len(pv.layoutItems))

	for i, pane := range pv.panes {
		pv.layoutItems = append(pv.layoutItems, guigui.LinearLayoutItem{
			Widget: pv.headers.At(i),
			Size:   guigui.FixedSize(paneHeaderHeight(u)),
		})
		if pane.Expanded {
			pv.layoutItems = append(pv.layoutItems, guigui.LinearLayoutItem{
				Widget: pane.Content,
				Size:   guigui.FlexibleSize(1),
			})
		}
	}
	guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     pv.layoutItems,
	}.LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func (pv *PaneView) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	fixedW, _ := constraints.FixedWidth()
	total := 0
	for _, pane := range pv.panes {
		total += paneHeaderHeight(u)
		if pane.Expanded {
			total += pane.MinBodySize
		}
	}
	return image.Pt(fixedW, total)
}

func (pv *PaneView) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	for _, pane := range pv.panes {
		w.WriteBool(pane.Expanded)
	}
}
