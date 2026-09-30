package dock

import (
	"image"
	"image/color"
	"math"
	"slices"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

// group is a leaf in the docking tree that holds one or more panels shown
// as tabs. The selected panel's content is displayed below the tab bar.
type group struct {
	guigui.DefaultWidget

	panels   []*Panel
	selected int

	locked bool

	onDragStart func(panel *Panel, cursor image.Point)

	onGroupDragStart func(group *group, cursor image.Point)

	// onTabClick, when set, overrides pressTab for tab presses. Used by edge
	// bars to implement expand/collapse/select instead of a drag.
	onTabClick func(panel *Panel, cursor image.Point)
	// onTabClose, when set, enables a close control on each tab.
	onTabClose func(panel *Panel)

	vertical     bool
	stripOnRight bool
	collapsed    bool

	tabs guigui.WidgetSlice[*groupTab]

	layoutItems []guigui.LinearLayoutItem

	tabBarUsed int
	// tabBarUsedY is the absolute Y where the tabs end and the empty part of
	// the vertical strip begins. It is set during Layout (vertical groups).
	tabBarUsedY int
	tabBounds   []image.Rectangle
	tabBar      image.Rectangle
}

// pressTab selects panel and starts a drag of it, unless the group is locked.
func (g *group) pressTab(panel *Panel, cursor image.Point) {
	for i, p := range g.panels {
		if p == panel {
			g.selected = i
			break
		}
	}
	if g.locked {
		return
	}
	if g.onDragStart != nil {
		g.onDragStart(panel, cursor)
	}
}

// HandlePointingInput starts a whole-group drag when the empty part of the tab
// bar (right of the tabs) is pressed. Vertical edge bars cannot be group-dragged.
func (g *group) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if g.vertical || g.locked || !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		return guigui.HandleInputResult{}
	}
	cursor := image.Pt(ebiten.CursorPosition())
	b := widgetBounds.Bounds()
	u := basicwidget.UnitSize(context)
	if cursor.Y >= b.Min.Y && cursor.Y < b.Min.Y+u && cursor.X >= g.tabBarUsed {
		if g.onGroupDragStart != nil {
			g.onGroupDragStart(g, cursor)
			return guigui.HandleInputByWidget(g)
		}
	}
	return guigui.HandleInputResult{}
}

func (g *group) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	context.SetClipChildren(g, true)
	if g.hideTabBar() {
		g.tabs.SetLen(0)
	} else {
		g.tabs.SetLen(len(g.panels))
		for i, panel := range g.panels {
			tab := g.tabs.At(i)
			tab.group = g
			tab.panel = panel
			tab.active = i == g.selected
			adder.AddWidget(tab)
		}
	}
	if !g.collapsed && g.selected >= 0 && g.selected < len(g.panels) {
		adder.AddWidget(g.panels[g.selected].Content)
	}
	return nil
}

// hideTabBar reports whether the tab bar should be skipped: a locked group
// with a single panel has no tab worth dragging, switching, or dropping onto.
func (g *group) hideTabBar() bool {
	return g.locked && len(g.panels) <= 1
}

