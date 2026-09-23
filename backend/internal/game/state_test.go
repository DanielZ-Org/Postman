package game

import (
	"sync"
	"testing"
	"time"
)

// newClockAt constructs a Clock at an arbitrary fictional instant for deterministic
// testing. Same-package tests may set the unexported speed/paused fields directly.
func newClockAt(t time.Time) *Clock {
	return &Clock{now: t, speed: 1, paused: false}
}

func TestInitialGameTimeMatchesSpec(t *testing.T) {
	if InitialGameTime != "1980-02-01T09:00:00" {
		t.Fatalf("InitialGameTime = %q, want the canonical SPEC 2.1 start time", InitialGameTime)
	}
}

func TestNewClockUsesCanonicalStartTime(t *testing.T) {
	got := NewClock().Snapshot()
	if got.GameDatetime != "1980-02-01T09:00:00" {
		t.Fatalf("new clock game_datetime = %q, want the canonical start time", got.GameDatetime)
	}
}

func TestNewClockInitialSpeedAndPausedMatchSpec(t *testing.T) {
	snap := NewClock().Snapshot()
	if snap.Speed != 1 {
		t.Errorf("initial speed = %d, want 1 (SPEC example)", snap.Speed)
	}
	if snap.Paused {
		t.Error("new clock is paused; SPEC example starts unpaused")
	}
}

func TestNewClockDerivedFieldsMatchSpecExample(t *testing.T) {
	snap := NewClock().Snapshot()
	if snap.DayOfWeek != "friday" {
		t.Errorf("day_of_week = %q, want friday (1 February 1980)", snap.DayOfWeek)
	}
	if !snap.OfficeOpen {
		t.Error("office_open = false at the canonical start; SPEC example is open")
	}
	if snap.DaysUntilNextPayroll != 4 {
		t.Errorf("days_until_next_payroll = %d, want 4 (next Tuesday)", snap.DaysUntilNextPayroll)
	}
	if snap.DaysUntilNextRent != 0 {
		t.Errorf("days_until_next_rent = %d, want 0 (today is Friday)", snap.DaysUntilNextRent)
	}
}

func TestReadingClockDoesNotMutateState(t *testing.T) {
	c := NewClock()
	first := c.Snapshot()
	for i := 0; i < 3; i++ {
		if got := c.Snapshot(); got != first {
			t.Fatalf("snapshot %d = %+v, want unchanged %+v", i+1, got, first)
		}
	}
}

func TestReturnedSnapshotCannotMutateAuthoritativeState(t *testing.T) {
	c := NewClock()
	mutated := c.Snapshot()
	mutated.Speed = 99
	mutated.Paused = true
	if got := c.Snapshot(); got.Speed != 1 || got.Paused {
		t.Fatalf("authoritative clock changed after mutating a returned snapshot: %+v", got)
	}
}

func TestNewInitialStateSeedsCanonicalClock(t *testing.T) {
	got := NewInitialState()
	if got.Clock.Snapshot().GameDatetime != "1980-02-01T09:00:00" {
		t.Fatalf("initial state clock = %q, want the canonical start time", got.Clock.Snapshot().GameDatetime)
	}
}

// --- M1C: pause mutation -------------------------------------------------------

func TestSetPausedTrue(t *testing.T) {
	c := NewClock()
	c.SetPaused(true)
	if !c.Snapshot().Paused {
		t.Fatal("paused = false after SetPaused(true), want true")
	}
}

func TestSetPausedFalse(t *testing.T) {
	c := NewClock()
	c.SetPaused(true)
	c.SetPaused(false)
	if c.Snapshot().Paused {
		t.Fatal("paused = true after SetPaused(false), want false")
	}
}

func TestSetPausedPreservesSpeedAndTime(t *testing.T) {
	c := newClockAt(time.Date(1980, 2, 4, 12, 0, 0, 0, time.UTC))
	c.speed = 3
	before := c.Snapshot()
	c.SetPaused(true)
	after := c.Snapshot()
	if after.Speed != before.Speed {
		t.Errorf("speed changed from %d to %d; pause must preserve speed", before.Speed, after.Speed)
	}
	if after.GameDatetime != before.GameDatetime {
		t.Errorf("time changed from %s to %s; pause must preserve game time", before.GameDatetime, after.GameDatetime)
	}
}

