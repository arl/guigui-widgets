package docked

import (
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"math"
	"slices"
	"sort"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

type Direction int

const (
	Horizontal Direction = iota
	Vertical
)

type Position int

const (
	Left Position = iota
	Right
	Top
	Bottom
	Center
)

type dropEdge int

const (
	dropEdgeNone dropEdge = iota
	dropEdgeLeft
	dropEdgeRight
	dropEdgeTop
	dropEdgeBottom
	dropEdgeCenter
)

type Node struct {
	ID     string
	group  *group
	split  *split
	panels []*Panel
}

// Group creates a named leaf node containing panels. ID must be stable across
// application runs when the node participates in layout serialization.
func Group(id string, panels ...*Panel) *Node {
	panels = append([]*Panel(nil), panels...)
	return &Node{ID: id, group: &group{panels: append([]*Panel(nil), panels...)}, panels: panels}
}

// LockedGroup creates a named leaf node like [Group], but one whose panels can
// never be dragged out, removed, or covered by another node's panels tabbed in
// via a drag. Use it for content that must always stay visible, such as a fixed
// viewport docked among otherwise rearrangeable panels. Other nodes can still
// be docked to its Left, Right, Top, or Bottom.
func LockedGroup(id string, panels ...*Panel) *Node {
	node := Group(id, panels...)
	node.group.locked = true
	return node
}

func (n *Node) locked() bool {
	return n != nil && n.group != nil && n.group.locked
}

func Split(direction Direction, ratio float64, first, second *Node) *Node {
	return &Node{split: &split{direction: direction, ratio: ratio, first: first, second: second}}
}

type split struct {
	direction Direction
	ratio     float64
	first     *Node
	second    *Node
}

type dockDivider struct {
	split     *split
	bounds    image.Rectangle
	available int
}

type dockGroupBounds struct {
	node   *Node
	bounds image.Rectangle
}

// dockOverlay draws the drop-preview on top of the groups during a drag. While
// a divider or panel drag is in flight it captures pointer input so widgets
// under the cursor cannot steal the gesture.
type dockOverlay struct {
	guigui.DefaultWidget

	dock *layout
}

func (o *dockOverlay) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if o.dock == nil {
		return guigui.HandleInputResult{}
	}
	return o.dock.handleCapturedPointer(context)
}

func (o *dockOverlay) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if o.dock != nil && o.dock.dragging != nil {
		return resizeCursorFor(o.dock.dragging.direction), true
	}
	return 0, false
}

func (o *dockOverlay) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	if o.dock == nil {
		return
	}
	r := o.dock.dropRect
	if r.Empty() {
		return
	}
	accent := color.RGBA{0x2f, 0x80, 0xed, 0xff}
	if o.dock.dropRoot {
		vector.StrokeRect(dst, float32(r.Min.X), float32(r.Min.Y), float32(r.Dx()), float32(r.Dy()), 2, accent, false)
		return
	}
	if o.dock.dropTabGroup != nil {
		// Insertion targets should guide placement without obscuring the tab rail.
		vector.StrokeRect(dst, float32(r.Min.X), float32(r.Min.Y), float32(r.Dx()), float32(r.Dy()), 1, accent, false)
		return
	}
	vector.StrokeRect(dst, float32(r.Min.X), float32(r.Min.Y), float32(r.Dx()), float32(r.Dy()), 1, accent, false)
}

// layout arranges dockable panels in a nestable tree of groups (tabs)
// and splits. Dividers resize; dragging a tab re-docks it, either as another
// tab (center) or as a new group split onto an edge.
type layout struct {
	guigui.DefaultWidget
	redrawTarget guigui.Widget

	root    *Node
	overlay dockOverlay

	left  *edgeBar
	right *edgeBar

	// leftZone/rightZone are the drop zones for the edges (bar bounds, or the
	// outer strip when no bar exists). rootBounds is the full layout bounds.
	leftZone   image.Rectangle
	rightZone  image.Rectangle
	rootBounds image.Rectangle

	dividers    []dockDivider
	groupBounds []dockGroupBounds

	// Divider-resize drag state.
	dragging       *split
	dragOrigin     image.Point
	dragStartRatio float64
	dragAvailable  int

	// Panel-move drag state.
	dragPanel    *Panel
	dragGroup    *group
	sourceNode   *Node
	dropNode     *Node
	dropEdge     dropEdge
	dropRect     image.Rectangle
	dropTabGroup *group
	dropTabIndex int
	dropRoot     bool
	dragFromBar  edgeSide
	dropBar      edgeSide

	// Group-move drag state. Non-nil when dragging a whole group.
	dragGroupNode *Node

	onClosePanel func(*Panel)
}

func (d *layout) requestRedraw() {
	if d.redrawTarget != nil {
		guigui.RequestRedraw(d.redrawTarget)
		return
	}
	guigui.RequestRedraw(d)
}

// Root is a docking layout. Register every leaf with [NewRoot], including
// leaves that start hidden, then add the root to the widget tree.
type Root struct {
	guigui.DefaultWidget
	layout        layout
	nodes         map[string]*Node
	onPanelClosed func(*Node)
}

func NewRoot(initial *Node, nodes ...*Node) (*Root, error) {
	r := &Root{nodes: make(map[string]*Node)}
	for _, node := range append(leafNodes(initial), nodes...) {
		if node == nil || node.group == nil {
			return nil, fmt.Errorf("dock: registered node must be a leaf")
		}
		if node.ID == "" {
			return nil, fmt.Errorf("dock: registered node has an empty ID")
		}
		if existing := r.nodes[node.ID]; existing != nil && existing != node {
			return nil, fmt.Errorf("dock: duplicate node ID %q", node.ID)
		}
		r.nodes[node.ID] = node
	}
	r.layout.setRoot(initial)
	return r, nil
}

func (r *Root) SetOnPanelClosed(f func(*Node)) {
	r.onPanelClosed = f
}

func (r *Root) contentAt(p image.Point) guigui.Widget {
	if r.layout.capturingPointer() {
		return nil
	}
	if bar := r.layout.left; bar != nil && p.In(r.layout.leftZone) {
		if p.In(bar.pinRect) {
			return nil
		}
		return selectedPanelContent(bar.group)
	}
	if bar := r.layout.right; bar != nil && p.In(r.layout.rightZone) {
		if p.In(bar.pinRect) {
			return nil
		}
		return selectedPanelContent(bar.group)
	}
	for _, gb := range r.layout.groupBounds {
		if p.In(gb.bounds) {
			return selectedPanelContent(gb.node.group)
		}
	}
	return nil
}

