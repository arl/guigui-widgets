package configui

import (
	"image"
	"reflect"
	"slices"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

// Editor is a modal settings overlay for a configuration struct T. Open it from
// a menu (or anywhere else) with [Editor.SetOpen]. Editing a field in the UI
// mutates *target directly through reflection.
type Editor[T any] struct {
	guigui.DefaultWidget

	popup basicwidget.Popup
	page  page[T]
}

func (e *Editor[T]) SetTarget(v *T) {
	e.page.target = v
}

func (e *Editor[T]) SetOpen(open bool) {
	e.popup.SetOpen(open)
}

func (e *Editor[T]) IsOpen() bool {
	return e.popup.IsOpen()
}

func (e *Editor[T]) OnClose(f func(context *guigui.Context, reason basicwidget.PopupCloseReason)) {
	e.popup.OnClose(f)
}

func (e *Editor[T]) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&e.popup)

	e.popup.SetContent(&e.page)
	e.popup.SetModal(true)
	e.popup.SetBackgroundDark(true)
	e.popup.SetCloseByClickingOutside(true)
	e.popup.SetAnimated(true)
	if e.page.close == nil {
		e.page.close = func() {
			e.popup.SetOpen(false)
		}
	}
	return nil
}

func (e *Editor[T]) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	app := context.AppBounds()
	e.popup.SetBackgroundBounds(app)
	w := app.Dx() * 9 / 10
	h := app.Dy() * 9 / 10
	x := app.Min.X + (app.Dx()-w)/2
	y := app.Min.Y + (app.Dy()-h)/2
	layouter.LayoutWidget(&e.popup, image.Rect(x, y, x+w, y+h))
}

type page[T any] struct {
	guigui.DefaultWidget

	target *T
	close  func()

	title    basicwidget.Text
	closeBtn basicwidget.Button

	sidebarPanel basicwidget.Panel
	sidebar      sidebar
	formPanel    basicwidget.Panel

	nodes     []treeNode
	groups    []*groupWidget
	selected  int
	collapsed map[int]bool
	listItems []basicwidget.ListItem[int]

	onCloseDown    func(context *guigui.Context)
	onItemSelected func(context *guigui.Context, index int)
	onExpander     func(context *guigui.Context, index int, expanded bool)

	headerLayout guigui.LinearLayout
	bodyLayout   guigui.LinearLayout
	headerItems  []guigui.LinearLayoutItem
	bodyItems    []guigui.LinearLayoutItem
	layoutItems  []guigui.LinearLayoutItem
}

func (p *page[T]) ensure() {
	if p.nodes != nil {
		return
	}
	p.nodes = groupTree(buildSchema[T]())
	p.groups = make([]*groupWidget, len(p.nodes))
	for i, n := range p.nodes {
		p.groups[i] = newGroupWidget(n.schema)
	}
	p.selected = firstLeafNode(p.nodes)
	p.collapsed = map[int]bool{}
}

