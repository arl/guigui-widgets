package datepicker

import (
	"image"
	"slices"
	"strconv"
	"time"

	"github.com/guigui-gui/guigui"
	"github.com/guigui-gui/guigui/basicwidget"
	"github.com/guigui-gui/guigui/basicwidget/basicwidgetdraw"
	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/inpututil"
)

var (
	datePickerEventValueChanged = guigui.GenerateEventKey()
	datePickerDayEventActivated = guigui.GenerateEventKey()
)

type datePickerKind int

const (
	datePickerKindDocked datePickerKind = iota
	datePickerKindModal
	datePickerKindModalInput
)

type datePickerMode int

const (
	datePickerModeCalendar datePickerMode = iota
	datePickerModeInput
)

func datePickerDefaultMode(kind datePickerKind) datePickerMode {
	if kind == datePickerKindModalInput {
		return datePickerModeInput
	}
	return datePickerModeCalendar
}

func datePickerPanelWidth(context *guigui.Context) int {
	return 14 * basicwidget.UnitSize(context)
}

func datePickerDaySize(context *guigui.Context) int {
	u := basicwidget.UnitSize(context)
	inner := datePickerPanelWidth(context) - u
	return inner / 7
}

type datePickerDay struct {
	guigui.DefaultWidget

	label basicwidget.Text

	date     Date
	inMonth  bool
	selected bool
	today    bool
	enabled  bool

	pressed     bool
	prevHovered bool
}

func (d *datePickerDay) OnActivated(f func(context *guigui.Context)) {
	guigui.SetEventHandler(d, datePickerDayEventActivated, f)
}

func (d *datePickerDay) configure(date Date, inMonth, selected, today, enabled bool) {
	d.date = date
	d.inMonth = inMonth
	d.selected = selected
	d.today = today
	d.enabled = enabled
}

func (d *datePickerDay) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&d.label)

	d.label.SetValue(strconv.Itoa(d.date.Day))
	d.label.SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
	d.label.SetVerticalAlign(basicwidget.VerticalAlignMiddle)
	context.SetPassthrough(&d.label, true)
	context.SetEnabled(d, d.enabled)

	cm := context.ColorMode()
	var style basicwidget.TextStyle
	switch {
	case !d.enabled:
		style.SetColor(basicwidgetdraw.TextColor(cm, false))
	case d.selected:
		style.SetColor(textOnAccentColor(cm))
	case d.today:
		style.SetColor(accentColor(cm))
	case !d.inMonth:
		style.SetColor(basicwidgetdraw.TextColor(cm, false))
	default:
		style.SetColor(basicwidgetdraw.TextColor(cm, true))
	}
	d.label.SetBaseStyle(&style)
	return nil
}

func (d *datePickerDay) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&d.label, widgetBounds.Bounds())
}

func (d *datePickerDay) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	s := datePickerDaySize(context)
	return image.Pt(s, s)
}

func (d *datePickerDay) HandlePointingInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if !d.enabled || !widgetBounds.IsHitAtCursor() {
		if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
			d.pressed = false
		}
		return guigui.HandleInputResult{}
	}
	if inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		d.pressed = true
		guigui.DispatchEvent(d, datePickerDayEventActivated)
		return guigui.HandleInputByWidget(d)
	}
	if !ebiten.IsMouseButtonPressed(ebiten.MouseButtonLeft) {
		d.pressed = false
	}
	return guigui.HandleInputResult{}
}

func (d *datePickerDay) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	hovered := widgetBounds.IsHitAtCursor()
	if hovered != d.prevHovered {
		d.prevHovered = hovered
		guigui.RequestRedraw(d)
	}
	return nil
}

func (d *datePickerDay) CursorShape(context *guigui.Context, widgetBounds *guigui.WidgetBounds) (ebiten.CursorShapeType, bool) {
	if d.enabled && widgetBounds.IsHitAtCursor() {
		return ebiten.CursorShapePointer, true
	}
	return 0, true
}

func (d *datePickerDay) Draw(context *guigui.Context, widgetBounds *guigui.WidgetBounds, dst *ebiten.Image) {
	bounds := widgetBounds.Bounds()
	side := min(bounds.Dx(), bounds.Dy())
	inset := int(2 * context.Scale())
	side = max(side-2*inset, 0)
	sq := image.Rectangle{
		Min: image.Pt(
			bounds.Min.X+(bounds.Dx()-side)/2,
			bounds.Min.Y+(bounds.Dy()-side)/2,
		),
	}
	sq.Max = sq.Min.Add(image.Pt(side, side))
	if sq.Empty() {
		return
	}

	cm := context.ColorMode()
	radius := side / 2
	hovered := widgetBounds.IsHitAtCursor()
	switch {
	case d.selected:
		basicwidgetdraw.DrawRoundedRect(context, dst, sq, accentColor(cm), radius)
	case d.enabled && (hovered || d.pressed):
		basicwidgetdraw.DrawRoundedRect(context, dst, sq, itemHoveredBackgroundColor(cm), radius)
	}
	if d.today && !d.selected {
		clr1, clr2 := basicwidgetdraw.BorderAccentColors(cm, basicwidgetdraw.RoundedRectBorderTypeRegular)
		width := float32(1 * context.Scale())
		basicwidgetdraw.DrawRoundedRectBorder(context, dst, sq, clr1, clr2, radius, width, basicwidgetdraw.RoundedRectBorderTypeRegular)
	}
}

