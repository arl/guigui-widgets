// Package docked arranges widgets in dockable panels.
//
// [Group], [LockedGroup], and [Split] build a tree of tabbed panels.
// [NewRoot] shows that tree and lets the user drag panels to redock them.
// [Root] can save and restore the arrangement as JSON. [PaneView] is a
// separate vertical stack of collapsible panes.
package docked