func (g *group) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	b := widgetBounds.Bounds()

	if g.hideTabBar() {
		if g.selected >= 0 && g.selected < len(g.panels) {
			layouter.LayoutWidget(g.panels[g.selected].Content, b)
		}
		return
	}

	if g.vertical {
		var strip, content image.Rectangle
		if g.stripOnRight {
			strip = image.Rectangle{Min: image.Pt(b.Max.X-u, b.Min.Y), Max: b.Max}
			content = image.Rectangle{Min: b.Min, Max: image.Pt(b.Max.X-u, b.Max.Y)}
		} else {
			strip = image.Rectangle{Min: b.Min, Max: image.Pt(b.Min.X+u, b.Max.Y)}
			content = image.Rectangle{Min: image.Pt(b.Min.X+u, b.Min.Y), Max: b.Max}
		}

		g.layoutItems = slices.Delete(g.layoutItems, 0, len(g.layoutItems))
		for i := range g.tabs.Len() {
			g.layoutItems = append(g.layoutItems, guigui.LinearLayoutItem{Widget: g.tabs.At(i)})
		}
		vertical := guigui.LinearLayout{Direction: guigui.LayoutDirectionVertical, Items: g.layoutItems}
		// Record where the tabs end so the empty remainder of the strip can be
		// used as a drop-preview target.
		var itemBounds []image.Rectangle
		itemBounds = vertical.AppendItemBounds(itemBounds, context, strip)
		g.tabBounds = slices.Delete(g.tabBounds, 0, len(g.tabBounds))
		g.tabBounds = append(g.tabBounds, itemBounds...)
		g.tabBar = strip
		g.tabBarUsedY = strip.Min.Y
		for _, ib := range itemBounds {
			if ib.Max.Y > g.tabBarUsedY {
				g.tabBarUsedY = ib.Max.Y
			}
		}
		vertical.LayoutWidgets(context, strip, layouter)

		if !g.collapsed && g.selected >= 0 && g.selected < len(g.panels) {
			layouter.LayoutWidget(g.panels[g.selected].Content, content)
		}
		return
	}

	tabBar := image.Rectangle{
		Min: b.Min,
		Max: image.Pt(b.Max.X, b.Min.Y+u),
	}

	g.layoutItems = slices.Delete(g.layoutItems, 0, len(g.layoutItems))
	for i := range g.tabs.Len() {
		g.layoutItems = append(g.layoutItems, guigui.LinearLayoutItem{Widget: g.tabs.At(i)})
	}
	linear := guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Items:     g.layoutItems,
	}
	// Record where the tabs end so the empty remainder of the tab bar can be
	// used as a drop-preview target.
	var itemBounds []image.Rectangle
	itemBounds = linear.AppendItemBounds(itemBounds, context, tabBar)
	g.tabBounds = slices.Delete(g.tabBounds, 0, len(g.tabBounds))
	g.tabBounds = append(g.tabBounds, itemBounds...)
	g.tabBar = tabBar
	g.tabBarUsed = tabBar.Min.X
	for _, ib := range itemBounds {
		if ib.Max.X > g.tabBarUsed {
			g.tabBarUsed = ib.Max.X
		}
	}
	linear.LayoutWidgets(context, tabBar, layouter)

	if g.selected >= 0 && g.selected < len(g.panels) {
		layouter.LayoutWidget(g.panels[g.selected].Content, image.Rectangle{
			Min: image.Pt(b.Min.X, tabBar.Max.Y),
			Max: b.Max,
		})
	}
}

func (g *group) tabInsertionIndex(cursor image.Point) (int, bool) {
	if !cursor.In(g.tabBar) {
		return 0, false
	}
	for i, bounds := range g.tabBounds {
		if g.vertical {
			if cursor.Y < bounds.Min.Y+bounds.Dy()/2 {
				return i, true
			}
		} else if cursor.X < bounds.Min.X+bounds.Dx()/2 {
			return i, true
		}
	}
	return len(g.tabBounds), true
}

func (g *group) tabInsertionRect(index int) image.Rectangle {
	const markerWidth = 3
	if g.vertical {
		if index >= len(g.tabBounds) {
			y := g.tabBar.Min.Y
			if len(g.tabBounds) > 0 {
				y = g.tabBounds[len(g.tabBounds)-1].Max.Y
			}
			return image.Rectangle{Min: image.Pt(g.tabBar.Min.X, y), Max: g.tabBar.Max}
		}
		y := g.tabBar.Max.Y
		y = g.tabBounds[index].Min.Y
		return image.Rectangle{
			Min: image.Pt(g.tabBar.Min.X, y-markerWidth/2),
			Max: image.Pt(g.tabBar.Max.X, y+(markerWidth+1)/2),
		}
	}
	if index >= len(g.tabBounds) {
		x := g.tabBar.Min.X
		if len(g.tabBounds) > 0 {
			x = g.tabBounds[len(g.tabBounds)-1].Max.X
		}
		return image.Rectangle{Min: image.Pt(x, g.tabBar.Min.Y), Max: g.tabBar.Max}
	}
	x := g.tabBar.Max.X
	x = g.tabBounds[index].Min.X
	return image.Rectangle{
		Min: image.Pt(x-markerWidth/2, g.tabBar.Min.Y),
		Max: image.Pt(x+(markerWidth+1)/2, g.tabBar.Max.Y),
	}
}