type datePickerContent struct {
	guigui.DefaultWidget

	supportingText basicwidget.Text
	headlineText   basicwidget.Text
	toggleButton   basicwidget.Button
	headerDivider  basicwidget.Divider
	monthSelect    basicwidget.Select[time.Month]
	yearSelect     basicwidget.Select[int]
	prevButton     basicwidget.Button
	nextButton     basicwidget.Button
	weekdays       [7]basicwidget.Text
	days           [42]datePickerDay
	input          basicwidget.TextInput
	footerDivider  basicwidget.Divider
	cancelButton   basicwidget.Button
	okButton       basicwidget.Button

	kind         datePickerKind
	mode         datePickerMode
	firstWeekday time.Weekday
	draft        Date
	viewedYear   int
	viewedMonth  time.Month
	minDate      Date
	maxDate      Date
	title        string

	monthItems []basicwidget.SelectItem[time.Month]
	yearItems  []basicwidget.SelectItem[int]

	onDayActivated      [42]func(context *guigui.Context)
	onMonthSelected     func(context *guigui.Context, index int)
	onYearSelected      func(context *guigui.Context, index int)
	onPrev              func(context *guigui.Context)
	onNext              func(context *guigui.Context)
	onToggle            func(context *guigui.Context)
	onCancel            func(context *guigui.Context)
	onOK                func(context *guigui.Context)
	onInputValueChanged func(context *guigui.Context, text string, committed bool)

	onConfirmed func(context *guigui.Context, date Date)
	onCancelled func(context *guigui.Context)

	headerItems    []guigui.LinearLayoutItem
	headerLayout   guigui.LinearLayout
	navItems       []guigui.LinearLayoutItem
	navLayout      guigui.LinearLayout
	headlineItems  []guigui.LinearLayoutItem
	headlineLayout guigui.LinearLayout
	weekdayItems   []guigui.LinearLayoutItem
	weekdayLayout  guigui.LinearLayout
	weekItems      [6][]guigui.LinearLayoutItem
	weekLayouts    [6]guigui.LinearLayout
	actionItems    []guigui.LinearLayoutItem
	actionLayout   guigui.LinearLayout
	layoutItems    []guigui.LinearLayoutItem
}

func (c *datePickerContent) setKind(kind datePickerKind) {
	c.kind = kind
}

func (c *datePickerContent) setTitle(title string) {
	c.title = title
}

func (c *datePickerContent) setFirstWeekday(day time.Weekday) {
	c.firstWeekday = day
}

func (c *datePickerContent) setMinDate(d Date) {
	c.minDate = d
}

func (c *datePickerContent) setMaxDate(d Date) {
	c.maxDate = d
}

func (c *datePickerContent) setOnConfirmed(f func(context *guigui.Context, date Date)) {
	c.onConfirmed = f
}

func (c *datePickerContent) setOnCancelled(f func(context *guigui.Context)) {
	c.onCancelled = f
}

func (c *datePickerContent) beginSession(value Date) {
	c.mode = datePickerDefaultMode(c.kind)
	if value.IsZero() {
		c.draft = Today()
	} else {
		c.draft = value
	}
	c.draft = clampDate(c.draft)
	c.viewedYear = c.draft.Year
	c.viewedMonth = c.draft.Month
}

func (c *datePickerContent) setDraft(d Date) {
	if d.IsZero() || !d.Valid() {
		return
	}
	d = clampDate(d)
	if !c.minDate.IsZero() && d.Compare(c.minDate) < 0 {
		return
	}
	if !c.maxDate.IsZero() && d.Compare(c.maxDate) > 0 {
		return
	}
	c.draft = d
	c.viewedYear = d.Year
	c.viewedMonth = d.Month
}

func (c *datePickerContent) inRange(d Date) bool {
	if !c.minDate.IsZero() && d.Compare(c.minDate) < 0 {
		return false
	}
	if !c.maxDate.IsZero() && d.Compare(c.maxDate) > 0 {
		return false
	}
	return true
}

func (c *datePickerContent) shiftViewedMonth(delta int) {
	t := time.Date(c.viewedYear, c.viewedMonth, 1, 0, 0, 0, 0, time.Local).AddDate(0, delta, 0)
	y, m, _ := t.Date()
	if y < datePickerMinYear || y > datePickerMaxYear {
		return
	}
	c.viewedYear = y
	c.viewedMonth = m
}

