// Command configui-advanced shows labels, ranges, an enum, nested groups, and a custom page.
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
	ProfileName string `configui:"label=Profile name"`
	Verbose     bool   `configui:"label=Write debug logs"`
	MaxRetries  int    `configui:"min=1,max=10"`
	ColorMode   colorMode
	Network     networkSettings
	Banner      banner
}

type colorMode string

const (
	colorModeSystem colorMode = "system"
	colorModeLight  colorMode = "light"
	colorModeDark   colorMode = "dark"
)

func (colorMode) EnumOptions() []configui.EnumOption {
	return []configui.EnumOption{
		{Value: string(colorModeSystem), Label: "System"},
		{Value: string(colorModeLight), Label: "Light"},
		{Value: string(colorModeDark), Label: "Dark"},
	}
}

type networkSettings struct {
	Host   string
	Port   int `configui:"min=1,max=65535"`
	UseTLS bool
	Proxy  proxySettings
}

type proxySettings struct {
	Enabled bool
	Address string
}

// banner is drawn by bannerEditor. A struct that implements CustomGroup is a
// sidebar page you render yourself, instead of a generated form.
type banner struct {
	Message   string
	Emphasize bool
}

func (banner) NewConfigWidget() configui.CustomWidget {
	return &bannerEditor{}
}

func defaultConfig() config {
	return config{
		ProfileName: "Guest",
		MaxRetries:  3,
		ColorMode:   colorModeSystem,
		Network: networkSettings{
			Host:  "localhost",
			Port:  8080,
			Proxy: proxySettings{Address: "http://proxy.example:3128"},
		},
		Banner: banner{Message: "Welcome back", Emphasize: true},
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

// bannerEditor is the custom page for banner. Bind is given *banner.
type bannerEditor struct {
	guigui.DefaultWidget

	target *banner

	form           basicwidget.Form
	formItems      []basicwidget.FormItem
	messageLabel   basicwidget.Text
	message        basicwidget.TextInput
	emphasizeLabel basicwidget.Text
	emphasize      basicwidget.Checkbox
	previewLabel   basicwidget.Text
	preview        basicwidget.Text
}

func (e *bannerEditor) Bind(v any) {
	e.target = v.(*banner)
}

func (e *bannerEditor) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&e.form)

	message, emphasize := "", false
	if e.target != nil {
		message, emphasize = e.target.Message, e.target.Emphasize
	}

	e.messageLabel.SetValue("Message")
	e.message.SetValue(message)
	e.message.OnValueChanged(func(context *guigui.Context, text string, committed bool) {
		if committed && e.target != nil {
			e.target.Message = text
		}
	})

	e.emphasizeLabel.SetValue("Emphasize")
	e.emphasize.SetValue(emphasize)
	e.emphasize.OnValueChanged(func(context *guigui.Context, checked bool) {
		if e.target != nil {
			e.target.Emphasize = checked
		}
	})

	preview := message
	if preview == "" {
		preview = "Preview"
	}
	e.previewLabel.SetValue("Preview")
	e.preview.SetValue(preview)
	var style basicwidget.TextStyle
	style.SetBold(emphasize)
	e.preview.SetBaseStyle(&style)

	e.formItems = slices.Delete(e.formItems, 0, len(e.formItems))
	e.formItems = append(e.formItems,
		basicwidget.FormItem{PrimaryWidget: &e.messageLabel, SecondaryWidget: &e.message},
		basicwidget.FormItem{PrimaryWidget: &e.emphasizeLabel, SecondaryWidget: &e.emphasize},
		basicwidget.FormItem{PrimaryWidget: &e.previewLabel, SecondaryWidget: &e.preview},
	)
	e.form.SetItems(e.formItems)
	return nil
}

func (e *bannerEditor) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&e.form, widgetBounds.Bounds())
}

func (e *bannerEditor) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return e.form.Measure(context, constraints)
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

func main() {
	path, err := filepath.Abs("config-advanced.json")
	if err != nil {
		path = "config-advanced.json"
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
