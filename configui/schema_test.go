package configui

import (
	"reflect"
	"testing"

	"github.com/guigui-gui/guigui"
)

type testEnum string

const (
	testEnumA testEnum = "a"
	testEnumB testEnum = "b"
)

func (testEnum) EnumOptions() []EnumOption {
	return []EnumOption{
		{Value: string(testEnumA), Label: "Option A"},
		{Value: string(testEnumB), Label: "Option B"},
	}
}

type testSubGroup struct {
	Enabled bool `configui:"label=Enabled Override"`
}

type testConfig struct {
	Name       string
	Verbose    bool
	Retries    int `configui:"min=1,max=10"`
	Mode       testEnum
	unexported string
	Ratio      float64 // unsupported kind: must not appear in the schema
	Sub        testSubGroup
}

func TestReflectSchema_FieldsInDeclarationOrder(t *testing.T) {
	schema := buildSchema[testConfig]()

	wantNames := []string{"Name", "Verbose", "Retries", "Mode", "Sub"}
	var gotNames []string
	for _, f := range schema.fields {
		gotNames = append(gotNames, f.name)
	}
	if !reflect.DeepEqual(gotNames, wantNames) {
		t.Fatalf("field names = %v, want %v (unexported and unsupported-kind fields must be skipped, order preserved)", gotNames, wantNames)
	}
}

func TestReflectSchema_Kinds(t *testing.T) {
	schema := buildSchema[testConfig]()
	byName := fieldsByName(schema)

	tests := []struct {
		name string
		kind kind
	}{
		{"Name", kindString},
		{"Verbose", kindBool},
		{"Retries", kindInt},
		{"Mode", kindEnum},
		{"Sub", kindGroup},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			f, ok := byName[tt.name]
			if !ok {
				t.Fatalf("field %q not found in schema", tt.name)
			}
			if f.Kind != tt.kind {
				t.Errorf("Kind = %v, want %v", f.Kind, tt.kind)
			}
		})
	}
}

func TestReflectSchema_DefaultLabelsAreHumanized(t *testing.T) {
	schema := buildSchema[testConfig]()
	byName := fieldsByName(schema)

	if got, want := byName["Retries"].label, "Retries"; got != want {
		t.Errorf("Retries label = %q, want %q", got, want)
	}
}

func TestReflectSchema_LabelTagOverridesDefault(t *testing.T) {
	schema := buildSchema[testConfig]()
	sub := fieldsByName(schema)["Sub"]
	if sub.group == nil {
		t.Fatalf("Sub field has no nested Group schema")
	}
	enabled := fieldsByName(sub.group)["Enabled"]
	if got, want := enabled.label, "Enabled Override"; got != want {
		t.Errorf("Enabled label = %q, want %q", got, want)
	}
}

func TestReflectSchema_IntMinMaxTag(t *testing.T) {
	schema := buildSchema[testConfig]()
	retries := fieldsByName(schema)["Retries"]

	if !retries.hasMin || retries.min != 1 {
		t.Errorf("Retries Min/HasMin = %d/%v, want 1/true", retries.min, retries.hasMin)
	}
	if !retries.hasMax || retries.max != 10 {
		t.Errorf("Retries Max/HasMax = %d/%v, want 10/true", retries.max, retries.hasMax)
	}
}

func TestReflectSchema_IntWithoutMinMaxTag(t *testing.T) {
	type cfg struct {
		Count int
	}
	f := fieldsByName(buildSchema[cfg]())["Count"]
	if f.hasMin || f.hasMax {
		t.Errorf("Count HasMin/HasMax = %v/%v, want false/false", f.hasMin, f.hasMax)
	}
}

func TestReflectSchema_EnumOptions(t *testing.T) {
	schema := buildSchema[testConfig]()
	mode := fieldsByName(schema)["Mode"]

	want := []EnumOption{
		{Value: "a", Label: "Option A"},
		{Value: "b", Label: "Option B"},
	}
	if !reflect.DeepEqual(mode.options, want) {
		t.Errorf("Mode.Options = %+v, want %+v", mode.options, want)
	}
}

func TestReflectSchema_NestedGroupFields(t *testing.T) {
	schema := buildSchema[testConfig]()
	sub := fieldsByName(schema)["Sub"]
	if sub.group == nil || len(sub.group.fields) != 1 {
		t.Fatalf("Sub.Group = %+v, want exactly one field", sub.group)
	}
	if sub.group.fields[0].Kind != kindBool {
		t.Errorf("Sub.Enabled kind = %v, want KindBool", sub.group.fields[0].Kind)
	}
}

func TestReflectSchema_PanicsOnNonStruct(t *testing.T) {
	defer func() {
		if recover() == nil {
			t.Fatal("expected ReflectSchema[int] to panic")
		}
	}()
	buildSchema[int]()
}

func TestHumanize(t *testing.T) {
	tests := []struct {
		name string
		want string
	}{
		{"ForceRegion", "Force Region"},
		{"RomsDir", "Roms Dir"},
		{"Name", "Name"},
		{"ID", "ID"},
		{"HTTPServer", "HTTP Server"},
		{"A", "A"},
		{"", ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := humanize(tt.name); got != tt.want {
				t.Errorf("humanize(%q) = %q, want %q", tt.name, got, tt.want)
			}
		})
	}
}

