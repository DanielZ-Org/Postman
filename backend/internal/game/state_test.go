package game

import "testing"

func TestInitialGameTimeMatchesSpec(t *testing.T) {
	const want = "1980-02-01T09:00:00"
	if InitialGameTime != want {
		t.Fatalf("InitialGameTime = %q, want %q", InitialGameTime, want)
	}
}

func TestNewClockUsesCanonicalStartTime(t *testing.T) {
	got := NewClock().Snapshot()
	if got.GameDatetime != "1980-02-01T09:00:00" {
		t.Fatalf("GameDatetime = %q, want %q", got.GameDatetime, "1980-02-01T09:00:00")
	}
}

func TestNewClockInitialSpeedAndPausedMatchSpec(t *testing.T) {
	snap := NewClock().Snapshot()
	if snap.Speed != 1 {
		t.Fatalf("initial Speed = %d, want 1", snap.Speed)
	}
	if snap.Paused {
		t.Fatal("initial Paused = true, want false")
	}
}

func TestNewClockDerivedFieldsMatchSpecExample(t *testing.T) {
	snap := NewClock().Snapshot()
	if snap.DayOfWeek != "friday" {
		t.Fatalf("DayOfWeek = %q, want %q", snap.DayOfWeek, "friday")
	}
	if !snap.OfficeOpen {
		t.Fatal("OfficeOpen = false, want true (Friday 09:00 is within working hours)")
	}
	if snap.DaysUntilNextPayroll != 4 {
		t.Fatalf("DaysUntilNextPayroll = %d, want 4", snap.DaysUntilNextPayroll)
	}
	if snap.DaysUntilNextRent != 0 {
		t.Fatalf("DaysUntilNextRent = %d, want 0 (rent is due on Fridays)", snap.DaysUntilNextRent)
	}
}

func TestReadingClockDoesNotMutateState(t *testing.T) {
	c := NewClock()
	first := c.Snapshot()
	for i := 0; i < 3; i++ {
		if got := c.Snapshot(); got != first {
			t.Fatalf("Snapshot changed on repeated read: %+v vs %+v", got, first)
		}
	}
}

func TestReturnedSnapshotCannotMutateAuthoritativeState(t *testing.T) {
	c := NewClock()
	mutated := c.Snapshot()
	mutated.Speed = 99
	mutated.Paused = true
	mutated.GameDatetime = "1999-01-01T00:00:00"

	// The authoritative clock must be unaffected by mutating a returned copy.
	if got := c.Snapshot(); got.Speed != 1 || got.Paused || got.GameDatetime != InitialGameTime {
		t.Fatalf("authoritative state was mutated through a snapshot: %+v", got)
	}
}

func TestNewInitialStateSeedsCanonicalClock(t *testing.T) {
	got := NewInitialState()
	if got.Clock.Snapshot().GameDatetime != InitialGameTime {
		t.Fatalf("initial state clock = %q, want %q", got.Clock.Snapshot().GameDatetime, InitialGameTime)
	}
}
