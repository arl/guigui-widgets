package dock

import (
	"encoding/json"
	"image"
	"strings"
	"testing"
)

func TestRootSnapshotRoundTrip(t *testing.T) {
	first := Group("first", &Panel{Title: "First"})
	second := Group("second", &Panel{Title: "Second"})
	absent := Group("absent", &Panel{Title: "Absent"})
	root, err := NewRoot(Split(Horizontal, 0.4, first, second), absent)
	if err != nil {
		t.Fatal(err)
	}

	data, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	var snapshot rootSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		t.Fatal(err)
	}
	if snapshot.Version != 1 || snapshot.Tree == nil || snapshot.Tree.Split == nil {
		t.Fatalf("unexpected snapshot: %#v", snapshot)
	}
	if len(snapshot.Tree.Split.First.Group.Nodes) != 1 || snapshot.Tree.Split.First.Group.Nodes[0] != "first" {
		t.Fatalf("first node was not serialized: %#v", snapshot.Tree.Split.First.Group)
	}
	if _, _, _, err := root.layoutFromSnapshot(&snapshot); err != nil {
		t.Fatalf("round trip validation failed: %v", err)
	}
}

func TestRootSnapshotRejectsUnknownAndDuplicateNodes(t *testing.T) {
	first := Group("first", &Panel{Title: "First"})
	second := Group("second", &Panel{Title: "Second"})
	root, err := NewRoot(Split(Horizontal, 0.4, first, second))
	if err != nil {
		t.Fatal(err)
	}

	unknown := &rootSnapshot{
		Version: 1,
		Tree:    &snapshotNode{Group: &snapshotTabGroup{Nodes: []string{"missing"}, Selected: 0}},
	}
	if _, _, _, err := root.layoutFromSnapshot(unknown); err == nil || !strings.Contains(err.Error(), "unknown node ID") {
		t.Fatalf("unknown node error = %v, want unknown node ID", err)
	}

	duplicate := &rootSnapshot{
		Version: 1,
		Tree:    &snapshotNode{Group: &snapshotTabGroup{Nodes: []string{"first", "first"}, Selected: 0}},
	}
	if _, _, _, err := root.layoutFromSnapshot(duplicate); err == nil || !strings.Contains(err.Error(), "appears more than once") {
		t.Fatalf("duplicate node error = %v, want duplicate node", err)
	}
}