func (g *group) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	if g.hideTabBar() {
		return
	}
	u := basicwidget.UnitSize(context)
	b := widgetBounds.Bounds()
	rail := color.RGBA{0x23, 0x26, 0x2a, 0xff}
	var clr color.RGBA
	if context.ColorMode() == ebiten.ColorModeLight {
		rail = color.RGBA{0xee, 0xf0, 0xf2, 0xff}
		clr = color.RGBA{0xc0, 0xc0, 0xc0, 0xff}
	} else {
		clr = color.RGBA{0x38, 0x38, 0x38, 0xff}
	}
	if g.vertical {
		if g.stripOnRight {
			vector.FillRect(dst, float32(b.Max.X-u), float32(b.Min.Y), float32(u), float32(b.Dy()), rail, false)
		} else {
			vector.FillRect(dst, float32(b.Min.X), float32(b.Min.Y), float32(u), float32(b.Dy()), rail, false)
		}
		// Separator between the vertical strip and the content.
		if g.stripOnRight {
			vector.StrokeLine(dst, float32(b.Max.X-u), float32(b.Min.Y), float32(b.Max.X-u), float32(b.Max.Y), 1, clr, false)
		} else {
			vector.StrokeLine(dst, float32(b.Min.X+u), float32(b.Min.Y), float32(b.Min.X+u), float32(b.Max.Y), 1, clr, false)
		}
		return
	}
	vector.FillRect(dst, float32(b.Min.X), float32(b.Min.Y), float32(b.Dx()), float32(u), rail, false)
	// A separator line under the tab bar.
	vector.StrokeLine(dst, float32(b.Min.X), float32(b.Min.Y+u), float32(b.Max.X), float32(b.Min.Y+u), 1, clr, false)
}

type groupTab struct {
	guigui.DefaultWidget

	group  *group
	panel  *Panel
	active bool

	label basicwidget.Text
}

func (t *groupTab) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	if !t.group.vertical {
		adder.AddWidget(&t.label)
		t.label.SetValue(t.panel.Title)
	}
	var style basicwidget.TextStyle
	if t.active {
		style.SetBold(true)
	}
	t.label.SetBaseStyle(&style)
	t.label.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	t.label.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
	return nil
}

func (t *groupTab) closable() bool {
	return t.group != nil && !t.group.locked && t.group.onTabClose != nil
}

func tabCloseSize(u int) int {
	return max(10, u/3)
}

func (t *groupTab) closeRect(b image.Rectangle, u int) image.Rectangle {
	if !t.closable() {
		return image.Rectangle{}
	}
	s := tabCloseSize(u)
	if t.group.vertical {
		x := b.Min.X + (b.Dx()-s)/2
		return image.Rectangle{
			Min: image.Pt(x, b.Max.Y-s-2),
			Max: image.Pt(x+s, b.Max.Y-2),
		}
	}
	y := b.Min.Y + (b.Dy()-s)/2
	return image.Rectangle{
		Min: image.Pt(b.Max.X-s-2, y),
		Max: image.Pt(b.Max.X-2, y+s),
	}
}

func (t *groupTab) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if !inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) || !widgetBounds.IsHitAtCursor() {
		return guigui.HandleInputResult{}
	}
	cursor := image.Pt(ebiten.CursorPosition())
	u := basicwidget.UnitSize(context)
	if r := t.closeRect(widgetBounds.Bounds(), u); !r.Empty() && cursor.In(r) {
		if t.group.onTabClose != nil {
			t.group.onTabClose(t.panel)
		}
		return guigui.HandleInputByWidget(t)
	}
	if t.group.onTabClick != nil {
		t.group.onTabClick(t.panel, cursor)
	} else {
		t.group.pressTab(t.panel, cursor)
	}
	return guigui.HandleInputByWidget(t)
}

func (t *groupTab) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	b := widgetBounds.Bounds().Inset(u / 4)
	if t.closable() && !t.group.vertical {
		if cr := t.closeRect(widgetBounds.Bounds(), u); cr.Min.X < b.Max.X {
			b.Max.X = cr.Min.X
		}
	}
	layouter.LayoutWidget(&t.label, b)
}

func (t *groupTab) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	if t.group.vertical {
		face := t.face(context)
		width, height := text.Measure(t.panel.Title, face, 0)
		u := basicwidget.UnitSize(context)
		h := int(math.Ceil(width)) + u/2
		if t.closable() {
			h += tabCloseSize(u) + 4
		}
		return image.Pt(int(math.Ceil(height))+u/2, h)
	}
	u := basicwidget.UnitSize(context)
	s := t.label.Measure(context, constraints)
	w := s.X + u/2
	if t.closable() {
		w += tabCloseSize(u) + 4
	}
	return image.Pt(w, u)
}