func (c *datePickerContent) canShiftViewedMonth(delta int) bool {
	t := time.Date(c.viewedYear, c.viewedMonth, 1, 0, 0, 0, 0, time.Local).AddDate(0, delta, 0)
	y := t.Year()
	return y >= datePickerMinYear && y <= datePickerMaxYear
}

func (c *datePickerContent) showsHeadline() bool {
	return c.kind != datePickerKindDocked
}

func (c *datePickerContent) showsToggle() bool {
	return c.kind != datePickerKindDocked
}

func (c *datePickerContent) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	if c.viewedYear == 0 {
		c.beginSession(c.draft)
	}

	showCalendar := c.mode == datePickerModeCalendar

	if c.showsHeadline() {
		adder.AddWidget(&c.supportingText)
		adder.AddWidget(&c.headlineText)
		adder.AddWidget(&c.headerDivider)
	}
	if c.showsToggle() {
		adder.AddWidget(&c.toggleButton)
	}
	if showCalendar {
		adder.AddWidget(&c.monthSelect)
		adder.AddWidget(&c.yearSelect)
		adder.AddWidget(&c.prevButton)
		adder.AddWidget(&c.nextButton)
		for i := range c.weekdays {
			adder.AddWidget(&c.weekdays[i])
		}
		for i := range c.days {
			adder.AddWidget(&c.days[i])
		}
	} else {
		adder.AddWidget(&c.input)
	}
	adder.AddWidget(&c.footerDivider)
	adder.AddWidget(&c.cancelButton)
	adder.AddWidget(&c.okButton)

	title := c.title
	if title == "" {
		title = "Select date"
	}
	c.supportingText.SetValue(title)
	var supportingStyle basicwidget.TextStyle
	supportingStyle.SetColor(basicwidgetdraw.TextColor(context.ColorMode(), false))
	c.supportingText.SetBaseStyle(&supportingStyle)

	c.headlineText.SetValue(formatDateHeadline(c.draft))
	var headlineStyle basicwidget.TextStyle
	headlineStyle.SetBold(true)
	c.headlineText.SetBaseStyle(&headlineStyle)
	c.headlineText.SetScale(1.75)
	c.headlineText.SetVerticalAlign(basicwidget.VerticalAlignMiddle)

	if c.showsToggle() {
		iconName := "edit"
		if c.mode == datePickerModeInput {
			iconName = "calendar_month"
		}
		img, err := theResourceImages.Get(iconName, context.ColorMode())
		if err != nil {
			return err
		}
		c.toggleButton.SetIcon(img)
		if c.onToggle == nil {
			c.onToggle = func(context *guigui.Context) {
				if c.mode == datePickerModeCalendar {
					c.mode = datePickerModeInput
				} else {
					c.mode = datePickerModeCalendar
				}
			}
		}
		c.toggleButton.OnDown(c.onToggle)
	}

	c.monthItems = adjustSliceSize(c.monthItems, 12)
	for i := time.January; i <= time.December; i++ {
		c.monthItems[i-1] = basicwidget.SelectItem[time.Month]{
			Text:  i.String(),
			Value: i,
		}
	}
	c.monthSelect.SetItems(c.monthItems)
	if c.onMonthSelected == nil {
		c.onMonthSelected = func(context *guigui.Context, index int) {
			item, ok := c.monthSelect.ItemByIndex(index)
			if !ok {
				return
			}
			c.viewedMonth = item.Value
		}
	}
	c.monthSelect.OnItemSelected(c.onMonthSelected)
	c.monthSelect.SelectItemByValue(c.viewedMonth)

	yearCount := datePickerMaxYear - datePickerMinYear + 1
	c.yearItems = adjustSliceSize(c.yearItems, yearCount)
	for i := range yearCount {
		y := datePickerMinYear + i
		c.yearItems[i] = basicwidget.SelectItem[int]{
			Text:  datePickerYearString(y),
			Value: y,
		}
	}
	c.yearSelect.SetItems(c.yearItems)
	if c.onYearSelected == nil {
		c.onYearSelected = func(context *guigui.Context, index int) {
			item, ok := c.yearSelect.ItemByIndex(index)
			if !ok {
				return
			}
			c.viewedYear = item.Value
		}
	}
	c.yearSelect.OnItemSelected(c.onYearSelected)
	c.yearSelect.SelectItemByValue(c.viewedYear)

	imgPrev, err := theResourceImages.Get("keyboard_arrow_left", context.ColorMode())
	if err != nil {
		return err
	}
	imgNext, err := theResourceImages.Get("keyboard_arrow_right", context.ColorMode())
	if err != nil {
		return err
	}
	c.prevButton.SetIcon(imgPrev)
	c.nextButton.SetIcon(imgNext)
	if c.onPrev == nil {
		c.onPrev = func(context *guigui.Context) {
			c.shiftViewedMonth(-1)
		}
	}
	if c.onNext == nil {
		c.onNext = func(context *guigui.Context) {
			c.shiftViewedMonth(1)
		}
	}
	c.prevButton.OnDown(c.onPrev)
	c.nextButton.OnDown(c.onNext)
	context.SetEnabled(&c.prevButton, c.canShiftViewedMonth(-1) && !c.monthSelect.IsPopupOpen() && !c.yearSelect.IsPopupOpen())
	context.SetEnabled(&c.nextButton, c.canShiftViewedMonth(1) && !c.monthSelect.IsPopupOpen() && !c.yearSelect.IsPopupOpen())
	context.SetEnabled(&c.monthSelect, !c.yearSelect.IsPopupOpen())
	context.SetEnabled(&c.yearSelect, !c.monthSelect.IsPopupOpen())

	today := Today()
	first := time.Date(c.viewedYear, c.viewedMonth, 1, 0, 0, 0, 0, time.Local)
	startOffset := (int(first.Weekday()) - int(c.firstWeekday) + 7) % 7
	start := DateFromTime(first.AddDate(0, 0, -startOffset))
	for i := range c.weekdays {
		day := time.Weekday((int(c.firstWeekday) + i) % 7)
		c.weekdays[i].SetValue(weekdayLetter(day))
		c.weekdays[i].SetHorizontalAlign(basicwidget.HorizontalAlignCenter)
		c.weekdays[i].SetVerticalAlign(basicwidget.VerticalAlignMiddle)
		var style basicwidget.TextStyle
		style.SetColor(basicwidgetdraw.TextColor(context.ColorMode(), false))
		c.weekdays[i].SetBaseStyle(&style)
	}
	for i := range c.days {
		date := start.addDays(i)
		inMonth := date.Month == c.viewedMonth && date.Year == c.viewedYear
		enabled := date.Valid() && c.inRange(date)
		c.days[i].configure(date, inMonth, date.Equal(c.draft), date.Equal(today), enabled)
		if c.onDayActivated[i] == nil {
			index := i
			c.onDayActivated[i] = func(context *guigui.Context) {
				c.setDraft(c.days[index].date)
			}
		}
		c.days[i].OnActivated(c.onDayActivated[i])
	}

	c.input.SetPlaceholder("DD/MM/YYYY")
	c.input.SetValue(formatDate(c.draft))
	if c.onInputValueChanged == nil {
		c.onInputValueChanged = func(context *guigui.Context, text string, committed bool) {
			d, ok := parseDate(text)
			c.input.SetError(!ok)
			if !ok {
				if committed {
					c.input.SetSupportText("Enter a date as DD/MM/YYYY")
				}
				return
			}
			c.input.SetSupportText("")
			if d.IsZero() {
				return
			}
			if committed || c.inRange(d) {
				c.setDraft(d)
			}
		}
	}
	c.input.OnValueChanged(c.onInputValueChanged)

	c.cancelButton.SetText("Cancel")
	c.okButton.SetText("OK")
	c.okButton.SetType(basicwidget.ButtonTypePrimary)
	if c.onCancel == nil {
		c.onCancel = func(context *guigui.Context) {
			if c.onCancelled != nil {
				c.onCancelled(context)
			}
		}
	}
	if c.onOK == nil {
		c.onOK = func(context *guigui.Context) {
			if c.mode == datePickerModeInput {
				c.input.CommitWithCurrentInputValue()
			}
			if c.onConfirmed != nil {
				c.onConfirmed(context, c.draft)
			}
		}
	}
	c.cancelButton.OnDown(c.onCancel)
	c.okButton.OnDown(c.onOK)
	context.SetEnabled(&c.okButton, c.draft.Valid() && c.inRange(c.draft))

	return nil
}