func (r *Root) selectedContent(node *Node) guigui.Widget {
	if node == nil {
		return nil
	}
	if g := r.layout.groupContainingPanels(node.panels); g != nil {
		return selectedPanelContent(g)
	}
	return selectedPanelContent(node.group)
}

func selectedPanelContent(g *group) guigui.Widget {
	if g == nil || g.selected < 0 || g.selected >= len(g.panels) {
		return nil
	}
	p := g.panels[g.selected]
	if p == nil {
		return nil
	}
	return p.Content
}

func (r *Root) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	r.layout.redrawTarget = r
	r.layout.onClosePanel = r.closePanel
	return r.layout.Build(context, adder)
}

func (r *Root) closePanel(panel *Panel) {
	n := r.nodeForPanel(panel)
	if n == nil || n.locked() {
		return
	}
	if !r.Remove(n) {
		return
	}
	if r.onPanelClosed != nil {
		r.onPanelClosed(n)
	}
}

func (r *Root) nodeForPanel(panel *Panel) *Node {
	if panel == nil {
		return nil
	}
	for _, n := range r.nodes {
		if slices.Contains(n.panels, panel) {
			return n
		}
	}
	return nil
}

func (r *Root) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	r.layout.Layout(context, widgetBounds, layouter)
}

func (r *Root) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	return r.layout.HandlePointingInput(context, widgetBounds)
}

func (r *Root) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	return r.layout.CursorShape(context, widgetBounds)
}

func (r *Root) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	r.layout.Draw(context, widgetBounds, dst)
}

func (r *Root) Contains(node *Node) bool { return r.layout.contains(node) }

func (r *Root) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	r.layout.WriteStateKey(context, w)
}

// ReplaceTree swaps the docking tree. Used to enter play mode (emulator
// only) or restore a debug layout without recreating registered nodes.
// Edge bars are cleared; call [Root.SetEdgeBar] afterwards if needed.
func (r *Root) ReplaceTree(node *Node) {
	r.layout.setRoot(node)
	r.layout.left = nil
	r.layout.right = nil
	r.layout.dragPanel = nil
	r.layout.dragGroup = nil
	r.layout.dragGroupNode = nil
	r.layout.dragging = nil
	guigui.RequestRebuild()
	guigui.RequestRedraw(r)
}

// SetEdgeBar docks nodes as a sticky tab strip on the Left or Right edge.
// Nodes already in the tree are moved onto the bar. Other Position values are
// ignored. Nil nodes are skipped.
func (r *Root) SetEdgeBar(position Position, nodes ...*Node) {
	var side edgeSide
	switch position {
	case Left:
		side = edgeSideLeft
	case Right:
		side = edgeSideRight
	default:
		return
	}
	var panels []*Panel
	for _, n := range nodes {
		if n == nil || len(n.panels) == 0 {
			continue
		}
		if r.Contains(n) {
			r.Remove(n)
		}
		panels = append(panels, n.panels...)
	}
	if len(panels) == 0 {
		r.layout.setBar(side, nil)
		guigui.RequestRebuild()
		guigui.RequestRedraw(r)
		return
	}
	bar := newEdgeBar(side)
	bar.group.panels = append([]*Panel(nil), panels...)
	bar.group.selected = 0
	r.layout.setBar(side, bar)
	guigui.RequestRebuild()
	guigui.RequestRedraw(r)
}

func (r *Root) Add(node, target *Node, position Position) bool {
	if !r.layout.add(node, target, position) {
		return false
	}
	guigui.RequestRedraw(r)
	return true
}

func (r *Root) Remove(node *Node) bool {
	if !r.layout.remove(node) {
		return false
	}
	guigui.RequestRedraw(r)
	return true
}

type rootSnapshot struct {
	Version int               `json:"version"`
	Tree    *snapshotNode     `json:"tree,omitempty"`
	Left    *snapshotTabGroup `json:"left,omitempty"`
	Right   *snapshotTabGroup `json:"right,omitempty"`
}

type snapshotNode struct {
	Group *snapshotTabGroup `json:"group,omitempty"`
	Split *snapshotSplit    `json:"split,omitempty"`
}

type snapshotSplit struct {
	Direction Direction     `json:"direction"`
	Ratio     float64       `json:"ratio"`
	First     *snapshotNode `json:"first"`
	Second    *snapshotNode `json:"second"`
}

type snapshotTabGroup struct {
	Nodes    []string `json:"nodes"`
	Selected int      `json:"selected"`
}

func (r *Root) MarshalJSON() ([]byte, error) {
	if r == nil {
		return []byte("null"), nil
	}
	snapshot, err := r.snapshot()
	if err != nil {
		return nil, err
	}
	return json.Marshal(snapshot)
}

// ApplyJSON validates and applies a layout snapshot atomically. Every node ID
// in the JSON must be registered with this Root; absent registered nodes remain
// absent from the restored layout.
func (r *Root) ApplyJSON(data []byte) error {
	var snapshot rootSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return fmt.Errorf("dock: decode layout: %w", err)
	}
	tree, left, right, err := r.layoutFromSnapshot(&snapshot)
	if err != nil {
		return err
	}
	r.layout.root = tree
	r.layout.left = newEdgeBarFromGroup(edgeSideLeft, left)
	r.layout.right = newEdgeBarFromGroup(edgeSideRight, right)
	r.layout.dragPanel = nil
	r.layout.dragGroup = nil
	r.layout.dragGroupNode = nil
	guigui.RequestRebuild()
	guigui.RequestRedraw(r)
	return nil
}

func (r *Root) layoutFromSnapshot(snapshot *rootSnapshot) (*Node, *group, *group, error) {
	if snapshot.Version != 1 {
		return nil, nil, nil, fmt.Errorf("dock: unsupported layout version %d", snapshot.Version)
	}
	seen := make(map[string]struct{})
	tree, err := r.nodeFromSnapshot(snapshot.Tree, seen)
	if err != nil {
		return nil, nil, nil, err
	}
	left, err := r.groupFromSnapshot(snapshot.Left, seen)
	if err != nil {
		return nil, nil, nil, err
	}
	right, err := r.groupFromSnapshot(snapshot.Right, seen)
	if err != nil {
		return nil, nil, nil, err
	}
	return tree, left, right, nil
}

func (r *Root) snapshot() (rootSnapshot, error) {
	tree, err := r.snapshotNode(r.layout.root)
	if err != nil {
		return rootSnapshot{}, err
	}
	left, err := r.snapshotGroup(r.layout.left)
	if err != nil {
		return rootSnapshot{}, err
	}
	right, err := r.snapshotGroup(r.layout.right)
	if err != nil {
		return rootSnapshot{}, err
	}
	return rootSnapshot{Version: 1, Tree: tree, Left: left, Right: right}, nil
}

