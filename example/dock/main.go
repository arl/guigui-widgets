package main

import (
	"fmt"
	"image"
	"os"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/arl/guigui-widgets/dock"
)

// Root hosts the docking layout that the example showcases.
type Root struct {
	guigui.DefaultWidget

	background basicwidget.Background
	menubar    basicwidget.Menubar[string]
	dock       *dock.Root

	editor     editorPanel
	form       formPanel
	properties propertiesPanel
	console    consolePanel
	paneview   paneViewPanel
	settings   settingsPanel

	formNode     *dock.Node
	editorNode   *dock.Node
	consoleNode  *dock.Node
	paneViewNode *dock.Node
	settingsNode *dock.Node
}

func (r *Root) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&r.background)
	adder.AddWidget(&r.menubar)
	adder.AddWidget(r.dock)

	nodes := []struct {
		text string
		node *dock.Node
	}{
		{text: "Form and Properties", node: r.formNode},
		{text: "Editor", node: r.editorNode},
		{text: "Console", node: r.consoleNode},
		{text: "PaneView", node: r.paneViewNode},
		{text: "Settings", node: r.settingsNode},
	}
	r.menubar.SetItems([]basicwidget.MenubarItem{{Text: "Panels"}, {Text: "File"}})
	panelsMenuItems := make([]basicwidget.PopupMenuItem[string], 0, len(nodes))
	for _, item := range nodes {
		panelsMenuItems = append(panelsMenuItems, basicwidget.PopupMenuItem[string]{
			Text:    item.text,
			Checked: r.dock.Contains(item.node),
			Value:   item.text,
		})
	}
	const (
		idxPanels = iota
		idxFile
	)

	r.menubar.PopupMenuAt(idxPanels).SetItems(panelsMenuItems)
	r.menubar.PopupMenuAt(idxFile).SetItems([]basicwidget.PopupMenuItem[string]{
		{Text: "Load", Value: "load"},
		{Text: "Save", Value: "save"},
	})

	r.menubar.OnItemSelected(func(context *guigui.Context, menuIndex, itemIndex int) {
		switch menuIndex {
		case idxPanels:
			if itemIndex >= 0 && itemIndex < len(nodes) {
				r.toggleNode(nodes[itemIndex].node)
			}
		case idxFile:
			switch itemIndex {
			case 0: // load
				buf, err := os.ReadFile("dock.json")
				if err != nil {
					fmt.Fprintln(os.Stderr, "load dock.json:", err)
					return
				}
				if err := r.dock.ApplyJSON(buf); err != nil {
					fmt.Fprintln(os.Stderr, "apply dock.json:", err)
					return
				}
				fmt.Println("loaded dock.json")

			case 1: // save
				buf, err := r.dock.MarshalJSON()
				if err != nil {
					fmt.Fprintln(os.Stderr, "save dock.json:", err)
					return
				}
				if err := os.WriteFile("dock.json", buf, 0o744); err != nil {
					fmt.Fprintln(os.Stderr, "save dock.json:", err)
					return
				}
				fmt.Println("saved dock.json")
			}
		}
	})
	return nil
}

func (r *Root) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	b := widgetBounds.Bounds()
	layouter.LayoutWidget(&r.background, b)
	menuHeight := r.menubar.Measure(context, guigui.Constraints{}).Y
	menuBounds := image.Rect(b.Min.X, b.Min.Y, b.Max.X, b.Min.Y+menuHeight)
	layouter.LayoutWidget(&r.menubar, menuBounds)
	layouter.LayoutWidget(r.dock, image.Rect(b.Min.X, menuBounds.Max.Y, b.Max.X, b.Max.Y))
}

func (r *Root) toggleNode(node *dock.Node) {
	if r.dock.Contains(node) {
		r.dock.Remove(node)
		return
	}
	for _, target := range []*dock.Node{r.formNode, r.editorNode, r.paneViewNode, r.settingsNode, r.consoleNode} {
		if target != node && r.dock.Contains(target) {
			r.dock.Add(node, target, dock.Center)
			return
		}
	}
	r.dock.Add(node, nil, dock.Center)
}

// paneViewPanel hosts a PaneView to showcase the vertical stack of collapsible panes.
type paneViewPanel struct {
	guigui.DefaultWidget

	paneView dock.PaneView

	pane1 basicwidget.TextInput
	pane2 basicwidget.TextInput
	pane3 basicwidget.TextInput
}

