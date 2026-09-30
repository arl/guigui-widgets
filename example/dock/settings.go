package main

import (
	"slices"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/hajimehoshi/ebiten/v2"
	"golang.org/x/text/language"
)

type settingsPanel struct {
	guigui.DefaultWidget

	panel basicwidget.Panel
	form  basicwidget.Form

	colorModeText             basicwidget.Text
	colorModeSegmentedControl basicwidget.SegmentedControl[string]
	localeText                basicwidget.Text
	localeSelect              basicwidget.Select[language.Tag]
	scaleText                 basicwidget.Text
	scaleSegmentedControl     basicwidget.SegmentedControl[float64]

	layoutItems []guigui.LinearLayoutItem
}

func (s *settingsPanel) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&s.panel)

	s.panel.SetContent(&s.form)
	s.panel.SetAutoBorder(true)
	s.panel.SetContentConstraints(basicwidget.PanelContentConstraintsFixedWidth)

	s.colorModeText.SetValue("Color mode")
	s.colorModeSegmentedControl.SetItems([]basicwidget.SegmentedControlItem[string]{
		{Text: "Auto", Value: "auto"},
		{Text: "Light", Value: "light"},
		{Text: "Dark", Value: "dark"},
	})
	s.colorModeSegmentedControl.OnItemSelected(func(context *guigui.Context, index int) {
		item, ok := s.colorModeSegmentedControl.ItemByIndex(index)
		if !ok {
			context.SetPreferredColorMode(ebiten.ColorModeUnknown)
			return
		}
		switch item.Value {
		case "light":
			context.SetPreferredColorMode(ebiten.ColorModeLight)
		case "dark":
			context.SetPreferredColorMode(ebiten.ColorModeDark)
		default:
			context.SetPreferredColorMode(ebiten.ColorModeUnknown)
		}
	})
	switch context.PreferredColorMode() {
	case ebiten.ColorModeLight:
		s.colorModeSegmentedControl.SelectItemByValue("light")
	case ebiten.ColorModeDark:
		s.colorModeSegmentedControl.SelectItemByValue("dark")
	default:
		s.colorModeSegmentedControl.SelectItemByValue("auto")
	}

	s.localeText.SetValue("Locale")
	s.localeSelect.SetItems([]basicwidget.SelectItem[language.Tag]{
		{Text: "(Default)", Value: language.Und},
		{Text: "English", Value: language.English},
		{Text: "Japanese", Value: language.Japanese},
		{Text: "Korean", Value: language.Korean},
		{Text: "Simplified Chinese", Value: language.SimplifiedChinese},
		{Text: "Traditional Chinese", Value: language.TraditionalChinese},
	})
	s.localeSelect.OnItemSelected(func(context *guigui.Context, index int) {
		item, ok := s.localeSelect.ItemByIndex(index)
		if !ok || item.Value == language.Und {
			context.SetAppLocales(nil)
			return
		}
		context.SetAppLocales([]language.Tag{item.Value})
	})
	if !s.localeSelect.IsPopupOpen() {
		locales := context.AppendAppLocales(nil)
		if len(locales) > 0 {
			s.localeSelect.SelectItemByValue(locales[0])
		} else {
			s.localeSelect.SelectItemByValue(language.Und)
		}
	}

	s.scaleText.SetValue("Scale")
	s.scaleSegmentedControl.SetItems([]basicwidget.SegmentedControlItem[float64]{
		{Text: "80%", Value: 0.8},
		{Text: "100%", Value: 1},
		{Text: "120%", Value: 1.2},
	})
	s.scaleSegmentedControl.OnItemSelected(func(context *guigui.Context, index int) {
		item, ok := s.scaleSegmentedControl.ItemByIndex(index)
		if !ok {
			context.SetAppScale(1)
			return
		}
		context.SetAppScale(item.Value)
	})
	s.scaleSegmentedControl.SelectItemByValue(context.AppScale())

	s.form.SetItems([]basicwidget.FormItem{
		{PrimaryWidget: &s.colorModeText, SecondaryWidget: &s.colorModeSegmentedControl},
		{PrimaryWidget: &s.localeText, SecondaryWidget: &s.localeSelect},
		{PrimaryWidget: &s.scaleText, SecondaryWidget: &s.scaleSegmentedControl},
	})
	return nil
}

func (s *settingsPanel) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	u := basicwidget.UnitSize(context)
	s.layoutItems = slices.Delete(s.layoutItems, 0, len(s.layoutItems))
	s.layoutItems = append(s.layoutItems, guigui.LinearLayoutItem{
		Widget: &s.panel,
		Size:   guigui.FlexibleSize(1),
	})
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     s.layoutItems,
		Gap:       u / 2,
		Padding: guigui.Padding{
			Start:  u / 2,
			Top:    u / 2,
			End:    u / 2,
			Bottom: u / 2,
		},
	}).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}