func (c *datePickerContent) layout(context *guigui.Context) guigui.LinearLayout {
	u := basicwidget.UnitSize(context)
	dayH := datePickerDaySize(context)
	showCalendar := c.mode == datePickerModeCalendar

	c.layoutItems = slices.Delete(c.layoutItems, 0, len(c.layoutItems))

	if c.showsHeadline() {
		c.headlineItems = slices.Delete(c.headlineItems, 0, len(c.headlineItems))
		c.headlineItems = append(
			c.headlineItems,
			guigui.LinearLayoutItem{Widget: &c.headlineText, Size: guigui.FlexibleSize(1)},
		)
		if c.showsToggle() {
			c.headlineItems = append(
				c.headlineItems,
				guigui.LinearLayoutItem{Widget: &c.toggleButton, Size: guigui.FixedSize(u)},
			)
		}
		c.headlineLayout = guigui.LinearLayout{
			Direction: guigui.LayoutDirectionHorizontal,
			Items:     c.headlineItems,
			Gap:       u / 4,
		}
		c.layoutItems = append(
			c.layoutItems,
			guigui.LinearLayoutItem{Widget: &c.supportingText},
			guigui.LinearLayoutItem{
				Size:   guigui.FixedSize(max(u+u/2, c.headlineText.Measure(context, guigui.Constraints{}).Y)),
				Layout: &c.headlineLayout,
			},
			guigui.LinearLayoutItem{Widget: &c.headerDivider},
		)
	}

	if showCalendar {
		c.navItems = slices.Delete(c.navItems, 0, len(c.navItems))
		c.navItems = append(
			c.navItems,
			guigui.LinearLayoutItem{Widget: &c.prevButton, Size: guigui.FixedSize(u)},
			guigui.LinearLayoutItem{Widget: &c.nextButton, Size: guigui.FixedSize(u)},
		)
		c.navLayout = guigui.LinearLayout{
			Direction: guigui.LayoutDirectionHorizontal,
			Items:     c.navItems,
			Gap:       u / 4,
		}
		c.headerItems = slices.Delete(c.headerItems, 0, len(c.headerItems))
		c.headerItems = append(
			c.headerItems,
			guigui.LinearLayoutItem{Widget: &c.monthSelect},
			guigui.LinearLayoutItem{Widget: &c.yearSelect},
			guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1)},
			guigui.LinearLayoutItem{Layout: &c.navLayout},
		)
		c.headerLayout = guigui.LinearLayout{
			Direction: guigui.LayoutDirectionHorizontal,
			Items:     c.headerItems,
			Gap:       u / 4,
		}

		c.weekdayItems = slices.Delete(c.weekdayItems, 0, len(c.weekdayItems))
		for i := range c.weekdays {
			c.weekdayItems = append(c.weekdayItems, guigui.LinearLayoutItem{
				Widget: &c.weekdays[i],
				Size:   guigui.FlexibleSize(1),
			})
		}
		c.weekdayLayout = guigui.LinearLayout{
			Direction: guigui.LayoutDirectionHorizontal,
			Items:     c.weekdayItems,
		}

		c.layoutItems = append(
			c.layoutItems,
			guigui.LinearLayoutItem{
				Size:   guigui.FixedSize(u),
				Layout: &c.headerLayout,
			},
			guigui.LinearLayoutItem{
				Size:   guigui.FixedSize(basicwidget.LineHeight(context)),
				Layout: &c.weekdayLayout,
			},
		)
		for week := range 6 {
			c.weekItems[week] = slices.Delete(c.weekItems[week], 0, len(c.weekItems[week]))
			for col := range 7 {
				c.weekItems[week] = append(c.weekItems[week], guigui.LinearLayoutItem{
					Widget: &c.days[week*7+col],
					Size:   guigui.FlexibleSize(1),
				})
			}
			c.weekLayouts[week] = guigui.LinearLayout{
				Direction: guigui.LayoutDirectionHorizontal,
				Items:     c.weekItems[week],
			}
			c.layoutItems = append(c.layoutItems, guigui.LinearLayoutItem{
				Size:   guigui.FixedSize(dayH),
				Layout: &c.weekLayouts[week],
			})
		}
	} else {
		c.layoutItems = append(
			c.layoutItems,
			guigui.LinearLayoutItem{Widget: &c.input},
		)
	}

	c.actionItems = slices.Delete(c.actionItems, 0, len(c.actionItems))
	c.actionItems = append(
		c.actionItems,
		guigui.LinearLayoutItem{Size: guigui.FlexibleSize(1)},
		guigui.LinearLayoutItem{Widget: &c.cancelButton},
		guigui.LinearLayoutItem{Widget: &c.okButton},
	)
	c.actionLayout = guigui.LinearLayout{
		Direction: guigui.LayoutDirectionHorizontal,
		Items:     c.actionItems,
		Gap:       u / 4,
	}
	c.layoutItems = append(
		c.layoutItems,
		guigui.LinearLayoutItem{Widget: &c.footerDivider},
		guigui.LinearLayoutItem{
			Size:   guigui.FixedSize(u),
			Layout: &c.actionLayout,
		},
	)

	return guigui.LinearLayout{
		Direction: guigui.LayoutDirectionVertical,
		Items:     c.layoutItems,
		Gap:       u / 4,
		Padding:   guigui.Padding{Start: u / 2, Top: u / 2, End: u / 2, Bottom: u / 2},
	}
}

