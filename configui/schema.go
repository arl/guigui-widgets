package configui

import (
	"reflect"
	"strconv"
	"strings"
	"unicode"
)

func buildSchema[T any]() *schema {
	return newSchema(reflect.TypeFor[T]())
}

func newSchema(t reflect.Type) *schema {
	if t.Kind() != reflect.Struct {
		panic("configui: " + t.String() + " is not a struct")
	}

	schema := &schema{}
	for i := range t.NumField() {
		sf := t.Field(i)
		if !sf.IsExported() {
			continue
		}

		tag := parseTag(sf.Tag.Get("configui"))
		field := field{
			name:  sf.Name,
			index: i,
			label: label(sf.Name, tag),
		}

		if factory, ok := customGroupFactory(sf.Type); ok {
			field.Kind = kindCustom
			field.newCustom = factory
			schema.fields = append(schema.fields, field)
			continue
		}

		switch {
		case sf.Type.Implements(enumType):
			field.Kind = kindEnum
			field.options = reflect.Zero(sf.Type).Interface().(Enum).EnumOptions()
		case sf.Type.Kind() == reflect.Bool:
			field.Kind = kindBool
		case sf.Type.Kind() == reflect.String:
			field.Kind = kindString
		case isIntKind(sf.Type.Kind()):
			field.Kind = kindInt
			field.min, field.hasMin = tagInt(tag, "min")
			field.max, field.hasMax = tagInt(tag, "max")
		case sf.Type.Kind() == reflect.Struct:
			field.Kind = kindGroup
			field.group = newSchema(sf.Type)
		default:
			// Unsupported field kind (slice, map, float, pointer, ...): not
			// rendered, unless the type implements CustomGroup (handled above).
			continue
		}

		schema.fields = append(schema.fields, field)
	}
	return schema
}

type kind int

const (
	kindBool kind = iota
	kindString
	kindInt
	kindEnum
	kindGroup
	kindCustom
)

type field struct {
	name  string // Go struct field name.
	index int    // field's index within the struct.
	label string // human-readable label presented to the user.

	Kind kind

	// Only applies to kindInt.
	min, max       int64
	hasMin, hasMax bool

	// Only applies to kindEnum.
	options []EnumOption

	// Only applies to kindGroup.
	group *schema

	// Only applies to kindCustom.
	newCustom func() CustomWidget
}

type schema struct{ fields []field }

// treeNode is one row in the settings navigation tree. path is the sequence of
// struct field indexes from the root value to this group's value; a nil path
// is the root struct itself.
type treeNode struct {
	id     int
	label  string
	indent int
	schema *schema
	path   []int
}

func (s *schema) hasLeaves() bool {
	for _, f := range s.fields {
		if f.Kind != kindGroup && f.Kind != kindCustom {
			return true
		}
	}
	return false
}

// groupTree flattens nested KindGroup fields into a depth-first list of
// navigation nodes. If the root schema has leaf settings of its own, they
// live under a synthetic "General" node; otherwise the tree starts at the
// top-level groups.
func groupTree(schema *schema) []treeNode {
	var nodes []treeNode
	if schema.hasLeaves() {
		nodes = append(nodes, treeNode{label: "General", schema: schema})
		appendGroups(&nodes, schema, nil, 1)
	} else {
		appendGroups(&nodes, schema, nil, 0)
	}
	for i := range nodes {
		nodes[i].id = i
	}
	return nodes
}

func appendGroups(nodes *[]treeNode, sch *schema, path []int, indent int) {
	for _, f := range sch.fields {
		if f.Kind == kindCustom {
			p := append(append([]int(nil), path...), f.index)
			leaf := &schema{fields: []field{f}}
			*nodes = append(*nodes, treeNode{
				label:  f.label,
				indent: indent,
				schema: leaf,
				path:   p,
			})
			continue
		}
		if f.Kind != kindGroup {
			continue
		}
		p := append(append([]int(nil), path...), f.index)
		*nodes = append(*nodes, treeNode{
			label:  f.label,
			indent: indent,
			schema: f.group,
			path:   p,
		})
		appendGroups(nodes, f.group, p, indent+1)
	}
}

func firstLeafNode(nodes []treeNode) int {
	for i, n := range nodes {
		if n.schema.hasLeaves() {
			return i
		}
	}
	return 0
}

func valueAt(root reflect.Value, path []int) reflect.Value {
	v := root
	for _, i := range path {
		v = v.Field(i)
	}
	return v
}

var enumType = reflect.TypeFor[Enum]()

func customGroupFactory(t reflect.Type) (func() CustomWidget, bool) {
	try := func(v any) (func() CustomWidget, bool) {
		cg, ok := v.(CustomGroup)
		if !ok {
			return nil, false
		}
		return cg.NewConfigWidget, true
	}
	if f, ok := try(reflect.Zero(t).Interface()); ok {
		return f, true
	}
	if t.Kind() != reflect.Pointer {
		if f, ok := try(reflect.New(t).Interface()); ok {
			return f, true
		}
	}
	return nil, false
}

func isIntKind(k reflect.Kind) bool {
	switch k {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return true
	default:
		return false
	}
}

// parseTag parses a `cfg:"key=value,key=value"` tag into a map.
// Keys without "=value" are stored with an empty value.
func parseTag(tag string) map[string]string {
	out := map[string]string{}
	for part := range strings.SplitSeq(tag, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		k, v, _ := strings.Cut(part, "=")
		out[strings.TrimSpace(k)] = strings.TrimSpace(v)
	}
	return out
}

func label(fieldName string, tag map[string]string) string {
	if l := tag["label"]; l != "" {
		return l
	}
	return humanize(fieldName)
}

func tagInt(tag map[string]string, key string) (int64, bool) {
	v, ok := tag[key]
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, false
	}
	return n, true
}

func humanize(name string) string {
	runes := []rune(name)
	var b strings.Builder
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			prev := runes[i-1]
			startOfWord := !unicode.IsUpper(prev)
			endOfAcronym := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if startOfWord || endOfAcronym {
				b.WriteRune(' ')
			}
		}
		b.WriteRune(r)
	}
	return b.String()
}
