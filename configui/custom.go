package configui

import "github.com/guigui-gui/guigui"

type CustomWidget interface {
	guigui.Widget
	Bind(v any)
}

type CustomGroup interface {
	NewConfigWidget() CustomWidget
}
