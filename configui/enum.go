package configui

// EnumOption holds a single choice in an [Enum] dropdown.
type EnumOption struct {
	Value string // Value is stored on the struct field.
	Label string // Label is shown in the dropdown.
}

// Enum is a string field shown as a dropdown.
type Enum interface {
	// EnumOptions returns the choices for the enumeration.
	EnumOptions() []EnumOption
}
