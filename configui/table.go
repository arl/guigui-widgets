package configui

import (
	"image"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
)

type mapTable struct {
	guigui.DefaultWidget

	table        basicwidget.Table[string]
	rows         []mapTableRow
	onSelect     func(id string)
	buttonHeader string
	inputHeader  string
}

type mapTableRow struct {
	ID, Label, Value string
}

func (t *mapTable) setRows(rows []mapTableRow) {
	t.rows = append(t.rows[:0], rows...)
}

func (t *mapTable) setHeaders(buttonHeader, inputHeader string) {
	t.buttonHeader = buttonHeader
	t.inputHeader = inputHeader
}

func (t *mapTable) onRowSelected(f func(id string)) {
	t.onSelect = f
}

// ClearSelection drops the highlighted row. The table reports a click only
// when the selection changes, so a row left selected cannot be chosen again.
func (t *mapTable) clearSelection() {
	t.table.SelectItemByIndex(-1)
}

func (t *mapTable) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&t.table)
	u := basicwidget.UnitSize(context)
	buttonHeader := t.buttonHeader
	if buttonHeader == "" {
		buttonHeader = "Button"
	}
	inputHeader := t.inputHeader
	if inputHeader == "" {
		inputHeader = "Input"
	}
	t.table.SetColumns([]basicwidget.TableColumn{
		{HeaderText: buttonHeader, Width: guigui.FlexibleSize(1), MinWidth: 4 * u},
		{HeaderText: inputHeader, Width: guigui.FlexibleSize(2), MinWidth: 6 * u},
	})
	t.table.SetItemHeight(u)
	t.table.SetUnfocusedSelectionVisible(true)
	items := make([]basicwidget.TableRow[string], 0, len(t.rows))
	for _, row := range t.rows {
		items = append(items, basicwidget.TableRow[string]{
			Value: row.ID,
			Cells: []basicwidget.TableCell{
				{Text: row.Label},
				{Text: row.Value},
			},
		})
	}
	t.table.SetItems(items)
	if t.onSelect != nil {
		t.table.OnItemSelected(func(context *guigui.Context, index int) {
			item, ok := t.table.ItemByIndex(index)
			if !ok {
				return
			}
			t.onSelect(item.Value)
		})
	}
	return nil
}

func (t *mapTable) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&t.table, widgetBounds.Bounds())
}

func (t *mapTable) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	u := basicwidget.UnitSize(context)
	// Table.Measure is a fixed short height and forces an inner scrollbar.
	// Size to the real list content: header + rows + list corner padding.
	h := u*(1+len(t.rows)) + 2*basicwidget.RoundedCornerRadius(context)
	w := 12 * u
	if fw, ok := constraints.FixedWidth(); ok {
		w = fw
	}
	return image.Pt(w, h)
}
