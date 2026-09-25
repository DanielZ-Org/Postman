// Package game establishes the backend-owned application boundary for game state:
// the authoritative clock, player, selected office, packages, employees, delivery
// runs and finance records. All mutations are validated by game rules under one lock
// before any state transition is committed; the API layer only transports JSON.
package game

import (
	"errors"
	"sort"
	"sync"
	"time"
)

// InitialGameTime is the canonical start time for a new game (SPEC 2.1): games begin
// on 1 February 1980 at 09:00 local game time.
const InitialGameTime = "1980-02-01T09:00:00"

// GameTimeFormat is the canonical wire/persistence format for fictional game
// instants (e.g. "1980-02-01T09:00:00").
const GameTimeFormat = "2006-01-02T15:04:05"

// Game status values. "running" is the normal state; "game_over" (SPEC 4.2) is a
// labelled temporary status name — the SPEC only says "the game ends" without naming
// the status. Reaching it freezes all scheduled events and pauses the clock.
const (
	GameStatusRunning  = "running"
	GameStatusGameOver = "game_over"
)

// startTime parses the canonical start instant once for statement periods and
// simulation schedules. The constant is fixed and well-formed; parsing cannot fail.
var startTime, _ = time.Parse(GameTimeFormat, "1980-02-01T09:00:00")

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

// NewClock returns the canonical initial clock state: start time from SPEC 2.1
// (1 February 1980, 09:00), speed 1, not paused - consistent with the SPEC example
// clock state. It is a single authoritative instance shared by reference.
func NewClock() *Clock {
	// InitialGameTime is a fixed, well-formed constant (SPEC 2.1); parsing it cannot
	// fail in practice. UTC is used only as an internal calendar representation of
	// fictional game time - never the host wall clock.
	t, _ := time.Parse(GameTimeFormat, "1980-02-01T09:00:00")
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
// lock, computes derived fields, and releases the lock before returning - so no lock
// is held during any downstream (e.g., HTTP JSON) encoding. Repeated snapshots do not
// change state.
func (c *Clock) Snapshot() ClockSnapshot {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return ClockSnapshot{
		GameDatetime:         c.now.Format(GameTimeFormat),
		DayOfWeek:            weekdayName(c.now),
		Speed:                c.speed,
		Paused:               c.paused,
		OfficeOpen:           officeOpen(c.now),
		DaysUntilNextPayroll: daysUntilNext(c.now, time.Tuesday), // SPEC 2.5: Tuesday payroll
		DaysUntilNextRent:    daysUntilNext(c.now, time.Friday),  // SPEC 2.5: Friday rent payment
	}
}

// Now returns the authoritative current game time. It takes an internal read lock and releases it
// before returning; no wall clock is used. This lets callers (e.g., office selection) derive
// deterministic values from the exact fictional instant without holding a lock themselves.
func (c *Clock) Now() time.Time {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.now
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
// deterministic input yields a deterministic result. The simulation loop drives it
// with real elapsed time between ticks.
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
// speed and paused state are all preserved). It changes only game date/time; it
// preserves speed and paused state and uses no wall clock.
func (c *Clock) SkipToNextOpening() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if officeOpen(c.now) {
		return // currently open - safe no-op
	}
	c.now = nextOpeningInstant(c.now)
}

// parts returns the raw clock fields under a read lock (used by snapshotting, which
// already holds the owning GameState lock and must not recurse into Snapshot's
// derived fields).
func (c *Clock) parts() (time.Time, int, bool) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.now, c.speed, c.paused
}

// restore replaces the clock's authoritative fields (used when loading persisted
// state; the caller holds the owning GameState lock).
func (c *Clock) restore(now time.Time, speed int, paused bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.now = now
	c.speed = speed
	c.paused = paused
}

// openingInterval returns the [startMin, endMin) working-hours interval for a weekday per
// SPEC 2.4 (Mon-Fri 09:00-17:00, Sat 10:00-13:00), or hasOpening=false for Sunday.
// Intervals are half-open [opening, closing): exactly at opening - open; exactly at
// closing - closed.
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

