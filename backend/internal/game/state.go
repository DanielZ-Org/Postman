// Package game establishes the backend-owned application boundary for M1 game
// state.
//
// It intentionally contains no package generation, delivery, finance or
// persistence logic — those belong to later bounded Acts. The authoritative
// in-memory clock is seeded from the canonical start time defined by SPEC (2.1)
// and, in M1B, is exposed read-only with no ticking or advancement.
package game

import "time"

// InitialGameTime is the canonical start time for a new M1 game (SPEC 2.1):
// games begin on 1 February 1980 at 09:00 local game time.
const InitialGameTime = "1980-02-01T09:00:00"

// Clock is the authoritative in-memory game clock owned by the application
// layer. It holds only the state that changes over a game; derived display
// values (day of week, office open, days until payroll/rent) are computed on
// read and never stored. In M1B the clock is created once at startup and never
// mutated, so concurrent reads require no synchronization.
type Clock struct {
	now    time.Time // fictional game time; UTC used only as an internal calendar representation
	speed  int       // 1, 2 or 3 (SPEC 2.2)
	paused bool
}

// NewClock returns the canonical initial M1 clock state:
//   - start time from SPEC 2.1 (1 February 1980, 09:00);
//   - speed 1 and not paused;
//
// consistent with the SPEC example clock state.
func NewClock() Clock {
	// InitialGameTime is a fixed, well-formed constant (SPEC 2.1), so parsing it
	// cannot fail in practice. UTC is used only as an internal calendar
	// representation of fictional game time — never the host wall clock.
	t, _ := time.Parse("2006-01-02T15:04:05", InitialGameTime)
	return Clock{now: t, speed: 1, paused: false}
}

// ClockSnapshot is a read-only view of the clock. It is a value type whose
// fields are all immutable (string/int/bool), so callers cannot mutate
// authoritative state through a returned snapshot. JSON field naming is a
// transport concern and lives in the API layer, not here.
type ClockSnapshot struct {
	GameDatetime         string
	DayOfWeek            string
	Speed                int
	Paused               bool
	OfficeOpen           bool
	DaysUntilNextPayroll int
	DaysUntilNextRent    int
}

// Snapshot returns a read-only view of the current clock. It does not mutate
// state, has no side effects, and does not depend on the computer wall clock.
func (c Clock) Snapshot() ClockSnapshot {
	return ClockSnapshot{
		GameDatetime:         c.now.Format("2006-01-02T15:04:05"),
		DayOfWeek:            weekdayName(c.now),
		Speed:                c.speed,
		Paused:               c.paused,
		OfficeOpen:           officeOpen(c.now),
		DaysUntilNextPayroll: daysUntilNext(c.now, time.Tuesday), // SPEC 2.5: Tuesday payroll
		DaysUntilNextRent:    daysUntilNext(c.now, time.Friday),  // SPEC 2.5: Friday rent payment
	}
}

// weekdayName returns the lowercase day-of-week name used by the API contract.
func weekdayName(t time.Time) string {
	switch t.Weekday() {
	case time.Monday:
		return "monday"
	case time.Tuesday:
		return "tuesday"
	case time.Wednesday:
		return "wednesday"
	case time.Thursday:
		return "thursday"
	case time.Friday:
		return "friday"
	case time.Saturday:
		return "saturday"
	default:
		return "sunday"
	}
}

// officeOpen reports whether the office is open at t per SPEC 2.4 working hours:
// Monday–Friday 09:00–17:00, Saturday 10:00–13:00, Sunday closed. The interval
// is treated as [start, end) in minutes since midnight.
func officeOpen(t time.Time) bool {
	var start, end int
	switch t.Weekday() {
	case time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday:
		start, end = 9*60, 17*60
	case time.Saturday:
		start, end = 10*60, 13*60
	default: // Sunday
		return false
	}

	now := t.Hour()*60 + t.Minute()
	return now >= start && now < end
}

// daysUntilNext returns the number of days from t's date until the next
// occurrence of target weekday (0 if today is already that day). Go numbers
// weekdays Sunday=0..Saturday=6, so a forward modular difference gives the
// distance to the next occurrence.
func daysUntilNext(t time.Time, target time.Weekday) int {
	return (int(target) - int(t.Weekday()) + 7) % 7
}

// GameState is the minimal in-memory game state created at backend startup when
// no persisted game exists yet. It currently holds only the authoritative clock;
// later Acts will extend it with player, office and other domains.
type GameState struct {
	Clock Clock
}

// NewInitialState returns the default M1 game state seeded from the canonical
// start time defined by SPEC (SPEC 2.1).
func NewInitialState() GameState {
	return GameState{Clock: NewClock()}
}