func (r *Root) snapshotNode(node *Node) (*snapshotNode, error) {
	if node == nil {
		return nil, nil
	}
	if node.group != nil {
		group, err := r.snapshotGroupFromGroup(node.group)
		if err != nil {
			return nil, err
		}
		return &snapshotNode{Group: group}, nil
	}
	if node.split == nil {
		return nil, fmt.Errorf("dock: node has neither a group nor a split")
	}
	first, err := r.snapshotNode(node.split.first)
	if err != nil {
		return nil, err
	}
	second, err := r.snapshotNode(node.split.second)
	if err != nil {
		return nil, err
	}
	return &snapshotNode{Split: &snapshotSplit{
		Direction: node.split.direction,
		Ratio:     node.split.ratio,
		First:     first,
		Second:    second,
	}}, nil
}

func (r *Root) snapshotGroup(bar *edgeBar) (*snapshotTabGroup, error) {
	if bar == nil {
		return nil, nil
	}
	return r.snapshotGroupFromGroup(bar.group)
}

func (r *Root) snapshotGroupFromGroup(current *group) (*snapshotTabGroup, error) {
	if current == nil {
		return nil, nil
	}
	type entry struct {
		id    string
		index int
	}
	entries := make([]entry, 0, len(r.nodes))
	covered := make(map[*Panel]struct{})
	for id, node := range r.nodes {
		if !groupContainsPanels(current, node.panels) {
			continue
		}
		index := slices.Index(current.panels, node.panels[0])
		entries = append(entries, entry{id: id, index: index})
		for _, panel := range node.panels {
			covered[panel] = struct{}{}
		}
	}
	if len(covered) != len(current.panels) {
		return nil, fmt.Errorf("dock: a tab group contains panels without a registered node")
	}
	for _, panel := range current.panels {
		if _, ok := covered[panel]; !ok {
			return nil, fmt.Errorf("dock: a tab group contains panels without a registered node")
		}
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].index < entries[j].index })
	result := &snapshotTabGroup{Selected: current.selected, Nodes: make([]string, len(entries))}
	for i, entry := range entries {
		result.Nodes[i] = entry.id
	}
	return result, nil
}

func (r *Root) nodeFromSnapshot(snapshot *snapshotNode, seen map[string]struct{}) (*Node, error) {
	if snapshot == nil {
		return nil, nil
	}
	if snapshot.Group != nil && snapshot.Split != nil {
		return nil, fmt.Errorf("dock: snapshot node has both a group and a split")
	}
	if snapshot.Group != nil {
		group, err := r.groupFromSnapshot(snapshot.Group, seen)
		if err != nil {
			return nil, err
		}
		return &Node{group: group}, nil
	}
	if snapshot.Split == nil {
		return nil, fmt.Errorf("dock: snapshot node has neither a group nor a split")
	}
	if snapshot.Split.Direction != Horizontal && snapshot.Split.Direction != Vertical {
		return nil, fmt.Errorf("dock: invalid split direction %d", snapshot.Split.Direction)
	}
	if math.IsNaN(snapshot.Split.Ratio) || math.IsInf(snapshot.Split.Ratio, 0) || snapshot.Split.Ratio <= 0 || snapshot.Split.Ratio >= 1 {
		return nil, fmt.Errorf("dock: invalid split ratio %v", snapshot.Split.Ratio)
	}
	first, err := r.nodeFromSnapshot(snapshot.Split.First, seen)
	if err != nil {
		return nil, err
	}
	second, err := r.nodeFromSnapshot(snapshot.Split.Second, seen)
	if err != nil {
		return nil, err
	}
	if first == nil || second == nil {
		return nil, fmt.Errorf("dock: split must have two children")
	}
	return &Node{split: &split{direction: snapshot.Split.Direction, ratio: snapshot.Split.Ratio, first: first, second: second}}, nil
}

func (r *Root) groupFromSnapshot(snapshot *snapshotTabGroup, seen map[string]struct{}) (*group, error) {
	if snapshot == nil {
		return nil, nil
	}
	if len(snapshot.Nodes) == 0 {
		return nil, fmt.Errorf("dock: tab group has no nodes")
	}
	panels := make([]*Panel, 0)
	var locked bool
	for _, id := range snapshot.Nodes {
		if _, ok := seen[id]; ok {
			return nil, fmt.Errorf("dock: node ID %q appears more than once", id)
		}
		node := r.nodes[id]
		if node == nil {
			return nil, fmt.Errorf("dock: snapshot references unknown node ID %q", id)
		}
		if node.locked() && len(snapshot.Nodes) > 1 {
			return nil, fmt.Errorf("dock: locked node %q cannot be tabbed with other nodes", id)
		}
		locked = locked || node.locked()
		seen[id] = struct{}{}
		panels = append(panels, node.panels...)
	}
	if snapshot.Selected < 0 || snapshot.Selected >= len(panels) {
		return nil, fmt.Errorf("dock: invalid selected tab index %d", snapshot.Selected)
	}
	return &group{panels: panels, selected: snapshot.Selected, locked: locked}, nil
}

func leafNodes(node *Node) []*Node {
	if node == nil {
		return nil
	}
	if node.group != nil {
		return []*Node{node}
	}
	if node.split == nil {
		return nil
	}
	return append(leafNodes(node.split.first), leafNodes(node.split.second)...)
}

func (d *layout) setRoot(root *Node) {
	d.root = root
}

func (d *layout) contains(node *Node) bool {
	return node != nil && len(node.panels) > 0 && d.panelsPresent(node.panels)
}

// add adds node next to target at position. Center adds node's panels as tabs
// in target's current group. When the layout is empty, target may be nil and
// node becomes the root. It returns false when node is already present or the
// target is absent.
func (d *layout) add(node, target *Node, position Position) bool {
	if node == nil || node.group == nil || len(node.panels) == 0 || d.contains(node) {
		return false
	}
	node.group.panels = append(node.group.panels[:0], node.panels...)
	node.group.selected = min(node.group.selected, len(node.panels)-1)
	if d.root == nil {
		if target != nil {
			return false
		}
		d.root = node
		guigui.RequestRebuild()
		d.requestRedraw()
		return true
	}
	if target == nil {
		return false
	}
	targetGroup := d.groupContainingPanels(target.panels)
	if targetGroup == nil {
		return false
	}
	switch position {
	case Center:
		if targetGroup.locked {
			return false
		}
		targetGroup.panels = append(targetGroup.panels, node.panels...)
		targetGroup.selected = len(targetGroup.panels) - 1
	case Left:
		targetNode := nodeContainingPanels(d.root, target.panels)
		if targetNode == nil {
			return false
		}
		d.root = attachNode(d.root, targetNode, node, dropEdgeLeft)
	case Right:
		targetNode := nodeContainingPanels(d.root, target.panels)
		if targetNode == nil {
			return false
		}
		d.root = attachNode(d.root, targetNode, node, dropEdgeRight)
	case Top:
		targetNode := nodeContainingPanels(d.root, target.panels)
		if targetNode == nil {
			return false
		}
		d.root = attachNode(d.root, targetNode, node, dropEdgeTop)
	case Bottom:
		targetNode := nodeContainingPanels(d.root, target.panels)
		if targetNode == nil {
			return false
		}
		d.root = attachNode(d.root, targetNode, node, dropEdgeBottom)
	default:
		return false
	}
	guigui.RequestRebuild()
	d.requestRedraw()
	return true
}