func TestRootApplyJSONIsAtomic(t *testing.T) {
	first := Group("first", &Panel{Title: "First"})
	second := Group("second", &Panel{Title: "Second"})
	root, err := NewRoot(Split(Horizontal, 0.4, first, second))
	if err != nil {
		t.Fatal(err)
	}

	valid, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := root.ApplyJSON(valid); err != nil {
		t.Fatalf("ApplyJSON(valid): %v", err)
	}
	before, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	invalid, err := json.Marshal(rootSnapshot{
		Version: 1,
		Tree:    &snapshotNode{Group: &snapshotTabGroup{Nodes: []string{"missing"}, Selected: 0}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := root.ApplyJSON(invalid); err == nil {
		t.Fatal("ApplyJSON(invalid) succeeded")
	}
	after, err := json.Marshal(root)
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Fatalf("invalid restore mutated layout:\n got %s\nwant %s", after, before)
	}
}

func TestRootMarshalRejectsUnregisteredPanelWithoutPanicking(t *testing.T) {
	registered := &Panel{Title: "Registered"}
	root, err := NewRoot(Group("registered", registered))
	if err != nil {
		t.Fatal(err)
	}

	root.layout.root.group.panels = append(root.layout.root.group.panels, &Panel{Title: "Unregistered"})

	defer func() {
		if recovered := recover(); recovered != nil {
			t.Fatalf("MarshalJSON panicked: %v", recovered)
		}
	}()
	if _, err := json.Marshal(root); err == nil || !strings.Contains(err.Error(), "without a registered node") {
		t.Fatalf("MarshalJSON error = %v, want unregistered panel error", err)
	}
}

func TestRootReplaceTreeHidesRegisteredLeaves(t *testing.T) {
	emu := LockedGroup("emulator", &Panel{Title: "Emulator"})
	cpu := Group("cpu", &Panel{Title: "CPU"})
	root, err := NewRoot(emu, cpu)
	if err != nil {
		t.Fatal(err)
	}
	if root.Contains(cpu) {
		t.Fatal("cpu extra should start absent")
	}
	root.ReplaceTree(Split(Horizontal, 0.5, emu, cpu))
	if !root.Contains(cpu) || !root.Contains(emu) {
		t.Fatal("debug tree should show both leaves")
	}
	root.ReplaceTree(emu)
	if root.Contains(cpu) {
		t.Fatal("play tree should hide cpu")
	}
	if !root.Contains(emu) {
		t.Fatal("play tree should keep emulator")
	}
}

func TestRootSetEdgeBarShowsRegisteredLeaves(t *testing.T) {
	emu := LockedGroup("emulator", &Panel{Title: "Emulator"})
	bp := Group("breakpoints", &Panel{Title: "Breakpoints"})
	wp := Group("watchpoints", &Panel{Title: "Watchpoints"})
	root, err := NewRoot(emu, bp, wp)
	if err != nil {
		t.Fatal(err)
	}
	root.SetEdgeBar(Right, bp, wp)
	if !root.Contains(bp) || !root.Contains(wp) {
		t.Fatal("edge bar should show both leaves")
	}
	if !root.Contains(emu) {
		t.Fatal("emulator should stay in the tree")
	}

	data, err := root.MarshalJSON()
	if err != nil {
		t.Fatal(err)
	}
	root.ReplaceTree(emu)
	if root.Contains(bp) || root.Contains(wp) {
		t.Fatal("ReplaceTree should clear the edge bar")
	}
	if err := root.ApplyJSON(data); err != nil {
		t.Fatal(err)
	}
	if !root.Contains(bp) || !root.Contains(wp) {
		t.Fatal("restored snapshot should bring the edge bar back")
	}
}

func TestRootClosePanelRemovesEdgeBarTab(t *testing.T) {
	emu := LockedGroup("emulator", &Panel{Title: "Emulator"})
	bp := Group("breakpoints", &Panel{Title: "Breakpoints"})
	wp := Group("watchpoints", &Panel{Title: "Watchpoints"})
	root, err := NewRoot(emu, bp, wp)
	if err != nil {
		t.Fatal(err)
	}
	root.SetEdgeBar(Right, bp, wp)

	var closed string
	root.SetOnPanelClosed(func(n *Node) {
		closed = n.ID
	})
	root.closePanel(bp.panels[0])
	if closed != "breakpoints" {
		t.Fatalf("closed = %q, want breakpoints", closed)
	}
	if root.Contains(bp) {
		t.Fatal("closed edge-bar tab should leave the layout")
	}
	if !root.Contains(wp) {
		t.Fatal("sibling edge-bar tab should remain")
	}
	if !root.Contains(emu) {
		t.Fatal("emulator should stay in the tree")
	}

	root.closePanel(wp.panels[0])
	if root.Contains(wp) {
		t.Fatal("last edge-bar tab should close the bar")
	}
	if root.layout.right != nil {
		t.Fatal("empty right edge bar should be removed")
	}
}

func TestRootClosePanelRemovesGroupTab(t *testing.T) {
	emu := LockedGroup("emulator", &Panel{Title: "Emulator"})
	cpu := Group("cpu", &Panel{Title: "CPU"})
	root, err := NewRoot(Split(Horizontal, 0.5, emu, cpu))
	if err != nil {
		t.Fatal(err)
	}
	root.closePanel(emu.panels[0])
	if !root.Contains(emu) {
		t.Fatal("locked panel must not close")
	}
	root.closePanel(cpu.panels[0])
	if root.Contains(cpu) {
		t.Fatal("group tab should close")
	}
	if !root.Contains(emu) {
		t.Fatal("emulator should stay")
	}
}

func TestGroupTabCloseRectPlacement(t *testing.T) {
	u := 24
	vert := &groupTab{
		group: &group{vertical: true, onTabClose: func(*Panel) {}},
		panel: &Panel{Title: "Logs"},
	}
	vb := image.Rect(0, 0, u, 80)
	vcr := vert.closeRect(vb, u)
	if vcr.Empty() || vcr.Min.Y < vb.Min.Y+vb.Dy()/2 {
		t.Fatalf("vertical close should sit below the title: %v in %v", vcr, vb)
	}

	horiz := &groupTab{
		group: &group{onTabClose: func(*Panel) {}},
		panel: &Panel{Title: "CPU"},
	}
	hb := image.Rect(0, 0, 80, u)
	hcr := horiz.closeRect(hb, u)
	if hcr.Empty() || hcr.Min.X < hb.Min.X+hb.Dx()/2 {
		t.Fatalf("horizontal close should sit at the end of the title: %v in %v", hcr, hb)
	}

	locked := &groupTab{
		group: &group{locked: true, onTabClose: func(*Panel) {}},
		panel: &Panel{Title: "Emulator"},
	}
	if !locked.closeRect(hb, u).Empty() {
		t.Fatal("locked tabs must not show a close control")
	}
}
