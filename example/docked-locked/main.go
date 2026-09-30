package main

import (
	"fmt"
	"image"
	"image/color"
	"os"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"

	"github.com/arl/guigui-widgets/docked"
)

// Root hosts a single dock tree: a locked "Emulator" node that can never
// leave, surrounded by ordinary dockable debugger panels.
type Root struct {
	guigui.DefaultWidget

	background basicwidget.Background
	menubar    basicwidget.Menubar[string]
	dock       *docked.Root

	screen      emulatorScreen
	registers   textPanel
	memory      textPanel
	tiles       textPanel
	console     textPanel
	breakpoints textPanel

	emulatorNode    *docked.Node
	registersNode   *docked.Node
	memoryNode      *docked.Node
	tilesNode       *docked.Node
	consoleNode     *docked.Node
	breakpointsNode *docked.Node
}

func (r *Root) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&r.background)
	adder.AddWidget(&r.menubar)
	adder.AddWidget(r.dock)

	// The emulator node is deliberately absent from this menu: it is always
	// present, so there is nothing to toggle.
	nodes := []struct {
		text string
		node *docked.Node
	}{
		{text: "CPU Registers", node: r.registersNode},
		{text: "Memory Viewer", node: r.memoryNode},
		{text: "Tile Viewer", node: r.tilesNode},
		{text: "Console", node: r.consoleNode},
		{text: "Breakpoints", node: r.breakpointsNode},
	}
	r.menubar.SetItems([]basicwidget.MenubarItem{{Text: "Panels"}})
	items := make([]basicwidget.PopupMenuItem[string], 0, len(nodes))
	for _, n := range nodes {
		items = append(items, basicwidget.PopupMenuItem[string]{
			Text:    n.text,
			Checked: r.dock.Contains(n.node),
			Value:   n.text,
		})
	}
	const idxPanels = 0
	r.menubar.PopupMenuAt(idxPanels).SetItems(items)
	r.menubar.OnItemSelected(func(context *guigui.Context, menuIndex, itemIndex int) {
		if menuIndex != idxPanels || itemIndex < 0 || itemIndex >= len(nodes) {
			return
		}
		r.toggleNode(nodes[itemIndex].node)
	})
	return nil
}

func (r *Root) toggleNode(node *docked.Node) {
	if r.dock.Contains(node) {
		r.dock.Remove(node)
		return
	}
	// Prefer stacking the newly shown panel onto another visible dockable
	// panel; the always-present emulator node is the fallback anchor so the
	// first toggle has somewhere to attach.
	for _, target := range []*docked.Node{r.registersNode, r.memoryNode, r.tilesNode, r.consoleNode, r.breakpointsNode} {
		if target != node && r.dock.Contains(target) {
			r.dock.Add(node, target, docked.Center)
			return
		}
	}
	r.dock.Add(node, r.emulatorNode, docked.Right)
}

func (r *Root) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	b := widgetBounds.Bounds()
	layouter.LayoutWidget(&r.background, b)
	menuHeight := r.menubar.Measure(context, guigui.Constraints{}).Y
	menuBounds := image.Rect(b.Min.X, b.Min.Y, b.Max.X, b.Min.Y+menuHeight)
	layouter.LayoutWidget(&r.menubar, menuBounds)
	layouter.LayoutWidget(r.dock, image.Rect(b.Min.X, menuBounds.Max.Y, b.Max.X, b.Max.Y))
}

// emulatorScreen is a stand-in for the game viewport. Docked as a
// [docked.LockedGroup], it can never be dragged away, removed, or hidden behind
// another tab, even though it lives in the same tree as the other panels.
type emulatorScreen struct {
	guigui.DefaultWidget
}

func (e *emulatorScreen) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	b := widgetBounds.Bounds()
	if b.Empty() {
		return
	}
	vector.FillRect(dst, float32(b.Min.X), float32(b.Min.Y), float32(b.Dx()), float32(b.Dy()), color.RGBA{0x14, 0x14, 0x18, 0xff}, false)

	margin := int(16 * context.Scale())
	inner := b.Inset(margin)
	if inner.Empty() {
		return
	}
	// The framebuffer keeps a fixed aspect ratio and is letterboxed within
	// whatever space the surrounding docks leave available.
	fb := fitAspect(inner, 4, 3)
	fw, fh := float32(fb.Dx()), float32(fb.Dy())
	vector.FillRect(dst, float32(fb.Min.X), float32(fb.Min.Y), fw, fh, color.RGBA{0x0d, 0x1b, 0x3a, 0xff}, false)

	// A couple of placeholder shapes so the framebuffer reads as a game scene
	// rather than a blank rectangle.
	vector.FillRect(dst, float32(fb.Min.X), float32(fb.Min.Y), fw, fh*0.4, color.RGBA{0x2f, 0x80, 0xed, 0x50}, false)
	vector.FillRect(dst, float32(fb.Min.X), float32(fb.Min.Y)+fh*0.75, fw, fh*0.25, color.RGBA{0x2e, 0x7d, 0x32, 0xff}, false)
	vector.FillRect(dst, float32(fb.Min.X)+fw*0.45, float32(fb.Min.Y)+fh*0.58, fw*0.06, fh*0.2, color.RGBA{0xe5, 0x39, 0x35, 0xff}, false)
}