func (p *paneViewPanel) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&p.paneView)

	p.pane1.SetMultiline(true)
	p.pane1.SetWrapMode(basicwidget.WrapModeNormal)
	p.pane1.SetPlaceholder("Pane 1 content…")
	p.pane2.SetMultiline(true)
	p.pane2.SetWrapMode(basicwidget.WrapModeNormal)
	p.pane2.SetPlaceholder("Pane 2 content…")
	p.pane3.SetMultiline(true)
	p.pane3.SetWrapMode(basicwidget.WrapModeNormal)
	p.pane3.SetPlaceholder("Pane 3 content…")

	if p.paneView.Len() == 0 {
		p.paneView.AddPane(dock.NewPane("First", &p.pane1))
		p.paneView.AddPane(dock.NewPane("Second", &p.pane2))
		p.paneView.AddPane(dock.NewPane("Third", &p.pane3))
	}
	return nil
}

func (p *paneViewPanel) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&p.paneView, widgetBounds.Bounds())
}

// editorPanel hosts a multiline rich-text editor.
type editorPanel struct {
	guigui.DefaultWidget

	editor      basicwidget.TextInput
	initialized bool
}

func (e *editorPanel) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&e.editor)
	e.editor.SetMultiline(true)
	e.editor.SetWrapMode(basicwidget.WrapModeNormal)
	e.editor.SetLineHeightMode(basicwidget.LineHeightModeFlexible)
	e.editor.SetRichTextEditable(true)
	e.editor.SetPlaceholder("Start writing…")

	// Seed the sample content once, so a later rebuild does not overwrite the
	// user's edits.
	if !e.initialized {
		e.editor.SetValue("A dockable rich-text editor.\n\nSelect text to apply inline styles.")
		e.initialized = true
	}
	return nil
}

func (e *editorPanel) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	layouter.LayoutWidget(&e.editor, widgetBounds.Bounds().Inset(u/2))
}

// formPanel hosts a form with a variety of controls. Its control state lives
// in the struct fields below, updated by the controls' callbacks, so a rebuild
// restores the user's choices rather than resetting them.
type formPanel struct {
	guigui.DefaultWidget

	panel basicwidget.Panel
	form  basicwidget.Form

	nameText  basicwidget.Text
	nameInput basicwidget.TextInput

	priorityGroup basicwidget.RadioButtonGroup[string]
	priorityTexts [3]basicwidget.Text

	enabledText  basicwidget.Text
	enabledCheck basicwidget.Checkbox

	themeText   basicwidget.Text
	themeSelect basicwidget.Select[string]

	opacityText  basicwidget.Text
	opacitySlide basicwidget.Slider

	volumeText  basicwidget.Text
	volumeInput basicwidget.NumberInput

	submitButton basicwidget.Button
	cancelButton basicwidget.Button

	priority string
	enabled  bool
	theme    string
	opacity  int
	volume   int

	initialized bool
}

func (f *formPanel) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&f.panel)
	adder.AddWidget(&f.priorityGroup)

	f.panel.SetContent(&f.form)
	f.panel.SetContentConstraints(basicwidget.PanelContentConstraintsFixedWidth)

	if !f.initialized {
		f.priority = "Medium"
		f.enabled = true
		f.theme = "Light"
		f.opacity = 75
		f.volume = 50
		f.initialized = true
	}

	f.nameText.SetValue("Name")
	f.nameInput.SetPlaceholder("Enter a name")

	f.priorityGroup.SetValues([]string{"Low", "Medium", "High"})
	f.priorityTexts[0].SetValue("Low")
	f.priorityTexts[1].SetValue("Medium")
	f.priorityTexts[2].SetValue("High")
	f.priorityGroup.SelectItemByValue(f.priority)
	f.priorityGroup.OnItemSelected(func(context *guigui.Context, index int) {
		if value, ok := f.priorityGroup.SelectedValue(); ok {
			f.priority = value
		}
	})

	f.enabledText.SetValue("Enabled")
	f.enabledCheck.SetValue(f.enabled)
	f.enabledCheck.OnValueChanged(func(context *guigui.Context, value bool) {
		f.enabled = value
	})

	f.themeText.SetValue("Theme")
	f.themeSelect.SetItemsByStrings([]string{"Light", "Dark", "System"})
	f.themeSelect.SelectItemByValue(f.theme)
	f.themeSelect.OnItemSelected(func(context *guigui.Context, index int) {
		if item, ok := f.themeSelect.SelectedItem(); ok {
			f.theme = item.Value
		}
	})

	f.opacityText.SetValue("Opacity")
	f.opacitySlide.SetMinimumValue(0)
	f.opacitySlide.SetMaximumValue(100)
	f.opacitySlide.SetValue(f.opacity)
	f.opacitySlide.OnValueChanged(func(context *guigui.Context, value int) {
		f.opacity = value
	})

	f.volumeText.SetValue("Volume")
	f.volumeInput.SetMinimumValue(0)
	f.volumeInput.SetMaximumValue(100)
	f.volumeInput.SetValue(f.volume)
	f.volumeInput.OnValueChanged(func(context *guigui.Context, value int, committed bool) {
		f.volume = value
	})

	f.submitButton.SetText("Submit")
	f.cancelButton.SetText("Cancel")

	f.form.SetItems([]basicwidget.FormItem{
		{PrimaryWidget: &f.nameText, SecondaryWidget: &f.nameInput},
		{PrimaryWidget: &f.priorityTexts[0], SecondaryWidget: f.priorityGroup.RadioButton(0)},
		{PrimaryWidget: &f.priorityTexts[1], SecondaryWidget: f.priorityGroup.RadioButton(1)},
		{PrimaryWidget: &f.priorityTexts[2], SecondaryWidget: f.priorityGroup.RadioButton(2)},
		{PrimaryWidget: &f.enabledText, SecondaryWidget: &f.enabledCheck},
		{PrimaryWidget: &f.themeText, SecondaryWidget: &f.themeSelect},
		{PrimaryWidget: &f.opacityText, SecondaryWidget: &f.opacitySlide},
		{PrimaryWidget: &f.volumeText, SecondaryWidget: &f.volumeInput},
		{SecondaryWidget: &f.submitButton},
		{SecondaryWidget: &f.cancelButton},
	})
	return nil
}