// remove removes node's panels from the layout. It returns false when node is
// absent. The node remains reusable with a later add call.
func (d *layout) remove(node *Node) bool {
	if node == nil || len(node.panels) == 0 || !d.contains(node) || node.locked() {
		return false
	}
	d.root = removePanels(d.root, node.panels)
	for _, side := range []edgeSide{edgeSideLeft, edgeSideRight} {
		bar := d.barFor(side)
		if bar == nil {
			continue
		}
		bar.group.panels = slices.DeleteFunc(bar.group.panels, func(panel *Panel) bool {
			return slices.Contains(node.panels, panel)
		})
		if len(bar.group.panels) == 0 {
			d.setBar(side, nil)
		} else {
			bar.group.selected = min(bar.group.selected, len(bar.group.panels)-1)
		}
	}
	node.group.panels = append(node.group.panels[:0], node.panels...)
	node.group.selected = min(node.group.selected, len(node.panels)-1)
	guigui.RequestRebuild()
	d.requestRedraw()
	return true
}

func (d *layout) capturingPointer() bool {
	return d.dragging != nil || d.dragPanel != nil || d.dragGroupNode != nil
}

func (d *layout) WriteStateKey(context *guigui.Context, w *guigui.StateKeyWriter) {
	w.WriteBool(d.dragging != nil)
	w.WriteBool(d.dragPanel != nil)
	w.WriteBool(d.dragGroupNode != nil)
}

func (d *layout) handleCapturedPointer(context *guigui.Context) guigui.HandleInputResult {
	cursor := image.Pt(ebiten.CursorPosition())
	if d.dragPanel != nil || d.dragGroupNode != nil {
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			d.updateDropTarget(context, cursor)
			d.requestRedraw()
			return guigui.HandleInputByWidget(d)
		}
		d.finishDrag()
		return guigui.HandleInputByWidget(d)
	}
	if d.dragging != nil {
		if ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			d.updateRatio(cursor)
			return guigui.HandleInputByWidget(d)
		}
		d.dragging = nil
		return guigui.HandleInputByWidget(d)
	}
	return guigui.HandleInputResult{}
}

func (d *layout) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	if d.root != nil {
		d.addNode(adder, d.root)
	}
	if d.left != nil {
		adder.AddWidget(d.left)
		d.left.onDragStart = func(panel *Panel, cursor image.Point) {
			d.beginDragFromBar(panel, d.left, cursor)
		}
		d.left.onClose = d.onClosePanel
	}
	if d.right != nil {
		adder.AddWidget(d.right)
		d.right.onDragStart = func(panel *Panel, cursor image.Point) {
			d.beginDragFromBar(panel, d.right, cursor)
		}
		d.right.onClose = d.onClosePanel
	}
	d.overlay.dock = d
	adder.AddWidget(&d.overlay)
	context.SetPassthrough(&d.overlay, !d.capturingPointer())
	return nil
}

func (d *layout) addNode(adder *guigui.ChildAdder, node *Node) {
	if node.group != nil {
		adder.AddWidget(node.group)
		nodeGroup := node.group
		nodeGroup.onDragStart = func(panel *Panel, cursor image.Point) {
			d.beginDrag(panel, nodeGroup, cursor)
		}
		nodeGroup.onGroupDragStart = func(g *group, cursor image.Point) {
			d.beginGroupDrag(g, cursor)
		}
		nodeGroup.onTabClose = d.onClosePanel
		return
	}
	if node.split != nil {
		d.addNode(adder, node.split.first)
		d.addNode(adder, node.split.second)
	}
}

func (d *layout) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	d.dividers = slices.Delete(d.dividers, 0, len(d.dividers))
	d.groupBounds = slices.Delete(d.groupBounds, 0, len(d.groupBounds))
	b := widgetBounds.Bounds()
	d.rootBounds = b

	u := basicwidget.UnitSize(context)
	mainBounds := b

	d.leftZone = image.Rectangle{Min: image.Pt(b.Min.X, b.Min.Y), Max: image.Pt(b.Min.X+u, b.Max.Y)}
	if d.left != nil {
		w := u
		if d.left.isOpen() {
			w = edgeOpenWidth(u)
		}
		leftBounds := image.Rectangle{Min: b.Min, Max: image.Pt(b.Min.X+w, b.Max.Y)}
		layouter.LayoutWidget(d.left, leftBounds)
		d.leftZone = leftBounds
		mainBounds.Min.X += w
	}

	d.rightZone = image.Rectangle{Min: image.Pt(b.Max.X-u, b.Min.Y), Max: b.Max}
	if d.right != nil {
		w := u
		if d.right.isOpen() {
			w = edgeOpenWidth(u)
		}
		rightBounds := image.Rectangle{Min: image.Pt(b.Max.X-w, b.Min.Y), Max: b.Max}
		layouter.LayoutWidget(d.right, rightBounds)
		d.rightZone = rightBounds
		mainBounds.Max.X -= w
	}

	if d.root != nil {
		d.layoutNode(context, d.root, mainBounds, layouter)
	}
	layouter.LayoutWidget(&d.overlay, b)
}

func (d *layout) dividerThickness(context *guigui.Context) int {
	return max(8, basicwidget.UnitSize(context)/4)
}

func (d *layout) splitExtents(context *guigui.Context, split *split, available int) (int, int) {
	minExtent := basicwidget.UnitSize(context)
	first := int(float64(available) * split.ratio)
	second := available - first
	if first < minExtent {
		first, second = minExtent, available-minExtent
	}
	if second < minExtent {
		first, second = available-minExtent, minExtent
	}
	return first, second
}

