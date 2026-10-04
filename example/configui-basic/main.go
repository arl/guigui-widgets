// Command configui-basic is a small configui.Editor: two fields and one nested page.
package main

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"

	"github.com/arl/guigui-widgets/configui"
)

type config struct {
	Name   string
	Debug  bool
	Window window // nested page
}

type window struct {
	Width  int
	Height int
}

func defaultConfig() config {
	return config{
		Name:   "Guest",
		Window: window{Width: 1280, Height: 720},
	}
}

func main() {
	path, err := filepath.Abs("config-basic.json")
	if err != nil {
		path = "config-basic.json"
	}

	app := &root{
		cfg:  defaultConfig(),
		path: path,
	}

	if cfg, err := loadConfig(path); err != nil {
		fmt.Fprintln(os.Stderr, "read error:", err)
		fmt.Println("using default config")
	} else {
		app.cfg = cfg
	}

	if err := guigui.Run(app, &guigui.RunOptions{
		Title:         "Config",
		WindowSize:    image.Pt(800, 600),
		WindowMinSize: image.Pt(640, 480),
	}); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

type root struct {
	guigui.DefaultWidget

	cfg  config
	path string

	settings basicwidget.Button
	editor   configui.Editor[config]

	buttonLayout guigui.LinearLayout
	items        []guigui.LinearLayoutItem
}

func (r *root) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&r.settings)
	adder.AddWidget(&r.editor)

	r.editor.SetTarget(&r.cfg)

	r.settings.SetText("Settings")
	r.settings.OnDown(func(context *guigui.Context) {
		r.editor.SetOpen(true)
	})
	r.editor.OnClose(func(context *guigui.Context, reason basicwidget.PopupCloseReason) {
		if err := saveConfig(r.path, r.cfg); err != nil {
			fmt.Fprintln(os.Stderr, "write error:", err)
		}
	})
	context.SetEnabled(&r.settings, !r.editor.IsOpen())
	return nil
}

func (r *root) layout(context *guigui.Context) guigui.LinearLayout {
	r.buttonLayout = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Items: []guigui.LinearLayoutItem{
			{Size: guigui.FlexibleSize(1)},
			{Widget: &r.settings},
			{Size: guigui.FlexibleSize(1)},
		},
	}
	r.items = []guigui.LinearLayoutItem{
		{Size: guigui.FlexibleSize(1)},
		{Layout: &r.buttonLayout},
		{Size: guigui.FlexibleSize(1)},
	}
	return guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     r.items,
	}
}

func (r *root) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	bounds := widgetBounds.Bounds()
	r.layout(context).LayoutWidgets(context, bounds, layouter)
	layouter.LayoutWidget(&r.editor, bounds)
}

func loadConfig(path string) (config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return config{}, err
	}
	var cfg config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return config{}, err
	}

	fmt.Println("read", path)
	return cfg, nil
}

func saveConfig(path string, cfg config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return err
	}

	fmt.Println("written", path)
	return nil
}