func (f *formPanel) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&f.panel, widgetBounds.Bounds())
}

// propertiesPanel holds a few settings, tabbed with the form to showcase
// groups.
type propertiesPanel struct {
	guigui.DefaultWidget

	panel basicwidget.Panel
	form  basicwidget.Form

	wordWrapText   basicwidget.Text
	wordWrapToggle basicwidget.Toggle

	lineNumbersText   basicwidget.Text
	lineNumbersToggle basicwidget.Toggle

	wordWrap    bool
	lineNumbers bool
	initialized bool
}

func (p *propertiesPanel) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&p.panel)

	p.panel.SetContent(&p.form)
	p.panel.SetContentConstraints(basicwidget.PanelContentConstraintsFixedWidth)

	if !p.initialized {
		p.wordWrap = true
		p.lineNumbers = true
		p.initialized = true
	}

	p.wordWrapText.SetValue("Word wrap")
	p.wordWrapToggle.SetValue(p.wordWrap)
	p.wordWrapToggle.OnValueChanged(func(context *guigui.Context, value bool) {
		p.wordWrap = value
	})

	p.lineNumbersText.SetValue("Line numbers")
	p.lineNumbersToggle.SetValue(p.lineNumbers)
	p.lineNumbersToggle.OnValueChanged(func(context *guigui.Context, value bool) {
		p.lineNumbers = value
	})

	p.form.SetItems([]basicwidget.FormItem{
		{PrimaryWidget: &p.wordWrapText, SecondaryWidget: &p.wordWrapToggle},
		{PrimaryWidget: &p.lineNumbersText, SecondaryWidget: &p.lineNumbersToggle},
	})
	return nil
}

func (p *propertiesPanel) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&p.panel, widgetBounds.Bounds())
}

// consolePanel hosts a scratchpad that demonstrates a third docked panel.
type consolePanel struct {
	guigui.DefaultWidget

	console basicwidget.TextInput
}

func (c *consolePanel) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&c.console)
	c.console.SetMultiline(true)
	c.console.SetWrapMode(basicwidget.WrapModeNormal)
	c.console.SetPlaceholder("Type a note…")
	return nil
}

func (c *consolePanel) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	layouter.LayoutWidget(&c.console, widgetBounds.Bounds().Inset(u/2))
}

func main() {
	root := &Root{}
	root.formNode = dock.Group(
		"form-properties",
		&dock.Panel{Title: "Form", Content: &root.form},
		&dock.Panel{Title: "Properties", Content: &root.properties},
	)
	root.editorNode = dock.Group("editor", &dock.Panel{Title: "Editor", Content: &root.editor})
	root.consoleNode = dock.Group("console", &dock.Panel{Title: "Console", Content: &root.console})
	root.paneViewNode = dock.Group("pane-view", &dock.Panel{Title: "PaneView", Content: &root.paneview})
	root.settingsNode = dock.Group("settings", &dock.Panel{Title: "Settings", Content: &root.settings})

	// A vertical split: a tab group (Form | Properties) beside the editor on
	// top, and the console docked at the bottom.
	initialLayout := dock.Split(
		dock.Vertical, 0.7,
		dock.Split(
			dock.Horizontal, 0.35,
			root.formNode,
			root.editorNode,
		),
		dock.Split(
			dock.Horizontal, 0.5,
			root.paneViewNode,
			root.settingsNode,
		),
	)
	var err error
	root.dock, err = dock.NewRoot(initialLayout, root.consoleNode)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return
	}

	if err := guigui.Run(root, &guigui.RunOptions{
		Title:      "Docking",
		WindowSize: image.Pt(960, 640),
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