func (c *datePickerContent) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	c.layout(context).LayoutWidgets(context, widgetBounds.Bounds(), layouter)
}

func (c *datePickerContent) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	s := c.layout(context).Measure(context, constraints)
	s.X = max(s.X, datePickerPanelWidth(context))
	return s
}

// Docked is a text field that opens a calendar dropdown, matching the
// Material 3 docked date picker: month and year menus, previous/next month,
// weekday headings, a six-week day grid, and Cancel/OK.
type Docked struct {
	guigui.DefaultWidget

	textInput basicwidget.TextInput
	popup     basicwidget.Popup
	content   datePickerContent

	value           Date
	firstWeekday    time.Weekday
	minDate         Date
	maxDate         Date
	placeholder     string
	seeding         bool
	prevTextFocused bool

	onTextChanged func(context *guigui.Context, text string, committed bool)
	onConfirmed   func(context *guigui.Context, date Date)
	onCancelled   func(context *guigui.Context)
	onClosed      func(context *guigui.Context, reason basicwidget.PopupCloseReason)
}

// Value returns the committed date. The zero value means no date is selected.
func (d *Docked) Value() Date {
	return d.value
}

// SetValue sets the committed date. The zero value clears the selection.
func (d *Docked) SetValue(date Date) {
	d.value = date
}