func (d *layout) layoutNode(context *guigui.Context, node *Node, bounds image.Rectangle, layouter *guigui.ChildLayouter) {
	if node.group != nil {
		layouter.LayoutWidget(node.group, bounds)
		d.groupBounds = append(d.groupBounds, dockGroupBounds{node: node, bounds: bounds})
		return
	}
	split := node.split
	if split == nil {
		return
	}

	t := d.dividerThickness(context)
	var first, divider, second image.Rectangle
	var available int
	switch split.direction {
	case Horizontal:
		available = bounds.Dx() - t
		f, _ := d.splitExtents(context, split, available)
		first = image.Rectangle{
			Min: bounds.Min,
			Max: image.Pt(bounds.Min.X+f, bounds.Max.Y),
		}
		divider = image.Rectangle{
			Min: image.Pt(bounds.Min.X+f, bounds.Min.Y),
			Max: image.Pt(bounds.Min.X+f+t, bounds.Max.Y),
		}
		second = image.Rectangle{
			Min: image.Pt(bounds.Min.X+f+t, bounds.Min.Y),
			Max: bounds.Max,
		}
	case Vertical:
		available = bounds.Dy() - t
		f, _ := d.splitExtents(context, split, available)
		first = image.Rectangle{
			Min: bounds.Min,
			Max: image.Pt(bounds.Max.X, bounds.Min.Y+f),
		}
		divider = image.Rectangle{
			Min: image.Pt(bounds.Min.X, bounds.Min.Y+f),
			Max: image.Pt(bounds.Max.X, bounds.Min.Y+f+t),
		}
		second = image.Rectangle{
			Min: image.Pt(bounds.Min.X, bounds.Min.Y+f+t),
			Max: bounds.Max,
		}
	}

	d.dividers = append(d.dividers, dockDivider{
		split:     split,
		bounds:    divider,
		available: available,
	})
	d.layoutNode(context, split.first, first, layouter)
	d.layoutNode(context, split.second, second, layouter)
}

func (d *layout) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	cursor := image.Pt(ebiten.CursorPosition())
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		for _, bar := range []*edgeBar{d.left, d.right} {
			if bar != nil && cursor.In(bar.pinRect) {
				bar.togglePin()
				return guigui.HandleInputByWidget(d)
			}
		}
	}

	// In-flight pointer captures are handled by the overlay so a panel
	// under the cursor cannot steal a divider drag (children are hit first).
	if d.capturingPointer() {
		return d.handleCapturedPointer(context)
	}

	// Start a divider resize on press.
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		for i := range d.dividers {
			if cursor.In(d.dividers[i].bounds) {
				d.dragging = d.dividers[i].split
				d.dragOrigin = cursor
				d.dragStartRatio = d.dividers[i].split.ratio
				d.dragAvailable = d.dividers[i].available
				return guigui.HandleInputByWidget(d)
			}
		}
	}
	return guigui.HandleInputResult{}
}

func (d *layout) beginDrag(panel *Panel, group *group, cursor image.Point) {
	d.dragPanel = panel
	d.dragGroup = group
	d.sourceNode = findGroupNode(d.root, group)
	d.dragGroupNode = nil
	d.dragFromBar = edgeSideNone
	d.dropNode = nil
	d.dropEdge = dropEdgeNone
	d.dropBar = edgeSideNone
	d.dropRect = image.Rectangle{}
	d.dropTabGroup = nil
	d.dropTabIndex = 0
	d.dropRoot = false
}

func (d *layout) beginDragFromBar(panel *Panel, bar *edgeBar, cursor image.Point) {
	d.dragPanel = panel
	d.dragGroup = bar.group
	d.sourceNode = nil
	d.dragGroupNode = nil
	d.dragFromBar = bar.side
	d.dropNode = nil
	d.dropEdge = dropEdgeNone
	d.dropBar = edgeSideNone
	d.dropRect = image.Rectangle{}
	d.dropTabGroup = nil
	d.dropTabIndex = 0
}

func (d *layout) beginGroupDrag(group *group, cursor image.Point) {
	d.dragGroupNode = findGroupNode(d.root, group)
	d.dragPanel = nil
	d.dragGroup = nil
	d.dragFromBar = edgeSideNone
	d.dropNode = nil
	d.dropEdge = dropEdgeNone
	d.dropBar = edgeSideNone
	d.dropRect = image.Rectangle{}
	d.dropTabGroup = nil
	d.dropTabIndex = 0
}

func (d *layout) updateDropTarget(context *guigui.Context, cursor image.Point) {
	d.dropNode = nil
	d.dropEdge = dropEdgeNone
	d.dropBar = edgeSideNone
	d.dropRect = image.Rectangle{}
	d.dropTabGroup = nil
	d.dropTabIndex = 0
	groupDrag := d.dragGroupNode != nil
	rootBand := basicwidget.UnitSize(context)
	if d.root != nil && cursor.In(d.rootBounds) && !d.cursorInRootChild(cursor) {
		if cursor.Y >= d.rootBounds.Min.Y+rootBand && cursor.Y < d.rootBounds.Min.Y+2*rootBand {
			d.dropNode = d.root
			d.dropEdge = dropEdgeTop
			d.dropRoot = true
			d.dropRect = image.Rectangle{Min: image.Pt(d.rootBounds.Min.X, d.rootBounds.Min.Y+rootBand), Max: image.Pt(d.rootBounds.Max.X, d.rootBounds.Min.Y+2*rootBand)}
			return
		}
		if cursor.Y >= d.rootBounds.Max.Y-2*rootBand && cursor.Y < d.rootBounds.Max.Y-rootBand {
			d.dropNode = d.root
			d.dropEdge = dropEdgeBottom
			d.dropRoot = true
			d.dropRect = image.Rectangle{Min: image.Pt(d.rootBounds.Min.X, d.rootBounds.Max.Y-2*rootBand), Max: image.Pt(d.rootBounds.Max.X, d.rootBounds.Max.Y-rootBand)}
			return
		}
	}

	// A tab strip is more specific than the root edge zone, which may overlap
	// the first tab at the left or right edge of the layout.
	for i := range d.groupBounds {
		gb := &d.groupBounds[i]
		if !cursor.In(gb.bounds) || gb.node.group.locked {
			continue
		}
		if tabIndex, ok := gb.node.group.tabInsertionIndex(cursor); ok && !(groupDrag && gb.node == d.dragGroupNode) {
			d.dropTabGroup = gb.node.group
			d.dropTabIndex = tabIndex
			d.dropRect = gb.node.group.tabInsertionRect(tabIndex)
			return
		}
	}
	for _, bar := range []*edgeBar{d.left, d.right} {
		if bar == nil {
			continue
		}
		if tabIndex, ok := bar.group.tabInsertionIndex(cursor); ok {
			d.dropTabGroup = bar.group
			d.dropTabIndex = tabIndex
			d.dropRect = bar.group.tabInsertionRect(tabIndex)
			return
		}
	}

	// Edge bar zones take priority. The drop site is the empty remainder of
	// the strip (or the whole strip when no bar exists yet).
	if !groupDrag || d.left == nil {
		if r := d.edgeBarDropRect(context, edgeSideLeft); !r.Empty() && cursor.In(r) {
			d.dropBar = edgeSideLeft
			d.dropRect = r
			return
		}
	}
	if !groupDrag || d.right == nil {
		if r := d.edgeBarDropRect(context, edgeSideRight); !r.Empty() && cursor.In(r) {
			d.dropBar = edgeSideRight
			d.dropRect = r
			return
		}
	}

	for i := range d.groupBounds {
		gb := &d.groupBounds[i]
		if !cursor.In(gb.bounds) {
			continue
		}
		edge := d.dropEdgeAt(context, cursor, gb.bounds)
		if edge == dropEdgeNone {
			continue
		}
		if edge == dropEdgeCenter && gb.node.group.locked {
			// A locked group's tabs are never covered by a drop; only edge
			// splits beside it are allowed.
			continue
		}
		if groupDrag {
			// Group drags cannot drop onto themselves, but can merge into another
			// group's center target.
			if gb.node == d.dragGroupNode {
				continue
			}
		} else {
			// A tab dropped back onto its own group is only meaningful as an
			// edge split; a center/tab-bar drop there would be a no-op.
			if gb.node == d.sourceNode && edge == dropEdgeCenter {
				continue
			}
		}
		d.dropNode = gb.node
		d.dropEdge = edge
		d.dropRect = d.dropRectFor(context, cursor, gb.bounds, edge, gb.node.group)
		return
	}
}