func TestPausedAdvanceDoesNotMoveTime(t *testing.T) {
	c := newClockAt(time.Date(1980, 2, 4, 12, 0, 0, 0, time.UTC))
	c.SetPaused(true)
	before := c.Snapshot().GameDatetime
	c.Advance(5 * time.Second)
	if got := c.Snapshot().GameDatetime; got != before {
		t.Fatalf("paused clock advanced from %s to %s; paused must not change game time", before, got)
	}
}

// --- M1C: speed mutation -------------------------------------------------------

func TestSetSpeedValidValues(t *testing.T) {
	for _, want := range []int{1, 2, 3} {
		c := NewClock()
		if err := c.SetSpeed(want); err != nil {
			t.Fatalf("SetSpeed(%d) returned error %v; want accepted", want, err)
		}
		if got := c.Snapshot().Speed; got != want {
			t.Errorf("speed = %d after SetSpeed(%d), want %d", got, want, want)
		}
	}
}

func TestSetSpeedInvalidRejectedAndPreservesPrevious(t *testing.T) {
	for _, invalid := range []int{0, 4, -1, 99} {
		c := NewClock()
		if err := c.SetSpeed(invalid); err != ErrInvalidClockSpeed {
			t.Fatalf("SetSpeed(%d) error = %v, want ErrInvalidClockSpeed", invalid, err)
		}
		if got := c.Snapshot().Speed; got != 1 {
			t.Errorf("speed = %d after rejected SetSpeed(%d), want previous value 1 preserved", got, invalid)
		}
	}
}

func TestSetSpeedWhilePausedDoesNotUnpause(t *testing.T) {
	c := NewClock()
	c.SetPaused(true)
	if err := c.SetSpeed(3); err != nil {
		t.Fatalf("changing speed while paused returned %v; want allowed", err)
	}
	snap := c.Snapshot()
	if snap.Speed != 3 {
		t.Errorf("speed = %d after change-while-paused, want 3", snap.Speed)
	}
	if !snap.Paused {
		t.Error("clock unpaused by a speed change; changing speed must not unpause")
	}
}

// --- M1C: deterministic advancement -------------------------------------------

func TestAdvanceExactAtEachSpeed(t *testing.T) {
	cases := []struct {
		speed int
		want  string // expected game_datetime after advancing one real second from the start
	}{
		{1, "1980-02-01T09:02:00"}, // +120s = 2 game minutes
		{2, "1980-02-01T09:04:00"}, // +240s = 4 game minutes
		{3, "1980-02-01T09:06:00"}, // +360s = 6 game minutes
	}
	for _, tc := range cases {
		c := NewClock()
		if err := c.SetSpeed(tc.speed); err != nil {
			t.Fatalf("SetSpeed(%d) error %v", tc.speed, err)
		}
		c.Advance(1 * time.Second)
		if got := c.Snapshot().GameDatetime; got != tc.want {
			t.Errorf("speed %d: advance 1 real second -> %s, want %s", tc.speed, got, tc.want)
		}
	}
}

func TestAdvanceIsDeterministic(t *testing.T) {
	a := NewClock()
	b := NewClock()
	a.Advance(37 * time.Second)
	b.Advance(37 * time.Second)
	if a.Snapshot().GameDatetime != b.Snapshot().GameDatetime {
		t.Fatalf("same input gave different results: %s vs %s", a.Snapshot().GameDatetime, b.Snapshot().GameDatetime)
	}
}

func TestAdvanceNonPositiveIsNoOp(t *testing.T) {
	c := NewClock()
	before := c.Snapshot().GameDatetime
	c.Advance(0)
	c.Advance(-time.Second)
	if got := c.Snapshot().GameDatetime; got != before {
		t.Fatalf("non-positive elapsed advanced time from %s to %s", before, got)
	}
}

// --- M1C: skip-to-next-opening -------------------------------------------------

