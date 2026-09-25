package calendar

import "time"

// gregorian.go — the two real-world date helpers the domain model depends on.

// daysInGregorianMonth returns the length of a real-world month, leap years
// included. `month` is 0-based, matching the model's real-time month index —
// time.Date normalises day 0 of month+1 to the last day of month.
func daysInGregorianMonth(year, month int) int {
	return time.Date(year, time.Month(month+1), 0, 0, 0, 0, 0, time.UTC).Day()
}

// gregorianJDN is the Julian Day Number for a proleptic-Gregorian date (the
// standard Fliegel–Van Flandern integer formula).
//
// It must be the ONE shared day counter for real-time calendars — moon phase
// math, the weekday column and recurrence expansion all count through it, so
// none of them can drift from the others across a leap day.
func gregorianJDN(y, m, d int) int {
	a := (14 - m) / 12
	yy := y + 4800 - a
	mm := m + 12*a - 3
	return d + (153*mm+2)/5 + 365*yy + yy/4 - yy/100 + yy/400 - 32045
}