func (d *layout) cursorInRootChild(cursor image.Point) bool {
	if d.root == nil || d.root.split == nil {
		return false
	}
	for _, bounds := range d.groupBounds {
		if !cursor.In(bounds.bounds) {
			continue
		}
		if nodeIsDescendantOf(d.root.split.first, bounds.node) || nodeIsDescendantOf(d.root.split.second, bounds.node) {
			return true
		}
	}
	return false
}

func nodeIsDescendantOf(ancestor, target *Node) bool {
	if ancestor == nil {
		return false
	}
	if ancestor == target {
		return true
	}
	return ancestor.split != nil && (nodeIsDescendantOf(ancestor.split.first, target) || nodeIsDescendantOf(ancestor.split.second, target))
}

func (d *layout) finishDrag() {
	switch {
	case d.dragGroupNode != nil:
		if d.dropTabGroup != nil {
			d.insertGroupAt(d.dragGroupNode, d.dropTabGroup, d.dropTabIndex)
		} else if d.dropBar != edgeSideNone {
			d.moveGroupToBar(d.dragGroupNode, d.dropBar)
		} else if d.dropNode != nil && d.dropEdge != dropEdgeNone {
			if d.dropNode == d.root && d.dropEdge != dropEdgeCenter {
				d.moveGroupToRoot(d.dragGroupNode, d.dropEdge)
			} else {
				d.moveGroup(d.dragGroupNode, d.dropNode, d.dropEdge)
			}
		}
	case d.dragPanel != nil:
		if d.dropTabGroup != nil {
			d.insertPanelAt(d.dragPanel, d.dragGroup, d.dragFromBar, d.dropTabGroup, d.dropTabIndex)
		} else if d.dropBar != edgeSideNone {
			d.movePanelToBar(d.dragPanel, d.dragGroup, d.dragFromBar, d.dropBar)
		} else if d.dropNode != nil && d.dropEdge != dropEdgeNone {
			if d.dropNode == d.root && d.dropEdge != dropEdgeCenter {
				d.movePanelToRoot(d.dragPanel, d.dragGroup, d.dragFromBar, d.dropEdge)
			} else {
				d.movePanel(d.dragPanel, d.dragGroup, d.dragFromBar, d.dropNode, d.dropEdge)
			}
		}
	}
	d.dragPanel = nil
	d.dragGroup = nil
	d.sourceNode = nil
	d.dragGroupNode = nil
	d.dragFromBar = edgeSideNone
	d.dropNode = nil
	d.dropEdge = dropEdgeNone
	d.dropBar = edgeSideNone
	d.dropRect = image.Rectangle{}
	d.dropTabGroup = nil
	d.dropTabIndex = 0
	d.dropRoot = false
	d.requestRedraw()
}

// movePanel removes panel from its source (a tree group or an edge bar) and
// re-docks it: into target's group on a center drop, or as a new group split
// next to target on an edge.
func (d *layout) movePanel(panel *Panel, source *group, fromBar edgeSide, targetNode *Node, edge dropEdge) {
	// A tab dropped back onto its own group splits it off as a sibling group.
	if fromBar == edgeSideNone && targetNode.group == source {
		if len(source.panels) <= 1 {
			// A sole tab is already alone in its group; nothing to split.
			return
		}
		removePanelFromGroup(source, panel)
		node := &Node{
			group: &group{
				panels:   []*Panel{panel},
				selected: 0,
			},
		}
		d.root = attachNode(d.root, targetNode, node, edge)
		return
	}

	d.removePanelFromSource(panel, source, fromBar)

	if edge == dropEdgeCenter {
		target := targetNode.group
		target.panels = append(target.panels, panel)
		target.selected = len(target.panels) - 1
		return
	}

	newGroup := &group{
		panels:   []*Panel{panel},
		selected: 0,
	}
	d.root = attachNode(d.root, targetNode, &Node{group: newGroup}, edge)
}

func (d *layout) movePanelToRoot(panel *Panel, source *group, fromBar edgeSide, edge dropEdge) {
	root := d.root
	d.removePanelFromSource(panel, source, fromBar)
	newNode := &Node{group: &group{panels: []*Panel{panel}, selected: 0}}
	d.root = attachNode(d.root, root, newNode, edge)
}

func (d *layout) moveGroupToRoot(sourceNode *Node, edge dropEdge) {
	root := d.root
	if sourceNode == root {
		return
	}
	d.root = removeNode(d.root, sourceNode)
	d.root = attachNode(d.root, root, sourceNode, edge)
}

func (d *layout) insertPanelAt(panel *Panel, source *group, fromBar edgeSide, target *group, index int) {
	if source == target {
		oldIndex := slices.Index(source.panels, panel)
		if oldIndex < 0 {
			return
		}
		if oldIndex < index {
			index--
		}
		removePanelFromGroup(source, panel)
	} else {
		d.removePanelFromSource(panel, source, fromBar)
	}

	index = max(0, min(index, len(target.panels)))
	target.panels = append(target.panels, nil)
	copy(target.panels[index+1:], target.panels[index:])
	target.panels[index] = panel
	target.selected = index
}