func (t *groupTab) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	return ebiten.CursorShapePointer, true
}

func (t *groupTab) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	b := widgetBounds.Bounds()
	var clr color.RGBA
	if t.active {
		if context.ColorMode() == ebiten.ColorModeLight {
			clr = color.RGBA{0xff, 0xff, 0xff, 0xff}
		} else {
			clr = color.RGBA{0x2d, 0x32, 0x38, 0xff}
		}
	} else {
		if context.ColorMode() == ebiten.ColorModeLight {
			clr = color.RGBA{0xe5, 0xe8, 0xeb, 0xff}
		} else {
			clr = color.RGBA{0x23, 0x26, 0x2a, 0xff}
		}
	}
	vector.FillRect(dst, float32(b.Min.X), float32(b.Min.Y), float32(b.Dx()), float32(b.Dy()), clr, false)
	if t.active {
		accent := color.RGBA{0x2f, 0x80, 0xed, 0xff}
		if t.group.vertical {
			if t.group.stripOnRight {
				vector.FillRect(dst, float32(b.Max.X-2), float32(b.Min.Y), 2, float32(b.Dy()), accent, false)
			} else {
				vector.FillRect(dst, float32(b.Min.X), float32(b.Min.Y), 2, float32(b.Dy()), accent, false)
			}
		} else {
			vector.FillRect(dst, float32(b.Min.X), float32(b.Min.Y), float32(b.Dx()), 2, accent, false)
		}
	}
	if t.closable() {
		u := basicwidget.UnitSize(context)
		cr := t.closeRect(b, u)
		xclr := color.RGBA{0x6b, 0x73, 0x7d, 0xff}
		if context.ColorMode() != ebiten.ColorModeLight {
			xclr = color.RGBA{0xaa, 0xb0, 0xb8, 0xff}
		}
		pad := float32(2)
		vector.StrokeLine(dst, float32(cr.Min.X)+pad, float32(cr.Min.Y)+pad, float32(cr.Max.X)-pad, float32(cr.Max.Y)-pad, 1.5, xclr, false)
		vector.StrokeLine(dst, float32(cr.Max.X)-pad, float32(cr.Min.Y)+pad, float32(cr.Min.X)+pad, float32(cr.Max.Y)-pad, 1.5, xclr, false)
	}
	if !t.group.vertical {
		return
	}

	textBounds := b
	if t.closable() {
		u := basicwidget.UnitSize(context)
		textBounds.Max.Y = t.closeRect(b, u).Min.Y
	}

	face := t.face(context)
	width, height := text.Measure(t.panel.Title, face, 0)
	var op text.DrawOptions
	op.ColorScale.ScaleWithColor(t.textColor(context))
	if t.group.stripOnRight {
		op.GeoM.Rotate(math.Pi / 2)
		op.GeoM.Translate(float64(textBounds.Max.X)-float64(textBounds.Dx()-int(math.Ceil(height)))/2, float64(textBounds.Min.Y)+(float64(textBounds.Dy())-width)/2)
	} else {
		op.GeoM.Rotate(-math.Pi / 2)
		op.GeoM.Translate(float64(textBounds.Min.X)+(float64(textBounds.Dx())-height)/2, float64(textBounds.Max.Y)-(float64(textBounds.Dy())-width)/2)
	}
	text.Draw(dst, t.panel.Title, face, &op)
}

func (t *groupTab) face(context *guigui.Context) *text.GoTextFace {
	face := &text.GoTextFace{
		Source: basicwidget.DefaultFaceSourceEntry().FaceSource,
		Size:   basicwidget.FontSize(context),
	}
	if t.active {
		face.SetVariation(text.MustParseTag("wght"), 700)
	}
	return face
}

func (t *groupTab) textColor(context *guigui.Context) color.RGBA {
	if context.ColorMode() == ebiten.ColorModeLight {
		return color.RGBA{0x20, 0x20, 0x20, 0xff}
	}
	return color.RGBA{0xe0, 0xe0, 0xe0, 0xff}
}