func TestParseTag(t *testing.T) {
	tests := []struct {
		name string
		tag  string
		want map[string]string
	}{
		{"empty", "", map[string]string{}},
		{"single key-value", "label=Force Region", map[string]string{"label": "Force Region"}},
		{"multiple", "min=1,max=10", map[string]string{"min": "1", "max": "10"}},
		{"flag without value", "readonly", map[string]string{"readonly": ""}},
		{"whitespace is trimmed", " label = Force Region , min = 1 ", map[string]string{"label": "Force Region", "min": "1"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := parseTag(tt.tag)
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("parseTag(%q) = %v, want %v", tt.tag, got, tt.want)
			}
		})
	}
}

func TestGroupTree_RootLeavesBecomeGeneral(t *testing.T) {
	nodes := groupTree(buildSchema[testConfig]())
	if len(nodes) != 2 {
		t.Fatalf("len(nodes) = %d, want 2 (General + Sub)", len(nodes))
	}
	if nodes[0].label != "General" || nodes[0].indent != 0 || nodes[0].path != nil {
		t.Errorf("nodes[0] = %+v, want General at indent 0 with nil path", nodes[0])
	}
	if nodes[1].label != "Sub" || nodes[1].indent != 1 || !reflect.DeepEqual(nodes[1].path, []int{6}) {
		t.Errorf("nodes[1] = %+v, want Sub at indent 1 with path [6] (field index of Sub)", nodes[1])
	}
}

func TestGroupTree_GroupsOnlyStartsAtTopLevel(t *testing.T) {
	type inner struct {
		Z bool
	}
	type cfg struct {
		A struct {
			X     int
			Inner inner
		}
		B struct {
			Y bool
		}
	}
	nodes := groupTree(buildSchema[cfg]())
	want := []struct {
		label  string
		indent int
		path   []int
	}{
		{label: "A", indent: 0, path: []int{0}},
		{label: "Inner", indent: 1, path: []int{0, 1}},
		{label: "B", indent: 0, path: []int{1}},
	}
	if len(nodes) != len(want) {
		t.Fatalf("len(nodes) = %d, want %d: %+v", len(nodes), len(want), nodes)
	}
	for i, n := range nodes {
		if n.label != want[i].label || n.indent != want[i].indent || !reflect.DeepEqual(n.path, want[i].path) {
			t.Errorf("nodes[%d] = {label:%q indent:%d path:%v}, want %+v", i, n.label, n.indent, n.path, want[i])
		}
		if n.id != i {
			t.Errorf("nodes[%d].id = %d, want %d", i, n.id, i)
		}
	}
}

func TestFirstLeafNode(t *testing.T) {
	type empty struct{}
	type cfg struct {
		Group empty
		Leaf  struct{ N int }
	}
	nodes := groupTree(buildSchema[cfg]())
	got := firstLeafNode(nodes)
	if got != 1 {
		t.Fatalf("firstLeafNode = %d (%q), want 1 (Leaf)", got, nodes[got].label)
	}
}

func TestValueAt(t *testing.T) {
	type inner struct{ N int }
	type cfg struct {
		A inner
	}
	v := cfg{A: inner{N: 7}}
	got := valueAt(reflect.ValueOf(&v).Elem(), []int{0}).Interface().(inner)
	if got.N != 7 {
		t.Errorf("valueAt A = %+v, want N=7", got)
	}
	root := valueAt(reflect.ValueOf(&v).Elem(), nil).Interface().(cfg)
	if root.A.N != 7 {
		t.Errorf("valueAt root = %+v, want A.N=7", root)
	}
}

func fieldsByName(schema *schema) map[string]field {
	m := make(map[string]field, len(schema.fields))
	for _, f := range schema.fields {
		m[f.name] = f
	}
	return m
}

type customGroup struct {
	N int
}

func (customGroup) NewConfigWidget() CustomWidget {
	return &dummyCustom{}
}

type dummyCustom struct {
	guigui.DefaultWidget
}

func (*dummyCustom) Bind(any) {}

type customMap map[string]string

func (customMap) NewConfigWidget() CustomWidget { return &dummyCustom{} }

func TestReflectSchema_CustomGroupMap(t *testing.T) {
	type cfg struct {
		Keys customMap
	}
	f, ok := fieldsByName(buildSchema[cfg]())["Keys"]
	if !ok {
		t.Fatal("Keys field missing")
	}
	if f.Kind != kindCustom || f.newCustom == nil {
		t.Fatalf("Keys = %+v, want a custom group", f)
	}
}

func TestReflectSchema_CustomGroup(t *testing.T) {
	type cfg struct {
		Pad customGroup
		Sub struct {
			On bool
		}
	}
	schema := buildSchema[cfg]()
	pad := fieldsByName(schema)["Pad"]
	if pad.Kind != kindCustom {
		t.Fatalf("Pad kind = %v, want kindCustom", pad.Kind)
	}
	if pad.newCustom == nil {
		t.Fatal("Pad missing custom factory")
	}
	nodes := groupTree(schema)
	if len(nodes) != 2 {
		t.Fatalf("len(nodes) = %d, want 2 (Pad + Sub)", len(nodes))
	}
	if nodes[0].label != "Pad" || nodes[1].label != "Sub" {
		t.Fatalf("nodes = %+v", nodes)
	}
	if nodes[0].schema.fields[0].Kind != kindCustom {
		t.Fatalf("Pad node schema kind = %v", nodes[0].schema.fields[0].Kind)
	}
}