func (d *layout) insertGroupAt(sourceNode *Node, target *group, index int) {
	source := sourceNode.group
	if source == nil || source == target {
		return
	}
	d.root = removeNode(d.root, sourceNode)

	index = max(0, min(index, len(target.panels)))
	panels := make([]*Panel, 0, len(target.panels)+len(source.panels))
	panels = append(panels, target.panels[:index]...)
	panels = append(panels, source.panels...)
	panels = append(panels, target.panels[index:]...)
	target.panels = panels
	target.selected = index + source.selected
}

func (d *layout) removePanelFromSource(panel *Panel, source *group, fromBar edgeSide) {
	if !removePanelFromGroup(source, panel) {
		return
	}
	if fromBar != edgeSideNone {
		d.setBar(fromBar, nil)
		return
	}
	d.root = removeNode(d.root, findGroupNode(d.root, source))
}

func (d *layout) moveGroupToBar(sourceNode *Node, side edgeSide) {
	if d.barFor(side) != nil {
		return
	}
	d.root = removeNode(d.root, sourceNode)
	bar := newEdgeBar(side)
	bar.group = sourceNode.group
	bar.group.vertical = true
	bar.group.stripOnRight = side == edgeSideRight
	d.setBar(side, bar)
}

func (d *layout) movePanelToBar(panel *Panel, source *group, fromBar, side edgeSide) {
	if fromBar == side {
		// Dropped back onto the bar it came from; nothing to do.
		return
	}
	d.removePanelFromSource(panel, source, fromBar)
	bar := d.barFor(side)
	if bar == nil {
		bar = newEdgeBar(side)
		bar.group.panels = []*Panel{panel}
		bar.group.selected = 0
		d.setBar(side, bar)
		return
	}
	bar.group.panels = append(bar.group.panels, panel)
	bar.group.selected = len(bar.group.panels) - 1
}

func (d *layout) barFor(side edgeSide) *edgeBar {
	switch side {
	case edgeSideLeft:
		return d.left
	case edgeSideRight:
		return d.right
	}
	return nil
}

// edgeBarDropRect returns the drop-preview area for an edge: the empty
// remainder of the vertical strip below the existing tabs, or the whole strip
// when no bar exists yet. It returns an empty rectangle when there is no room.
func (d *layout) edgeBarDropRect(context *guigui.Context, side edgeSide) image.Rectangle {
	u := basicwidget.UnitSize(context)
	var zone image.Rectangle
	var bar *edgeBar
	if side == edgeSideLeft {
		zone, bar = d.leftZone, d.left
	} else {
		zone, bar = d.rightZone, d.right
	}
	if bar == nil {
		return zone
	}
	var strip image.Rectangle
	if side == edgeSideLeft {
		strip = image.Rectangle{Min: zone.Min, Max: image.Pt(zone.Min.X+u, zone.Max.Y)}
	} else {
		strip = image.Rectangle{Min: image.Pt(zone.Max.X-u, zone.Min.Y), Max: zone.Max}
	}
	used := bar.group.tabBarUsedY
	if used < strip.Min.Y {
		used = strip.Min.Y
	}
	if used >= strip.Max.Y {
		return image.Rectangle{}
	}
	return image.Rectangle{Min: image.Pt(strip.Min.X, used), Max: image.Pt(strip.Max.X, strip.Max.Y)}
}

func (d *layout) setBar(side edgeSide, bar *edgeBar) {
	switch side {
	case edgeSideLeft:
		d.left = bar
	case edgeSideRight:
		d.right = bar
	}
}

// moveGroup re-docks an entire group (keeping its tabs intact) as a sibling
// of targetNode, split onto edge. The source node keeps its identity.
func (d *layout) moveGroup(sourceNode, targetNode *Node, edge dropEdge) {
	if edge == dropEdgeCenter {
		source := sourceNode.group
		target := targetNode.group
		if source == nil || target == nil || source == target {
			return
		}
		d.root = removeNode(d.root, sourceNode)
		target.panels = append(target.panels, source.panels...)
		target.selected = len(target.panels) - 1
		return
	}
	d.root = removeNode(d.root, sourceNode)
	d.root = attachNode(d.root, targetNode, sourceNode, edge)
}

func removePanelFromGroup(group *group, panel *Panel) bool {
	for i, p := range group.panels {
		if p == panel {
			group.panels = append(group.panels[:i], group.panels[i+1:]...)
			if group.selected >= len(group.panels) {
				group.selected = len(group.panels) - 1
			}
			return len(group.panels) == 0
		}
	}
	return false
}

func findGroupNode(node *Node, group *group) *Node {
	if node == nil {
		return nil
	}
	if node.group == group {
		return node
	}
	if node.split != nil {
		if n := findGroupNode(node.split.first, group); n != nil {
			return n
		}
		if n := findGroupNode(node.split.second, group); n != nil {
			return n
		}
	}
	return nil
}

func nodeContainingPanels(node *Node, panels []*Panel) *Node {
	if node == nil {
		return nil
	}
	if node.group != nil && groupContainsPanels(node.group, panels) {
		return node
	}
	if node.split != nil {
		if n := nodeContainingPanels(node.split.first, panels); n != nil {
			return n
		}
		return nodeContainingPanels(node.split.second, panels)
	}
	return nil
}

func groupContainingPanels(node *Node, panels []*Panel) *group {
	if n := nodeContainingPanels(node, panels); n != nil {
		return n.group
	}
	return nil
}

func (d *layout) panelsPresent(panels []*Panel) bool {
	if nodeContainsAnyPanels(d.root, panels) {
		return true
	}
	for _, bar := range []*edgeBar{d.left, d.right} {
		if bar != nil && groupContainsAnyPanels(bar.group, panels) {
			return true
		}
	}
	return false
}

func nodeContainsAnyPanels(node *Node, panels []*Panel) bool {
	if node == nil {
		return false
	}
	if node.group != nil {
		return groupContainsAnyPanels(node.group, panels)
	}
	return node.split != nil && (nodeContainsAnyPanels(node.split.first, panels) || nodeContainsAnyPanels(node.split.second, panels))
}

func (d *layout) groupContainingPanels(panels []*Panel) *group {
	if target := groupContainingPanels(d.root, panels); target != nil {
		return target
	}
	for _, bar := range []*edgeBar{d.left, d.right} {
		if bar != nil && groupContainsPanels(bar.group, panels) {
			return bar.group
		}
	}
	return nil
}

func groupContainsPanels(group *group, panels []*Panel) bool {
	if group == nil || len(panels) == 0 {
		return false
	}
	for _, panel := range panels {
		if !slices.Contains(group.panels, panel) {
			return false
		}
	}
	return true
}

func groupContainsAnyPanels(group *group, panels []*Panel) bool {
	if group == nil {
		return false
	}
	for _, panel := range panels {
		if slices.Contains(group.panels, panel) {
			return true
		}
	}
	return false
}