func TestSkipToNextOpeningBoundaries(t *testing.T) {
	cases := []struct {
		name string
		from time.Time
		want time.Time // expected instant after SkipToNextOpening (== from when currently open)
	}{
		{"weekday pre-open", time.Date(1980, 2, 4, 8, 0, 0, 0, time.UTC), time.Date(1980, 2, 4, 9, 0, 0, 0, time.UTC)},
		{"weekday exact opening (open)", time.Date(1980, 2, 4, 9, 0, 0, 0, time.UTC), time.Date(1980, 2, 4, 9, 0, 0, 0, time.UTC)},
		{"weekday during open", time.Date(1980, 2, 4, 12, 0, 0, 0, time.UTC), time.Date(1980, 2, 4, 12, 0, 0, 0, time.UTC)},
		{"weekday exact closing", time.Date(1980, 2, 4, 17, 0, 0, 0, time.UTC), time.Date(1980, 2, 5, 9, 0, 0, 0, time.UTC)},
		{"Friday after close", time.Date(1980, 2, 8, 17, 0, 0, 0, time.UTC), time.Date(1980, 2, 9, 10, 0, 0, 0, time.UTC)},
		{"Saturday pre-open", time.Date(1980, 2, 9, 9, 0, 0, 0, time.UTC), time.Date(1980, 2, 9, 10, 0, 0, 0, time.UTC)},
		{"Saturday during open", time.Date(1980, 2, 9, 11, 0, 0, 0, time.UTC), time.Date(1980, 2, 9, 11, 0, 0, 0, time.UTC)},
		{"Saturday exact close", time.Date(1980, 2, 9, 13, 0, 0, 0, time.UTC), time.Date(1980, 2, 11, 9, 0, 0, 0, time.UTC)},
		{"Sunday", time.Date(1980, 2, 3, 12, 0, 0, 0, time.UTC), time.Date(1980, 2, 4, 9, 0, 0, 0, time.UTC)},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := newClockAt(tc.from)
			c.SkipToNextOpening()
			got := c.now
			if !got.Equal(tc.want) {
				t.Fatalf("skip from %s -> %s, want %s", tc.from.Format(time.RFC3339), got.Format(time.RFC3339), tc.want.Format(time.RFC3339))
			}
		})
	}
}

func TestSkipPreservesSpeedAndPause(t *testing.T) {
	c := newClockAt(time.Date(1980, 2, 4, 17, 0, 0, 0, time.UTC)) // Monday after close (closed)
	c.speed = 3
	c.SetPaused(true)
	c.SkipToNextOpening()
	snap := c.Snapshot()
	if snap.Speed != 3 {
		t.Errorf("speed = %d after skip, want preserved 3", snap.Speed)
	}
	if !snap.Paused {
		t.Error("skip unpaused the clock; skip must preserve paused state")
	}
}

func TestSkipCurrentlyOpenIsNoOp(t *testing.T) {
	c := newClockAt(time.Date(1980, 2, 4, 12, 0, 0, 0, time.UTC)) // Monday mid-morning (open)
	before := c.now
	c.SkipToNextOpening()
	if !c.now.Equal(before) {
		t.Fatalf("skip while open moved time from %s to %s; currently-open must be a no-op", before.Format(time.RFC3339), c.now.Format(time.RFC3339))
	}
}

// --- M1C: snapshot isolation + concurrency -------------------------------------

func TestConcurrentAccessDoesNotCorruptState(t *testing.T) {
	c := NewClock()
	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func(n int) {
			defer wg.Done()
			for j := 0; j < 250; j++ {
				_ = c.Snapshot()            // concurrent reads
				c.SetPaused(false)          // concurrent writes
				_ = c.SetSpeed(1 + (j % 3)) // valid speeds only
				c.Advance(time.Second)      // concurrent advancement
			}
		}(i)
	}
	wg.Wait()

	snap := c.Snapshot()
	if snap.Speed < 1 || snap.Speed > 3 {
		t.Fatalf("speed = %d after concurrent access, want a valid speed in [1,3]", snap.Speed)
	}
	// Time must have advanced monotonically from the canonical start (all advances positive).
	if snap.GameDatetime <= "1980-02-01T09:00:00" {
		t.Fatalf("game time = %s did not advance past the start after concurrent Advance calls", snap.GameDatetime)
	}
}