// SetPlaceholder sets the placeholder shown while no date is selected.
func (d *Docked) SetPlaceholder(placeholder string) {
	d.placeholder = placeholder
}

// SetFirstWeekday sets the first column of the calendar. The default is Sunday.
func (d *Docked) SetFirstWeekday(day time.Weekday) {
	d.firstWeekday = day
}

// SetMinDate sets the earliest selectable date. The zero value leaves it
// unbounded.
func (d *Docked) SetMinDate(date Date) {
	d.minDate = date
}

// SetMaxDate sets the latest selectable date. The zero value leaves it
// unbounded.
func (d *Docked) SetMaxDate(date Date) {
	d.maxDate = date
}

// OnValueChanged sets the handler invoked when the user confirms a date.
func (d *Docked) OnValueChanged(f func(context *guigui.Context, date Date)) {
	guigui.SetEventHandler(d, datePickerEventValueChanged, f)
}

func (d *Docked) commit(context *guigui.Context, date Date) {
	d.value = date
	d.seeding = true
	d.textInput.ForceSetValue(formatDate(date))
	d.seeding = false
	d.textInput.SetError(false)
	d.textInput.SetSupportText("")
	guigui.DispatchEvent(d, datePickerEventValueChanged, date)
}

func (d *Docked) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	adder.AddWidget(&d.textInput)
	adder.AddWidget(&d.popup)

	context.SetButtonInputReceptive(d, d.popup.IsOpen())
	context.DelegateFocus(d, &d.textInput)

	img, err := theResourceImages.Get("calendar_month", context.ColorMode())
	if err != nil {
		return err
	}
	d.textInput.SetIcon(img)
	placeholder := d.placeholder
	if placeholder == "" {
		placeholder = "DD/MM/YYYY"
	}
	d.textInput.SetPlaceholder(placeholder)
	if d.popup.IsOpen() {
		d.textInput.SetValue(formatDate(d.content.draft))
	} else {
		d.textInput.SetValue(formatDate(d.value))
	}

	if d.onTextChanged == nil {
		d.onTextChanged = func(context *guigui.Context, text string, committed bool) {
			if d.seeding {
				return
			}
			parsed, ok := parseDate(text)
			d.textInput.SetError(!ok)
			if !ok {
				if committed {
					d.textInput.SetSupportText("Enter a date as DD/MM/YYYY")
				}
				return
			}
			d.textInput.SetSupportText("")
			if !committed {
				if d.popup.IsOpen() && !parsed.IsZero() {
					d.content.setDraft(parsed)
				}
				return
			}
			d.value = parsed
			guigui.DispatchEvent(d, datePickerEventValueChanged, parsed)
		}
	}
	d.textInput.OnValueChanged(d.onTextChanged)

	d.content.setKind(datePickerKindDocked)
	d.content.setFirstWeekday(d.firstWeekday)
	d.content.setMinDate(d.minDate)
	d.content.setMaxDate(d.maxDate)
	if d.onConfirmed == nil {
		d.onConfirmed = func(context *guigui.Context, date Date) {
			d.commit(context, date)
			d.popup.SetOpen(false)
		}
	}
	if d.onCancelled == nil {
		d.onCancelled = func(context *guigui.Context) {
			d.popup.SetOpen(false)
		}
	}
	d.content.setOnConfirmed(d.onConfirmed)
	d.content.setOnCancelled(d.onCancelled)
	d.popup.SetContent(&d.content)
	d.popup.SetCloseByClickingOutside(true)
	d.popup.SetModal(false)
	d.popup.SetAnimated(true)
	if d.popup.IsOpen() {
		d.popup.BringToFrontLayer(context)
	}
	if d.onClosed == nil {
		d.onClosed = func(context *guigui.Context, reason basicwidget.PopupCloseReason) {
			d.textInput.SetValue(formatDate(d.value))
			d.textInput.SetError(false)
			d.textInput.SetSupportText("")
		}
	}
	d.popup.OnClose(d.onClosed)

	return nil
}

