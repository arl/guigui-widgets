package configui

import (
	"image"
	"reflect"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
)

// groupWidget renders the leaf settings of one configuration group as a
// [basicwidget.Form]. Nested groups are not shown here; they appear as nodes in
// the settings tree instead. The set of rows is fixed for the widget's
// lifetime.
type groupWidget struct {
	guigui.DefaultWidget

	schema  *schema
	value   reflect.Value
	hasRows bool
	custom  CustomWidget

	rows []row

	form      basicwidget.Form
	formItems []basicwidget.FormItem
}

type row struct {
	field field
	fv    reflect.Value

	label     basicwidget.Text
	boolCtl   basicwidget.Checkbox
	stringCtl basicwidget.TextInput
	intCtl    basicwidget.NumberInput
	enumCtl   basicwidget.Select[string]
}

func newGroupWidget(schema *schema) *groupWidget {
	g := &groupWidget{schema: schema}
	if len(schema.fields) == 1 && schema.fields[0].Kind == kindCustom && schema.fields[0].newCustom != nil {
		g.custom = schema.fields[0].newCustom()
		g.hasRows = true
		return g
	}
	for _, f := range schema.fields {
		if f.Kind == kindGroup || f.Kind == kindCustom {
			continue
		}
		g.rows = append(g.rows, row{field: f})
		g.hasRows = true
	}
	return g
}

func (g *groupWidget) setValue(v reflect.Value) {
	g.value = v
}

func (g *groupWidget) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	if g.custom != nil {
		adder.AddWidget(g.custom)
		if g.value.IsValid() && g.value.CanAddr() {
			g.custom.Bind(g.value.Addr().Interface())
		}
		return nil
	}

	g.formItems = g.formItems[:0]

	for i := range g.rows {
		r := &g.rows[i]
		r.fv = g.value.Field(r.field.index)

		adder.AddWidget(&r.label)
		r.label.SetValue(r.field.label)

		var secondary guigui.Widget
		switch r.field.Kind {
		case kindBool:
			adder.AddWidget(&r.boolCtl)
			secondary = &r.boolCtl
			r.boolCtl.SetValue(r.fv.Bool())
			fv := r.fv
			r.boolCtl.OnValueChanged(func(context *guigui.Context, v bool) {
				fv.SetBool(v)
			})
		case kindString:
			adder.AddWidget(&r.stringCtl)
			secondary = &r.stringCtl
			r.stringCtl.SetValue(r.fv.String())
			fv := r.fv
			r.stringCtl.OnValueChanged(func(context *guigui.Context, v string, committed bool) {
				if committed {
					fv.SetString(v)
				}
			})
		case kindInt:
			adder.AddWidget(&r.intCtl)
			secondary = &r.intCtl
			if r.field.hasMin {
				r.intCtl.SetMinimumValueInt64(r.field.min)
			}
			if r.field.hasMax {
				r.intCtl.SetMaximumValueInt64(r.field.max)
			}
			r.intCtl.SetValueInt64(r.fv.Int())
			fv := r.fv
			r.intCtl.OnValueChangedInt64(func(context *guigui.Context, v int64, committed bool) {
				if committed {
					fv.SetInt(v)
				}
			})
		case kindEnum:
			adder.AddWidget(&r.enumCtl)
			secondary = &r.enumCtl
			items := make([]basicwidget.SelectItem[string], len(r.field.options))
			for i, opt := range r.field.options {
				items[i] = basicwidget.SelectItem[string]{Text: opt.Label, Value: opt.Value}
			}
			r.enumCtl.SetItems(items)
			r.enumCtl.SelectItemByValue(r.fv.String())
			fv := r.fv
			options := r.field.options
			r.enumCtl.OnItemSelected(func(context *guigui.Context, index int) {
				if index >= 0 && index < len(options) {
					fv.SetString(options[index].Value)
				}
			})
		}

		g.formItems = append(g.formItems, basicwidget.FormItem{
			PrimaryWidget:   &r.label,
			SecondaryWidget: secondary,
		})
	}
	if g.hasRows {
		adder.AddWidget(&g.form)
		g.form.SetItems(g.formItems)
	}
	return nil
}

func (g *groupWidget) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	if !g.hasRows {
		return
	}
	if g.custom != nil {
		layouter.LayoutWidget(g.custom, widgetBounds.Bounds())
		return
	}
	layouter.LayoutWidget(&g.form, widgetBounds.Bounds())
}

func (g *groupWidget) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	if !g.hasRows {
		return image.Point{}
	}
	if g.custom != nil {
		return g.custom.Measure(context, constraints)
	}
	return g.form.Measure(context, constraints)
}