// fitAspect returns the largest aw:ah rectangle that fits centered within r.
func fitAspect(r image.Rectangle, aw, ah int) image.Rectangle {
	if r.Dx()*ah > r.Dy()*aw {
		w := r.Dy() * aw / ah
		x0 := r.Min.X + (r.Dx()-w)/2
		return image.Rect(x0, r.Min.Y, x0+w, r.Max.Y)
	}
	h := r.Dx() * ah / aw
	y0 := r.Min.Y + (r.Dy()-h)/2
	return image.Rect(r.Min.X, y0, r.Max.X, y0+h)
}

// textPanel is a read-only, scrollable block of placeholder text used to
// stand in for a real debugger panel (registers, memory, console, ...).
type textPanel struct {
	guigui.DefaultWidget

	input       basicwidget.TextInput
	value       string
	initialized bool
}

func (p *textPanel) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&p.input)
	p.input.SetMultiline(true)
	p.input.SetWrapMode(basicwidget.WrapModeNormal)
	p.input.SetEditable(false)
	// Seed the placeholder content once, so a later rebuild does not fight a
	// (hypothetical, here read-only) edit.
	if !p.initialized {
		p.input.SetValue(p.value)
		p.initialized = true
	}
	return nil
}

func (p *textPanel) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	layouter.LayoutWidget(&p.input, widgetBounds.Bounds().Inset(u/2))
}

func main() {
	root := &Root{}
	root.registers.value = "PC   $8000\nSP   $01FD\nA $00  X $00  Y $00\nP nvubdIzc\n\nCycles: 1,234,567"
	root.memory.value = "$0000: A9 00 8D 00 20 A2 FF 9A\n$0008: A9 00 95 00 CA 10 FB EA\n$0010: 4C 00 80 00 00 00 00 00"
	root.tiles.value = "[tile viewer placeholder]\n\n16x16 grid of 8x8 tiles\nwould render here."
	root.console.value = "> reset\nBIOS loaded\nCartridge: SUPER GAME (JP)\nMapper: 1 (MMC1)"
	root.breakpoints.value = "$8123  PC == $8123\n$81F0  A == $00        (disabled)\n$0200  write           (disabled)"

	// LockedGroup, unlike Group, can never be dragged out, removed, or
	// covered by a tab dropped on top of it - it just always sits in the
	// tree, wherever the initial layout (or a later Add) places it.
	root.emulatorNode = docked.LockedGroup("emulator", &docked.Panel{Title: "Emulator", Content: &root.screen})
	root.registersNode = docked.Group("registers", &docked.Panel{Title: "CPU Registers", Content: &root.registers})
	root.memoryNode = docked.Group("memory", &docked.Panel{Title: "Memory Viewer", Content: &root.memory})
	root.tilesNode = docked.Group("tiles", &docked.Panel{Title: "Tile Viewer", Content: &root.tiles})
	root.consoleNode = docked.Group("console", &docked.Panel{Title: "Console", Content: &root.console})
	root.breakpointsNode = docked.Group("breakpoints", &docked.Panel{Title: "Breakpoints", Content: &root.breakpoints})

	// The emulator gets most of the width; registers/memory share a column
	// beside it, and the console sits below everything.
	initialLayout := docked.Split(
		docked.Vertical, 0.75,
		docked.Split(
			docked.Horizontal, 0.7,
			root.emulatorNode,
			docked.Split(docked.Vertical, 0.5, root.registersNode, root.memoryNode),
		),
		root.consoleNode,
	)
	var err error
	root.dock, err = docked.NewRoot(initialLayout, root.tilesNode, root.breakpointsNode)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	if err := guigui.Run(root, &guigui.RunOptions{
		Title:      "Docking / Locked Group",
		WindowSize: image.Pt(1100, 700),
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
