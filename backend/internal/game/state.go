// Package game establishes the backend-owned application boundary for M1 game
// state.
//
// It intentionally contains no package generation, delivery, finance or
// persistence logic — those belong to later bounded Acts. The authoritative
// in-memory clock is seeded from the canonical start time defined by SPEC (2.1)
// and advanced deterministically; M1C makes it mutable and concurrency-safe but
// introduces no background ticker or autonomous simulation loop.
package game

import (
	"errors"
	"sort"
	"sync"
	"time"
)

// InitialGameTime is the canonical start time for a new M1 game (SPEC 2.1):
// games begin on 1 February 1980 at 09:00 local game time.
const InitialGameTime = "1980-02-01T09:00:00"

// speedRates maps each allowed clock speed to its game-time advancement rate in
// game seconds per one real second, from SPEC 2.2 (at 1x one real second advances
// two game minutes = 120 game seconds; the rate scales with speed).
var speedRates = map[int]int{1: 120, 2: 240, 3: 360}

// ErrInvalidClockSpeed is returned by SetSpeed when a requested speed is not one of
// the SPEC-allowed values (1, 2, 3). The previous speed is left unchanged.
var ErrInvalidClockSpeed = errors.New("invalid clock speed")

// ValidSpeeds returns the clock speeds permitted by SPEC 2.2 in ascending order.
func ValidSpeeds() []int {
	speeds := make([]int, 0, len(speedRates))
	for s := range speedRates {
		speeds = append(speeds, s)
	}
	sort.Ints(speeds)
	return speeds
}

// Clock is the authoritative in-memory game clock owned by the application layer.
// It holds only the state that changes over a game; derived display values are
// computed on read and never stored. All access to mutable fields goes through
// methods guarded by mu, so the clock is safe for concurrent reads and writes.
// The mutex must not be copied: use *Clock (a reference) everywhere.
type Clock struct {
	mu     sync.RWMutex // guards now/speed/paused; never copy a Clock by value
	now    time.Time    // fictional game time; UTC used only as an internal calendar representation
	speed  int          // one of the SPEC-allowed speeds (1, 2, 3)
	paused bool
}

// NewClock returns the canonical initial M1 clock state: start time from SPEC 2.1
// (1 February 1980, 09:00), speed 1, not paused — consistent with the SPEC example
// clock state. It is a single authoritative instance shared by reference.
func NewClock() *Clock {
	// InitialGameTime is a fixed, well-formed constant (SPEC 2.1); parsing it cannot
	// fail in practice. UTC is used only as an internal calendar representation of
	// fictional game time — never the host wall clock.
	t, _ := time.Parse("2006-01-02T15:04:05", InitialGameTime)
	return &Clock{now: t, speed: 1, paused: false}
}

// ClockSnapshot is a read-only view of the clock. It is a value type whose fields
// are all immutable (string/int/bool), so callers cannot mutate authoritative state
// through a returned snapshot. JSON field naming lives in the API layer.
type ClockSnapshot struct {
	GameDatetime         string
	DayOfWeek            string
	Speed                int
	Paused               bool
	OfficeOpen           bool
	DaysUntilNextPayroll int
	DaysUntilNextRent    int
}

// Snapshot returns a read-only view of the current clock. It takes an internal read
// lock, computes derived fields, and releases the lock before returning — so no lock
// is held during any downstream (e.g., HTTP JSON) encoding. Repeated snapshots do not
// change state.
func (c *Clock) Snapshot() ClockSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
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

// SetPaused sets the paused state (true pauses advancement, false resumes). It changes
// only the paused flag; speed and game time are preserved. Safe under concurrent access.
func (c *Clock) SetPaused(paused bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.paused = paused
}