func removePanels(node *Node, panels []*Panel) *Node {
	if node == nil {
		return nil
	}
	if node.group != nil {
		node.group.panels = slices.DeleteFunc(node.group.panels, func(panel *Panel) bool {
			return slices.Contains(panels, panel)
		})
		if len(node.group.panels) == 0 {
			return nil
		}
		node.group.selected = min(node.group.selected, len(node.group.panels)-1)
		return node
	}
	if node.split == nil {
		return node
	}
	node.split.first = removePanels(node.split.first, panels)
	node.split.second = removePanels(node.split.second, panels)
	switch {
	case node.split.first == nil:
		return node.split.second
	case node.split.second == nil:
		return node.split.first
	default:
		return node
	}
}

// removeNode returns node's subtree with the target node removed, collapsing
// the split that directly contained it so the sibling keeps its own identity.
func removeNode(node, target *Node) *Node {
	if node == target {
		return nil
	}
	if node.split == nil {
		return node
	}
	if node.split.first == target {
		return node.split.second
	}
	if node.split.second == target {
		return node.split.first
	}
	node.split.first = removeNode(node.split.first, target)
	node.split.second = removeNode(node.split.second, target)
	return node
}

func attachNode(root, target, newNode *Node, edge dropEdge) *Node {
	direction, newNodeFirst := splitForEdge(edge)
	newSplit := &Node{
		split: &split{
			direction: direction,
			ratio:     0.5,
		},
	}
	if newNodeFirst {
		newSplit.split.first = newNode
		newSplit.split.second = target
	} else {
		newSplit.split.first = target
		newSplit.split.second = newNode
	}
	if root == target {
		return newSplit
	}
	replaceNode(root, target, newSplit)
	return root
}

func splitForEdge(edge dropEdge) (Direction, bool) {
	switch edge {
	case dropEdgeLeft:
		return Horizontal, true
	case dropEdgeRight:
		return Horizontal, false
	case dropEdgeTop:
		return Vertical, true
	case dropEdgeBottom:
		return Vertical, false
	}
	return Horizontal, false
}

func replaceNode(node, target, replacement *Node) {
	if node.split == nil {
		return
	}
	if node.split.first == target {
		node.split.first = replacement
		return
	}
	if node.split.second == target {
		node.split.second = replacement
		return
	}
	replaceNode(node.split.first, target, replacement)
	replaceNode(node.split.second, target, replacement)
}

// dropEdgeAt returns the drop zone the cursor falls in within bounds. Dropping
// on the tab bar (the top strip) tabs the panel into the group; the outer
// thirds are the four edge-split targets; the rest is the center.
func (d *layout) dropEdgeAt(context *guigui.Context, cursor image.Point, b image.Rectangle) dropEdge {
	u := basicwidget.UnitSize(context)
	if cursor.Y < b.Min.Y+u {
		return dropEdgeCenter
	}
	edgeX := b.Dx() / 3
	edgeY := b.Dy() / 3
	switch {
	case cursor.Y < b.Min.Y+edgeY:
		return dropEdgeTop
	case cursor.Y >= b.Max.Y-edgeY:
		return dropEdgeBottom
	case cursor.X < b.Min.X+edgeX:
		return dropEdgeLeft
	case cursor.X >= b.Max.X-edgeX:
		return dropEdgeRight
	default:
		return dropEdgeCenter
	}
}

func (d *layout) dropRectFor(context *guigui.Context, cursor image.Point, b image.Rectangle, edge dropEdge, group *group) image.Rectangle {
	u := basicwidget.UnitSize(context)
	edgeX := b.Dx() / 3
	edgeY := b.Dy() / 3
	switch edge {
	case dropEdgeLeft:
		return image.Rectangle{Min: b.Min, Max: image.Pt(b.Min.X+edgeX, b.Max.Y)}
	case dropEdgeRight:
		return image.Rectangle{Min: image.Pt(b.Max.X-edgeX, b.Min.Y), Max: b.Max}
	case dropEdgeTop:
		return image.Rectangle{Min: b.Min, Max: image.Pt(b.Max.X, b.Min.Y+edgeY)}
	case dropEdgeBottom:
		return image.Rectangle{Min: image.Pt(b.Min.X, b.Max.Y-edgeY), Max: b.Max}
	case dropEdgeCenter:
		if cursor.Y < b.Min.Y+u {
			// The panel is appended as the last tab, so highlight only the
			// empty part of the tab bar after the existing tabs.
			start := max(b.Min.X, group.tabBarUsed)
			if start >= b.Max.X {
				return image.Rectangle{}
			}
			return image.Rectangle{Min: image.Pt(start, b.Min.Y), Max: image.Pt(b.Max.X, b.Min.Y+u)}
		}
		return image.Rectangle{
			Min: image.Pt(b.Min.X+edgeX, b.Min.Y+edgeY),
			Max: image.Pt(b.Max.X-edgeX, b.Max.Y-edgeY),
		}
	}
	return image.Rectangle{}
}

func (d *layout) updateRatio(cursor image.Point) {
	if d.dragging == nil || d.dragAvailable <= 0 {
		return
	}
	var delta int
	switch d.dragging.direction {
	case Horizontal:
		delta = cursor.X - d.dragOrigin.X
	case Vertical:
		delta = cursor.Y - d.dragOrigin.Y
	}
	ratio := d.dragStartRatio + float64(delta)/float64(d.dragAvailable)
	d.dragging.ratio = min(max(ratio, 0.1), 0.9)
}

func (d *layout) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if d.dragPanel != nil || d.dragGroupNode != nil {
		return 0, false
	}
	if d.dragging != nil {
		return resizeCursorFor(d.dragging.direction), true
	}
	cursor := image.Pt(ebiten.CursorPosition())
	for i := range d.dividers {
		if cursor.In(d.dividers[i].bounds) {
			return resizeCursorFor(d.dividers[i].split.direction), true
		}
	}
	return 0, false
}

func resizeCursorFor(direction Direction) ebiten.CursorShapeType {
	if direction == Horizontal {
		return ebiten.CursorShapeEWResize
	}
	return ebiten.CursorShapeNSResize
}

func (d *layout) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	var clr color.RGBA
	if context.ColorMode() == ebiten.ColorModeLight {
		clr = color.RGBA{0x80, 0x80, 0x80, 0xff}
	} else {
		clr = color.RGBA{0x40, 0x40, 0x40, 0xff}
	}
	for _, div := range d.dividers {
		b := div.bounds
		vector.FillRect(dst, float32(b.Min.X), float32(b.Min.Y), float32(b.Dx()), float32(b.Dy()), clr, false)
	}
}