// closingInstantToday returns today's closing time, or hasClosing=false when the
// office has no opening today (Sunday).
func closingInstantToday(t time.Time) (time.Time, bool) {
	_, endMin, has := openingInterval(t.Weekday())
	if !has {
		return time.Time{}, false
	}
	midnight := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
	return midnight.Add(time.Duration(endMin) * time.Minute), true
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

// dayKey returns the calendar-date key ("2006-01-02") used to detect game-day
// rollovers for daily counters.
func dayKey(t time.Time) string {
	return t.Format("2006-01-02")
}

// StartingCash is the canonical starting player cash for a new game (SPEC 4.3): £1000,
// represented as integer pounds.
const StartingCash = 1000

// GameState is the authoritative in-memory game state created at backend startup when no
// persisted game exists yet. It owns the clock, player, selected office, packages,
// employees, delivery runs, finance records and the scheduling cursors the simulation
// advances. All mutable fields are guarded by mu; use *GameState (a reference)
// everywhere - never copy it. Lock order: GameState.mu may be held while taking a
// Clock lock; never the reverse.
type GameState struct {
	mu             sync.Mutex // guards every mutable field below; never copy a GameState by value
	Clock          *Clock     // authoritative clock (owns its own lock)
	player         Player
	selectedOffice *RuntimeOffice // nil until an office is selected
	cash           int            // authoritative cash balance, integer pounds (starts at StartingCash)
	transactions   []Transaction
	packages       []*Package
	employees      []*Employee
	runs           []*Run
	status         string // GameStatusRunning or GameStatusGameOver

	nextTxnID  int // 1-based; the initial loan transaction consumes 1
	nextPkgSeq int // 1-based package id sequence
	nextEmpSeq int // 1-based employee id sequence
	totalHires int // lifetime hire counter driving the hiring fee (SPEC 8)

	lastGeneration time.Time // package-generation cursor (30-minute grid)
	interestDue    time.Time // next four-week loan-interest instant (SPEC 11.1)
	payrollDue     time.Time // next Tuesday payroll instant (SPEC 11.2)

	deliveredToday    int // packages delivered on deliveredTodayKey (game date)
	deliveredTodayKey string
	dailyRevenue      int // revenue accrued on dailyRevenueKey; settled at day rollover
	dailyRevenueKey   string
}

// NewInitialState returns the default game state seeded from the canonical start
// time and starting cash defined by SPEC, with the initial loan transaction on
// record and all schedules (generation cursor, first interest, first payroll) set.
func NewInitialState() *GameState {
	s := &GameState{
		Clock:             NewClock(),
		player:            defaultPlayer,
		cash:              StartingCash,
		status:            GameStatusRunning,
		nextTxnID:         1,
		lastGeneration:    startTime,
		interestDue:       startTime.AddDate(0, 0, intervalWeeks*7),
		payrollDue:        nextPayrollInstant(startTime),
		deliveredTodayKey: dayKey(startTime),
		dailyRevenueKey:   dayKey(startTime),
	}
	// The starting loan is the opening condition, not a mutation: cash is seeded at
	// StartingCash and the disbursement transaction records why.
	s.transactions = []Transaction{{
		ID:           txnID(1),
		GameDatetime: InitialGameTime,
		Category:     CategoryLoanDisbursement,
		Amount:       StartingCash,
		Description:  "Starting loan",
		ReferenceID:  loanReferenceID,
	}}
	return s
}

// nextPayrollInstant returns the first payroll moment (Tuesday 09:00, SPEC 11.2)
// strictly after from.
func nextPayrollInstant(from time.Time) time.Time {
	days := (int(payrollWeekday) - int(from.Weekday()) + 7) % 7
	day := time.Date(from.Year(), from.Month(), from.Day()+days, payrollHour, 0, 0, 0, from.Location())
	if !day.After(from) {
		day = day.AddDate(0, 0, 7)
	}
	return day
}

// SelectedOffice returns the currently selected office, or nil if none has been selected yet.
func (s *GameState) SelectedOffice() *RuntimeOffice {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.selectedOffice
}

// Cash returns the current authoritative cash balance in integer pounds.
func (s *GameState) Cash() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.cash
}

// Transactions returns a copy of the finance transaction list; callers cannot mutate the
// authoritative collection through the returned slice.
func (s *GameState) Transactions() []Transaction {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Transaction, len(s.transactions))
	copy(out, s.transactions)
	return out
}

// Status returns the current game status ("running" or "game_over").
func (s *GameState) Status() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.status
}
