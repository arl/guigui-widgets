package main

import (
	"fmt"
	"image"
	"os"
	"slices"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	_ "github.com/guigui-gui/guigui/basicwidget/cjkfont"

	"github.com/arl/guigui-widgets/datepicker"
)

type Root struct {
	guigui.DefaultWidget

	background basicwidget.Background

	introTitle basicwidget.Text
	introBody  basicwidget.Text

	panel basicwidget.Panel
	form  basicwidget.Form

	dockedLabel  basicwidget.Text
	dockedPicker datepicker.Docked
	dockedValue  basicwidget.Text

	modalLabel  basicwidget.Text
	modalPicker datepicker.Modal
	modalValue  basicwidget.Text

	inputLabel  basicwidget.Text
	inputPicker datepicker.ModalDateInput
	inputValue  basicwidget.Text

	layoutItems []guigui.LinearLayoutItem
}

func formatSelected(d datepicker.Date) string {
	if d.IsZero() {
		return "No date selected"
	}
	return d.Time().Format("Monday, January 2, 2006")
}

func (r *Root) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&r.background)
	adder.AddWidget(&r.introTitle)
	adder.AddWidget(&r.introBody)
	adder.AddWidget(&r.panel)

	r.introTitle.SetValue("Date pickers")
	var titleStyle basicwidget.TextStyle
	titleStyle.SetBold(true)
	r.introTitle.SetBaseStyle(&titleStyle)
	r.introTitle.SetScale(1.5)

	r.introBody.SetValue("Three Material 3 date picker kinds: a docked field with a calendar dropdown, a modal calendar dialog, and a modal text-input dialog. Each calendar uses month and year menus, previous/next month, weekday headings, and Cancel/OK.")
	r.introBody.SetWrapMode(basicwidget.WrapModeNormal)

	r.panel.SetContent(&r.form)
	r.panel.SetAutoBorder(true)
	r.panel.SetContentConstraints(basicwidget.PanelContentConstraintsFixedWidth)

	r.dockedLabel.SetValue("Docked")
	r.dockedPicker.SetPlaceholder("DD/MM/YYYY")
	r.dockedPicker.OnValueChanged(func(context *guigui.Context, date datepicker.Date) {
		r.dockedValue.SetValue(formatSelected(date))
	})
	r.dockedValue.SetValue(formatSelected(r.dockedPicker.Value()))

	r.modalLabel.SetValue("Modal")
	r.modalPicker.SetTitle("Select date")
	r.modalPicker.OnValueChanged(func(context *guigui.Context, date datepicker.Date) {
		r.modalValue.SetValue(formatSelected(date))
	})
	r.modalValue.SetValue(formatSelected(r.modalPicker.Value()))

	r.inputLabel.SetValue("Modal input")
	r.inputPicker.SetTitle("Enter date")
	r.inputPicker.OnValueChanged(func(context *guigui.Context, date datepicker.Date) {
		r.inputValue.SetValue(formatSelected(date))
	})
	r.inputValue.SetValue(formatSelected(r.inputPicker.Value()))

	r.form.SetItems([]basicwidget.FormItem{
		{
			PrimaryWidget:   &r.dockedLabel,
			SecondaryWidget: &r.dockedPicker,
		},
		{
			PrimaryWidget:   nil,
			SecondaryWidget: &r.dockedValue,
		},
		{
			PrimaryWidget:   &r.modalLabel,
			SecondaryWidget: &r.modalPicker,
		},
		{
			PrimaryWidget:   nil,
			SecondaryWidget: &r.modalValue,
		},
		{
			PrimaryWidget:   &r.inputLabel,
			SecondaryWidget: &r.inputPicker,
		},
		{
			PrimaryWidget:   nil,
			SecondaryWidget: &r.inputValue,
		},
	})

	return nil
}

func (r *Root) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&r.background, widgetBounds.Bounds())

	u := basicwidget.UnitSize(context)
	r.layoutItems = slices.Delete(r.layoutItems, 0, len(r.layoutItems))
	r.layoutItems = append(
		r.layoutItems,
		guigui.LinearLayoutItem{Widget: &r.introTitle},
		guigui.LinearLayoutItem{Widget: &r.introBody},
		guigui.LinearLayoutItem{
			Widget: &r.panel,
			Size:   guigui.FlexibleSize(1),
		},
	)
	(guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     r.layoutItems,
		Gap:       u / 2,
		Padding: guigui.Padding{
			Start:  u / 2,
			Top:    u / 2,
			End:    u / 2,
			Bottom: u / 2,
		},
	}).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func main() {
	op := &guigui.RunOptions{
		Title:         "Date pickers",
		WindowSize:    image.Pt(720, 640),
		WindowMinSize: image.Pt(560, 480),
	}
	if err := guigui.Run(&Root{}, op); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
