package configui

import "github.com/guigui-gui/guigui"

type CustomWidget interface {
	guigui.Widget
	Bind(v any)
}

type CustomGroup interface {
	NewConfigWidget() CustomWidget
}

type PadConfig interface {
	Normalize()
	Attached(port int) bool
	SetAttached(port int, v bool)
	Preset(port int) int
	SetPreset(port int, n int)
	BindingLabel(preset int, button string) string
	SetBinding(preset int, button, key string, gamepad int, padButton string)
}
