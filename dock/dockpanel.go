// Package dock provides a dockable tab and split layout widget.
package dock

import "github.com/guigui-gui/guigui"

// Panel is a unit of dockable content. Title labels the panel's tab, and
// Content is the widget shown while the panel is the active tab of its group.
type Panel struct {
	Title   string
	Content guigui.Widget
}
