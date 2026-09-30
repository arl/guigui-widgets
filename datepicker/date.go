package datepicker

import (
	"cmp"
	"fmt"
	"strconv"
	"strings"
	"time"
)

const (
	datePickerMinYear = 1900
	datePickerMaxYear = 2100
)

// Date is a calendar day without a time of day or location.
// The zero value is an unset date.
type Date struct {
	Year  int
	Month time.Month
	Day   int
}

// Today returns the current local date.
func Today() Date {
	return DateFromTime(time.Now())
}

// DateFromTime returns the year, month, and day of t in t's location.
func DateFromTime(t time.Time) Date {
	y, m, d := t.Date()
	return Date{Year: y, Month: m, Day: d}
}

// NewDate returns the date y/m/d. The values are not normalized.
func NewDate(year int, month time.Month, day int) Date {
	return Date{Year: year, Month: month, Day: day}
}

// IsZero reports whether d is the unset date.
func (d Date) IsZero() bool {
	return d.Year == 0 && d.Month == 0 && d.Day == 0
}

// Valid reports whether d is a real calendar day (for example, 2026-02-30 is not).
func (d Date) Valid() bool {
	if d.IsZero() {
		return false
	}
	t := d.Time()
	return t.Year() == d.Year && t.Month() == d.Month && t.Day() == d.Day
}

// Equal reports whether d and o are the same day.
func (d Date) Equal(o Date) bool {
	return d.Year == o.Year && d.Month == o.Month && d.Day == o.Day
}

// Compare returns -1 if d is before o, 0 if they are equal, and +1 if d is after o.
func (d Date) Compare(o Date) int {
	if c := cmp.Compare(d.Year, o.Year); c != 0 {
		return c
	}
	if c := cmp.Compare(int(d.Month), int(o.Month)); c != 0 {
		return c
	}
	return cmp.Compare(d.Day, o.Day)
}

// Time returns local midnight on d.
func (d Date) Time() time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.Local)
}

// String returns d in ISO-8601 form (2006-01-02). The zero date is "".
func (d Date) String() string {
	if d.IsZero() {
		return ""
	}
	return fmt.Sprintf("%04d-%02d-%02d", d.Year, d.Month, d.Day)
}

func (d Date) addDays(n int) Date {
	return DateFromTime(d.Time().AddDate(0, 0, n))
}

func clampDate(d Date) Date {
	if d.Year < datePickerMinYear {
		d.Year = datePickerMinYear
		d.Month = time.January
		d.Day = 1
	}
	if d.Year > datePickerMaxYear {
		d.Year = datePickerMaxYear
		d.Month = time.December
		d.Day = 31
	}
	return d
}

func formatDate(d Date) string {
	if d.IsZero() {
		return ""
	}
	return d.Time().Format("02/01/2006")
}

func formatDateHeadline(d Date) string {
	if d.IsZero() {
		return "Selected date"
	}
	return d.Time().Format("Mon, Jan 2")
}

func parseDate(s string) (Date, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return Date{}, true
	}
	t, err := time.ParseInLocation("02/01/2006", s, time.Local)
	if err != nil {
		return Date{}, false
	}
	d := DateFromTime(t)
	if !d.Valid() {
		return Date{}, false
	}
	return d, true
}

func weekdayLetter(day time.Weekday) string {
	return []string{"S", "M", "T", "W", "T", "F", "S"}[int(day)]
}

func datePickerYearString(year int) string {
	return strconv.Itoa(year)
}
