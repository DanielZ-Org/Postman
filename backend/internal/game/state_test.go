package game

import "testing"

func TestNewInitialStateUsesCanonicalStartTime(t *testing.T) {
	got := NewInitialState()
	if got.StartTime != InitialGameTime {
		t.Fatalf("StartTime = %q, want %q", got.StartTime, InitialGameTime)
	}
}

func TestInitialGameTimeMatchesSpec(t *testing.T) {
	const want = "1980-02-01T09:00:00"
	if InitialGameTime != want {
		t.Fatalf("InitialGameTime = %q, want %q", InitialGameTime, want)
	}
}