func (d *Docked) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	bounds := widgetBounds.Bounds()
	layouter.LayoutWidget(&d.textInput, bounds)

	popupSize := d.content.Measure(context, guigui.Constraints{})
	popupSize.X = max(popupSize.X, bounds.Dx())
	app := context.AppBounds()
	pos := image.Pt(bounds.Min.X, bounds.Max.Y)
	if pos.Y+popupSize.Y > app.Max.Y {
		pos.Y = bounds.Min.Y - popupSize.Y
	}
	if pos.X+popupSize.X > app.Max.X {
		pos.X = max(app.Max.X-popupSize.X, app.Min.X)
	}
	layouter.LayoutWidget(&d.popup, image.Rectangle{
		Min: pos,
		Max: pos.Add(popupSize),
	})
}

func (d *Docked) openPopup(context *guigui.Context) {
	if d.popup.IsOpen() {
		return
	}
	d.textInput.CommitWithCurrentInputValue()
	d.content.beginSession(d.value)
	if !d.value.IsZero() {
		d.content.setDraft(d.value)
	}
	d.popup.SetOpen(true)
	d.popup.BringToFrontLayer(context)
}

func (d *Docked) Tick(context *guigui.Context, widgetBounds *guigui.WidgetBounds) error {
	textFocused := context.IsFocusedOrHasFocusedDescendant(&d.textInput)
	if textFocused && !d.prevTextFocused {
		d.openPopup(context)
	}
	if !d.popup.IsOpen() && widgetBounds.IsHitAtCursor() && inpututil.IsMouseButtonJustPressed(ebiten.MouseButtonLeft) {
		d.openPopup(context)
	}
	d.prevTextFocused = textFocused
	return nil
}

func (d *Docked) HandleButtonInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if d.popup.IsOpen() && inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		d.popup.SetOpen(false)
		return guigui.HandleInputByWidget(d)
	}
	return guigui.HandleInputResult{}
}

func (d *Docked) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	h := d.textInput.Measure(context, constraints).Y
	w := datePickerPanelWidth(context)
	if fixedWidth, ok := constraints.FixedWidth(); ok {
		w = min(w, fixedWidth)
	}
	return image.Pt(w, h)
}

type modalDatePicker struct {
	button  basicwidget.Button
	popup   basicwidget.Popup
	content datePickerContent

	kind         datePickerKind
	value        Date
	firstWeekday time.Weekday
	minDate      Date
	maxDate      Date
	title        string
	buttonText   string

	onDown      func(context *guigui.Context)
	onConfirmed func(context *guigui.Context, date Date)
	onCancelled func(context *guigui.Context)
}

func (m *modalDatePicker) setOpen(open bool) {
	if open {
		m.content.beginSession(m.value)
		if !m.value.IsZero() {
			m.content.setDraft(m.value)
		}
	}
	m.popup.SetOpen(open)
}

func (m *modalDatePicker) build(self guigui.Widget, kind datePickerKind, context *guigui.Context, adder *guigui.ChildAdder) error {
	m.kind = kind
	adder.AddWidget(&m.button)
	adder.AddWidget(&m.popup)

	context.SetButtonInputReceptive(self, m.popup.IsOpen())

	img, err := theResourceImages.Get("calendar_month", context.ColorMode())
	if err != nil {
		return err
	}
	m.button.SetIcon(img)
	text := m.buttonText
	if text == "" {
		if m.kind == datePickerKindModalInput {
			text = "Enter date"
		} else {
			text = "Select date"
		}
	}
	if !m.value.IsZero() {
		text = formatDate(m.value)
	}
	m.button.SetText(text)

	if m.onDown == nil {
		m.onDown = func(context *guigui.Context) {
			m.setOpen(true)
			m.popup.BringToFrontLayer(context)
		}
	}
	m.button.OnDown(m.onDown)

	m.content.setKind(m.kind)
	m.content.setTitle(m.title)
	m.content.setFirstWeekday(m.firstWeekday)
	m.content.setMinDate(m.minDate)
	m.content.setMaxDate(m.maxDate)
	if m.onConfirmed == nil {
		m.onConfirmed = func(context *guigui.Context, date Date) {
			m.value = date
			m.popup.SetOpen(false)
			guigui.DispatchEvent(self, datePickerEventValueChanged, date)
		}
	}
	if m.onCancelled == nil {
		m.onCancelled = func(context *guigui.Context) {
			m.popup.SetOpen(false)
		}
	}
	m.content.setOnConfirmed(m.onConfirmed)
	m.content.setOnCancelled(m.onCancelled)
	m.popup.SetContent(&m.content)
	m.popup.SetCloseByClickingOutside(true)
	m.popup.SetModal(true)
	m.popup.SetBackgroundDark(true)
	m.popup.SetAnimated(true)
	if m.popup.IsOpen() {
		m.popup.BringToFrontLayer(context)
	}

	return nil
}

