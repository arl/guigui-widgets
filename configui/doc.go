// Package configui builds a settings dialog from a Go struct.
//
// [Editor] lists the exported fields and edits them in place:
//   - bool -> checkbox
//   - string -> text field
//   - integer -> number field
//   - type implementing [Enum] -> dropdown
//   - struct -> nested settings section
//   - type implementing [CustomGroup] -> custom widget
package configui
