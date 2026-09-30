package configui

type EnumOption struct {
	Value string // Value is the underlying value stored on the struct field.
	Label string // Label is the string presented in the UI.
}

type Enum interface {
	EnumOptions() []EnumOption
}