func (m *modalDatePicker) layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	layouter.LayoutWidget(&m.button, widgetBounds.Bounds())

	app := context.AppBounds()
	m.popup.SetBackgroundBounds(app)
	popupSize := m.content.Measure(context, guigui.Constraints{})
	pos := image.Pt(
		app.Min.X+(app.Dx()-popupSize.X)/2,
		app.Min.Y+(app.Dy()-popupSize.Y)/2,
	)
	layouter.LayoutWidget(&m.popup, image.Rectangle{
		Min: pos,
		Max: pos.Add(popupSize),
	})
}

func (m *modalDatePicker) handleButtonInput(self guigui.Widget, context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	if !m.popup.IsOpen() {
		return guigui.HandleInputResult{}
	}
	if inpututil.IsKeyJustPressed(ebiten.KeyEscape) {
		m.popup.SetOpen(false)
		return guigui.HandleInputByWidget(self)
	}
	return guigui.AbortHandlingInputByWidget(self)
}

func (m *modalDatePicker) measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return m.button.Measure(context, constraints)
}

// Modal is a button that opens a modal calendar dialog. The calendar matches
// the Material 3 docked date picker (month/year menus, day grid, OK/Cancel) and
// adds a headline plus a toggle into text-input mode.
type Modal struct {
	guigui.DefaultWidget
	modalDatePicker
}

func (m *Modal) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	return m.build(m, datePickerKindModal, context, adder)
}

func (m *Modal) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	m.layout(context, widgetBounds, layouter)
}

func (m *Modal) HandleButtonInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	return m.handleButtonInput(m, context, widgetBounds)
}

func (m *Modal) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return m.measure(context, constraints)
}

func (m *Modal) Value() Date {
	return m.value
}

func (m *Modal) SetValue(date Date) {
	m.value = date
}

func (m *Modal) SetTitle(title string) {
	m.title = title
}

func (m *Modal) SetButtonText(text string) {
	m.buttonText = text
}

func (m *Modal) SetFirstWeekday(day time.Weekday) {
	m.firstWeekday = day
}

func (m *Modal) SetMinDate(date Date) {
	m.minDate = date
}

func (m *Modal) SetMaxDate(date Date) {
	m.maxDate = date
}

func (m *Modal) OnValueChanged(f func(context *guigui.Context, date Date)) {
	guigui.SetEventHandler(m, datePickerEventValueChanged, f)
}

func (m *Modal) SetOpen(open bool) {
	m.setOpen(open)
}

func (m *Modal) IsOpen() bool {
	return m.popup.IsOpen()
}

// ModalDateInput is a button that opens a modal dialog for typing a date, with
// a toggle into the same calendar as [Modal].
type ModalDateInput struct {
	guigui.DefaultWidget
	modalDatePicker
}

func (m *ModalDateInput) Build(context *guigui.Context, adder *guigui.ChildAdder) error {
	return m.build(m, datePickerKindModalInput, context, adder)
}

func (m *ModalDateInput) Layout(context *guigui.Context, widgetBounds *guigui.WidgetBounds, layouter *guigui.ChildLayouter) {
	m.layout(context, widgetBounds, layouter)
}

func (m *ModalDateInput) HandleButtonInput(context *guigui.Context, widgetBounds *guigui.WidgetBounds) guigui.HandleInputResult {
	return m.handleButtonInput(m, context, widgetBounds)
}

func (m *ModalDateInput) Measure(context *guigui.Context, constraints guigui.Constraints) image.Point {
	return m.measure(context, constraints)
}

func (m *ModalDateInput) Value() Date {
	return m.value
}

func (m *ModalDateInput) SetValue(date Date) {
	m.value = date
}

func (m *ModalDateInput) SetTitle(title string) {
	m.title = title
}

func (m *ModalDateInput) SetButtonText(text string) {
	m.buttonText = text
}

func (m *ModalDateInput) SetFirstWeekday(day time.Weekday) {
	m.firstWeekday = day
}

func (m *ModalDateInput) SetMinDate(date Date) {
	m.minDate = date
}

func (m *ModalDateInput) SetMaxDate(date Date) {
	m.maxDate = date
}

func (m *ModalDateInput) OnValueChanged(f func(context *guigui.Context, date Date)) {
	guigui.SetEventHandler(m, datePickerEventValueChanged, f)
}

func (m *ModalDateInput) SetOpen(open bool) {
	m.setOpen(open)
}

func (m *ModalDateInput) IsOpen() bool {
	return m.popup.IsOpen()
}
