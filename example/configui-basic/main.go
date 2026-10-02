// Command configui-basic is a small configui.Editor: two fields and one nested page.
package main

import (
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"slices"

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
	app := &root{cfg: defaultConfig(), path: path}
	if cfg, err := loadConfig(path); err == nil {
		app.cfg = cfg
		fmt.Println("loaded", path)
	} else if !os.IsNotExist(err) {
		fmt.Println("load:", err)
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

	background basicwidget.Background
	load       basicwidget.Button
	save       basicwidget.Button
	settings   basicwidget.Button
	editor     configui.Editor[config]

	buttonItems  []guigui.LinearLayoutItem
	buttonLayout guigui.LinearLayout
	items        []guigui.LinearLayoutItem
}

func (r *root) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&r.background)
	adder.AddWidget(&r.load)
	adder.AddWidget(&r.save)
	adder.AddWidget(&r.settings)
	adder.AddWidget(&r.editor)

	r.editor.SetTarget(&r.cfg)

	r.load.SetText("Load")
	r.load.OnDown(func(context *guigui.Context) {
		cfg, err := loadConfig(r.path)
		if err != nil {
			fmt.Println("load:", err)
			return
		}
		r.cfg = cfg
		fmt.Println("loaded", r.path)
	})

	r.save.SetText("Save")
	r.save.OnDown(func(context *guigui.Context) {
		if err := saveConfig(r.path, r.cfg); err != nil {
			fmt.Println("save:", err)
			return
		}
		fmt.Println("saved", r.path)
	})

	r.settings.SetText("Settings")
	r.settings.OnDown(func(context *guigui.Context) {
		r.editor.SetOpen(true)
	})
	context.SetEnabled(&r.settings, !r.editor.IsOpen())
	return nil
}

func (r *root) layout(context *guigui.Context) guigui.LinearLayout {
	u := basicwidget.UnitSize(context)
	r.buttonItems = slices.Delete(r.buttonItems, 0, len(r.buttonItems))
	r.buttonItems = append(r.buttonItems,
		guigui.LinearLayoutItem{Widget: &r.load},
		guigui.LinearLayoutItem{Widget: &r.save},
		guigui.LinearLayoutItem{Widget: &r.settings},
	)
	r.buttonLayout = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Gap:       u / 2,
		Items:     r.buttonItems,
	}
	r.items = slices.Delete(r.items, 0, len(r.items))
	r.items = append(r.items,
		guigui.LinearLayoutItem{Layout: &r.buttonLayout},
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1)},
	)
	return guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     r.items,
		Padding:   guigui.Padding{Start: u / 2, Top: u / 2, End: u / 2, Bottom: u / 2},
	}
}

func (r *root) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	bounds := widgetBounds.Bounds()
	layouter.LayoutWidget(&r.background, bounds)
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
	return cfg, nil
}

func saveConfig(path string, cfg config) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}
