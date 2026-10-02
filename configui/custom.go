package configui

import "github.com/guigui-gui/guigui"

// CustomWidget edits one configuration field.
//
// Bind receives a pointer to that field on each build. A field of type T is
// passed as *T.
type CustomWidget interface {
	guigui.Widget
	Bind(v any)
}

// CustomGroup replaces the generated controls for a field with a custom widget.
//
// Implement it on the field's type. NewConfigWidget returns the widget shown
// for that field; configui keeps the widget and calls [CustomWidget.Bind] with
// a pointer to the value. Use this when the field needs a layout the generated
// form does not provide.
type CustomGroup interface {
	NewConfigWidget() CustomWidget
}