func (p *page[T]) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	if p.target == nil {
		return nil
	}
	p.ensure()

	adder.AddWidget(&p.title)
	adder.AddWidget(&p.closeBtn)
	adder.AddWidget(&p.sidebarPanel)
	adder.AddWidget(&p.formPanel)

	p.title.SetValue("Settings")
	var titleStyle basicwidget.TextStyle
	titleStyle.SetBold(true)
	p.title.SetBaseStyle(&titleStyle)
	p.title.SetVerticalAlign(basicwidget.VerticalAlignMiddle)

	p.closeBtn.SetText("Close")
	if p.onCloseDown == nil {
		p.onCloseDown = func(context *guigui.Context) {
			if p.close != nil {
				p.close()
			}
		}
	}
	p.closeBtn.OnDown(p.onCloseDown)

	p.sidebarPanel.SetStyle(basicwidget.PanelStyleSide)
	p.sidebarPanel.SetBorders(basicwidget.PanelBorders{End: true})
	p.sidebarPanel.SetContent(&p.sidebar)

	p.formPanel.SetAutoBorder(true)
	p.formPanel.SetContentConstraints(basicwidget.PanelContentConstraintsFixedWidth)
	if len(p.groups) > 0 {
		if p.selected < 0 || p.selected >= len(p.groups) {
			p.selected = 0
		}
		p.formPanel.SetContent(p.groups[p.selected])
		p.groups[p.selected].setValue(valueAt(reflect.ValueOf(p.target).Elem(), p.nodes[p.selected].path))
	}

	p.sidebar.list.SetStyle(basicwidget.ListStyleSidebar)
	p.sidebar.list.SetItemHeight(basicwidget.UnitSize(context))
	p.listItems = slices.Delete(p.listItems, 0, len(p.listItems))
	for _, n := range p.nodes {
		p.listItems = append(p.listItems, basicwidget.ListItem[int]{
			Text:        n.label,
			Value:       n.id,
			IndentLevel: n.indent,
			Collapsed:   p.collapsed[n.id],
		})
	}
	p.sidebar.list.SetItems(p.listItems)
	if len(p.nodes) > 0 {
		p.sidebar.list.SelectItemByValue(p.nodes[p.selected].id)
	}
	if p.onItemSelected == nil {
		p.onItemSelected = func(context *guigui.Context, index int) {
			item, ok := p.sidebar.list.ItemByIndex(index)
			if !ok {
				return
			}
			p.selected = item.Value
		}
	}
	p.sidebar.list.OnItemSelected(p.onItemSelected)
	if p.onExpander == nil {
		p.onExpander = func(context *guigui.Context, index int, expanded bool) {
			item, ok := p.sidebar.list.ItemByIndex(index)
			if !ok {
				return
			}
			p.collapsed[item.Value] = !expanded
		}
	}
	p.sidebar.list.OnItemExpanderToggled(p.onExpander)

	context.SetButtonInputReceptive(p, true)
	return nil
}

func (p *page[T]) layout(context *guigui.Context) guigui.LinearLayout {
	u := basicwidget.UnitSize(context)

	p.headerItems = slices.Delete(p.headerItems, 0, len(p.headerItems))
	p.headerItems = append(
		p.headerItems,
		guigui.LinearLayoutItem{Widget: &p.title, Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &p.closeBtn},
	)
	p.headerLayout = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Gap:       u / 2,
		Items:     p.headerItems,
	}

	p.bodyItems = slices.Delete(p.bodyItems, 0, len(p.bodyItems))
	p.bodyItems = append(
		p.bodyItems,
		guigui.LinearLayoutItem{Widget: &p.sidebarPanel, Size: guigui.FixedSize(8 * u)},
		guigui.LinearLayoutItem{Widget: &p.formPanel, Size: guigui.FlexibleSize(1)},
	)
	p.bodyLayout = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Gap:       u / 2,
		Items:     p.bodyItems,
	}

	p.layoutItems = slices.Delete(p.layoutItems, 0, len(p.layoutItems))
	p.layoutItems = append(
		p.layoutItems,
		guigui.LinearLayoutItem{Layout: &p.headerLayout},
		guigui.LinearLayoutItem{Layout: &p.bodyLayout, Size: guigui.FlexibleSize(1)},
	)
	return guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Gap:       u / 2,
		Items:     p.layoutItems,
		Padding: guigui.Padding{
			Start:  u / 2,
			Top:    u / 2,
			End:    u / 2,
			Bottom: u / 2,
		},
	}
}

func (p *page[T]) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	p.layout(context).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func (p *page[T]) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return p.layout(context).Measure(context, constraints)
}

func (p *page[T]) HandleButtonInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		if p.close != nil {
			p.close()
		}
		return guigui.HandleInputByWidget(p)
	}
	return guigui.AbortHandlingInputByWidget(p)
}

type sidebar struct {
	guigui.DefaultWidget

	list basicwidget.List[int]
	size image.Point
}

func (s *sidebar) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&s.list)
	return nil
}

func (s *sidebar) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	s.size = widgetBounds.Bounds().Size()
	layouter.LayoutWidget(&s.list, widgetBounds.Bounds())
}

func (s *sidebar) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	if s.size != (image.Point{}) {
		return s.size
	}
	return s.list.Measure(context, constraints)
}