// SetSpeed validates and sets the configured speed. Only SPEC-allowed speeds (1, 2, 3)
// are accepted; an invalid value returns ErrInvalidClockSpeed and leaves the previous
// speed unchanged. Changing speed while paused is allowed and does not unpause. Safe
// under concurrent access.
func (c *Clock) SetSpeed(speed int) error {
	if _, ok := speedRates[speed]; !ok {
		return ErrInvalidClockSpeed
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.speed = speed
	return nil
}

// Advance deterministically advances game time by the given real-world elapsed duration
// at the current configured speed. A paused clock does not advance, and a non-positive
// elapsed is a no-op. It uses no wall clock, sleep, ticker or goroutine: a
// deterministic input yields a deterministic result. This primitive will later be driven
// by a simulation loop; M1C contains no such loop.
func (c *Clock) Advance(elapsed time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.paused || elapsed <= 0 {
		return
	}
	rate, ok := speedRates[c.speed]
	if !ok {
		return // defensive: speed is always valid via NewClock/SetSpeed
	}
	// At speed s, one real second advances rate game seconds (SPEC 2.2). Scaling the
	// elapsed Duration (in nanoseconds) by that integer ratio yields the exact game-time
	// delta; converting back to a Duration gives the amount of game time to add.
	advance := time.Duration(int64(elapsed) * int64(rate))
	c.now = c.now.Add(advance)
}

// SkipToNextOpening advances game time to the earliest upcoming office opening per SPEC
// 2.4 working hours. If the office is currently open it is a safe no-op (game time,
// speed and paused state are all preserved). It changes only game date/time; it preserves
// speed and paused state and uses no wall clock.
func (c *Clock) SkipToNextOpening() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if officeOpen(c.now) {
		return // currently open → safe no-op
	}
	c.now = nextOpeningInstant(c.now)
}

// openingInterval returns the [startMin, endMin) working-hours interval for a weekday per
// SPEC 2.4 (Mon–Fri 09:00–17:00, Sat 10:00–13:00), or hasOpening=false for Sunday.
// Intervals are half-open [opening, closing): exactly at opening → open; exactly at
// closing → closed.
func openingInterval(wd time.Weekday) (startMin, endMin int, hasOpening bool) {
	switch wd {
	case time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday:
		return 9 * 60, 17 * 60, true
	case time.Saturday:
		return 10 * 60, 13 * 60, true
	default: // Sunday
		return 0, 0, false
	}
}

// officeOpen reports whether the office is open at t per SPEC 2.4 working hours.
func officeOpen(t time.Time) bool {
	startMin, endMin, has := openingInterval(t.Weekday())
	if !has {
		return false
	}
	now := t.Hour()*60 + t.Minute()
	return now >= startMin && now < endMin
}

// nextOpeningInstant returns the earliest office-opening instant strictly after t. It is
// only called when the office is currently closed, so the result is always in the future.
// The weekly schedule guarantees an opening within a few days; searching 8 consecutive
// days always finds one (the panic below guards an unreachable invariant).
func nextOpeningInstant(t time.Time) time.Time {
	for i := 0; i < 8; i++ {
		day := t.AddDate(0, 0, i)
		startMin, _, has := openingInterval(day.Weekday())
		if !has {
			continue // Sunday: no opening
		}
		midnight := time.Date(day.Year(), day.Month(), day.Day(), 0, 0, 0, 0, t.Location())
		candidate := midnight.Add(time.Duration(startMin) * time.Minute)
		if candidate.After(t) {
			return candidate
		}
	}
	panic("game: weekly schedule always has an upcoming office opening") // unreachable
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

// daysUntilNext returns the number of days from t's date until the next occurrence of
// target weekday (0 if today is already that day). Go numbers weekdays Sunday=0..Saturday=6,
// so a forward modular difference gives the distance to the next occurrence.
func daysUntilNext(t time.Time, target time.Weekday) int {
	return (int(target) - int(t.Weekday()) + 7) % 7
}

// GameState is the minimal in-memory game state created at backend startup when no
// persisted game exists yet. It owns exactly one authoritative clock; later Acts will
// extend it with player, office and other domains.
type GameState struct {
	Clock *Clock
}

// NewInitialState returns the default M1 game state seeded from the canonical start time
// defined by SPEC (SPEC 2.1).
func NewInitialState() *GameState {
	return &GameState{Clock: NewClock()}
}
