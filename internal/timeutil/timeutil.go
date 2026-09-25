// Package timeutil provides DST-correct conversions between zone-local
// wall-clock times and absolute instants, plus helpers for projecting a
// recurring weekly pattern onto a concrete calendar week in a viewer's zone.
//
// Deliberately dependency-free (standard library only) and imports no
// Chronicle plugin packages, so multiple callers can share one converter.
//
// The load-bearing rule: a *recurring* availability block is stored as a
// zone-local wall-clock (weekday + minute-of-local-midnight + IANA zone),
// never as a UTC instant. It only becomes an absolute instant when projected
// onto a specific real-world date, because the UTC offset of "18:00 local"
// depends on whether that date is inside daylight-saving time.
package timeutil

import "time"

// MinutesPerDay is the number of minutes in a 24-hour civil day. Availability
// minute-of-day values live in [0, MinutesPerDay]; an end value equal to
// MinutesPerDay means "up to local midnight".
const MinutesPerDay = 24 * 60

// LoadLocation returns the IANA location for name, or time.UTC when name is
// empty or not resolvable. Availability rows carry their own IANA zone; a
// missing or unknown zone degrades safely to UTC rather than failing the
// whole overlay render. Callers that need to *reject* a bad zone (e.g. on
// write) should use time.LoadLocation directly.
func LoadLocation(name string) *time.Location {
	if name == "" {
		return time.UTC
	}
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.UTC
	}
	return loc
}

// IsValidLocation reports whether name resolves to a real IANA zone. Empty is
// not valid (callers should require an explicit zone on write).
func IsValidLocation(name string) bool {
	if name == "" {
		return false
	}
	_, err := time.LoadLocation(name)
	return err == nil
}

// WallClockInstant resolves a zone-local wall-clock — a civil date plus a
// minute offset from local midnight — to an absolute instant. DST-correct:
// time.Date computes the zone offset against the real (y, m, d), so the same
// minuteOfDay maps to different absolute instants across a DST transition.
//
// minuteOfDay may be 0..MinutesPerDay (or beyond); time.Date normalizes it,
// so MinutesPerDay correctly rolls to 00:00 of the following civil day —
// which is exactly what an end-of-day block boundary should mean.
func WallClockInstant(loc *time.Location, year int, month time.Month, day, minuteOfDay int) time.Time {
	if loc == nil {
		loc = time.UTC
	}
	// Hour is 0 and the whole time-of-day is expressed as minutes so callers
	// never have to pre-split hours/minutes; time.Date normalizes overflow.
	return time.Date(year, month, day, 0, minuteOfDay, 0, 0, loc)
}

// LocalWallClock converts an absolute instant into the given zone and returns
// the local weekday and the minute-of-local-midnight. Used to place a UTC
// instant onto a viewer's weekly grid.
func LocalWallClock(t time.Time, loc *time.Location) (weekday time.Weekday, minuteOfDay int) {
	if loc == nil {
		loc = time.UTC
	}
	lt := t.In(loc)
	return lt.Weekday(), lt.Hour()*60 + lt.Minute()
}

// CivilDate is a timezone-independent calendar date (a real Gregorian day).
// A date's weekday is the same in every zone, which is why the overlay keys
// its columns on CivilDate rather than on an instant.
type CivilDate struct {
	Year  int
	Month time.Month
	Day   int
}

// ParseCivilDate parses a YYYY-MM-DD string into a CivilDate.
func ParseCivilDate(s string) (CivilDate, error) {
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return CivilDate{}, err
	}
	return CivilDate{Year: t.Year(), Month: t.Month(), Day: t.Day()}, nil
}

// String renders the date as YYYY-MM-DD.
func (d CivilDate) String() string {
	return d.midnightUTC().Format("2006-01-02")
}

// Weekday returns the day of week (Sunday=0..Saturday=6). Zone-independent.
func (d CivilDate) Weekday() time.Weekday {
	return d.midnightUTC().Weekday()
}

// AddDays returns the civil date n days after d (n may be negative). It
// normalizes across month and year boundaries via time arithmetic.
func (d CivilDate) AddDays(n int) CivilDate {
	t := d.midnightUTC().AddDate(0, 0, n)
	return CivilDate{Year: t.Year(), Month: t.Month(), Day: t.Day()}
}

// midnightUTC anchors the civil date at 00:00 UTC purely for calendar
// arithmetic and weekday derivation — never used as a real instant.
func (d CivilDate) midnightUTC() time.Time {
	return time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, time.UTC)
}

// StartOfCivilDay returns the first real instant of civil date d in loc.
//
// `time.Date(y, m, d, 0, 0, 0, 0, loc)` is not the start of the day in every
// zone: where a DST jump lands on midnight (e.g. 00:00 → 01:00), local 00:00
// does not exist and Go normalises it backwards to 23:00 the previous day. A
// caller using that naive expression as a day boundary can get a boundary at
// or before the instant it already holds, which can make a day-splitting loop
// non-terminating. Every "end of the local day" computation must come through
// here instead.
//
// The returned instant is always on d in loc, and is always the earliest such
// instant, so it is a strictly-increasing function of the civil date — the
// property a splitting loop needs to terminate.
func StartOfCivilDay(loc *time.Location, d CivilDate) time.Time {
	if loc == nil {
		loc = time.UTC
	}
	naive := time.Date(d.Year, d.Month, d.Day, 0, 0, 0, 0, loc)
	if y, m, dd := naive.Date(); y == d.Year && m == d.Month && dd == d.Day {
		return naive
	}
	// Local midnight does not exist on this date; `naive` normalised backwards
	// into the previous local day. Step a minute at a time from it to find
	// the transition instant (the first instant whose local date is d); zone
	// transitions land on whole minutes and the largest IANA jump is two
	// hours, so this settles in <=120 steps. The 24h ceiling bounds a future
	// tzdata oddity to a wrong-but-finite answer instead of an unbounded loop.
	for i := 1; i <= 24*60; i++ {
		t := naive.Add(time.Duration(i) * time.Minute)
		if y, m, dd := t.Date(); y == d.Year && m == d.Month && dd == d.Day {
			return t
		}
	}
	return naive.Add(24 * time.Hour)
}
