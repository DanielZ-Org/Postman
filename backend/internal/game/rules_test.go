package game

import "testing"

// TestLogisticsCapacityRounding pins the decided SPEC 16.6 rule: the logistics trait
// adds +10% rounded half away from zero to whole units, consistent with the SPEC 4.3
// percentage rule. Current bases yield foot 11, bicycle 22, car 55.
func TestLogisticsCapacityRounding(t *testing.T) {
	cases := []struct {
		mode       string
		base, want int
	}{
		{ModeFoot, 10, 11},
		{ModeBicycle, 20, 22},
		{ModeCar, 50, 55},
	}
	for _, c := range cases {
		if got := deliveryCapacityUnits(c.mode, false); got != c.base {
			t.Errorf("%s capacity = %d, want %d", c.mode, got, c.base)
		}
		if got := deliveryCapacityUnits(c.mode, true); got != c.want {
			t.Errorf("%s capacity with logistics = %d, want %d (+10%% rounded half away, SPEC 16.6)",
				c.mode, got, c.want)
		}
	}
}

// TestDeliveryCycleDurationsPerMode pins the decided SPEC 9.1/16.13 table and the
// SPEC 16.5 trait application: 1h packing for all modes; out-phase foot 180m,
// bicycle 90m, car 60m base, divided by the speed modifier (snail 4/5, chicken 1/1,
// cheetah 6/5) and rounded half away from zero. Bicycle + snail = 113 exercises the
// half-minute rounding.
func TestDeliveryCycleDurationsPerMode(t *testing.T) {
	cases := []struct {
		mode, trait string
		wantOut     int // out-phase minutes, trait-adjusted
		wantCycle   int // packing + out
	}{
		{ModeFoot, "snail", 225, 285},
		{ModeFoot, "chicken", 180, 240},
		{ModeFoot, "cheetah", 150, 210},
		{ModeBicycle, "snail", 113, 173}, // 90 / 0.8 = 112.5 -> 113 (half away)
		{ModeBicycle, "chicken", 90, 150},
		{ModeBicycle, "cheetah", 75, 135},
		{ModeCar, "snail", 75, 135},
		{ModeCar, "chicken", 60, 120},
		{ModeCar, "cheetah", 50, 110},
	}
	for _, c := range cases {
		if got := outPhaseMinutes(c.mode, c.trait); got != c.wantOut {
			t.Errorf("%s/%s out-phase = %d min, want %d (SPEC 9.1/16.5)", c.mode, c.trait, got, c.wantOut)
		}
		if got := cycleMinutes(c.mode, c.trait); got != c.wantCycle {
			t.Errorf("%s/%s cycle = %d min, want %d (SPEC 9.1)", c.mode, c.trait, got, c.wantCycle)
		}
	}
}

// TestSmallSizeProbabilityThreshold pins the decided SPEC 16.1 generation split:
// rolls 0..69 produce a small package and 70..99 a medium one (70/30).
func TestSmallSizeProbabilityThreshold(t *testing.T) {
	cases := []struct {
		roll int
		want string
	}{
		{0, "small"},
		{69, "small"},  // last small roll
		{70, "medium"}, // first medium roll
		{99, "medium"},
	}
	for _, c := range cases {
		s := newStateAt(t, "1980-02-01T09:00:00")
		selectSmall(t, s)
		withDeterministicRand(t, func(int) int { return c.roll })
		if !s.generatePackagesLocked(mustParseTime(t, "1980-02-01T09:30:00")) {
			t.Fatalf("roll %d: generation reported no change, want one package", c.roll)
		}
		if got := s.packages[0].Size; got != c.want {
			t.Errorf("roll %d -> size %q, want %q (SPEC 16.1: 70/30 small/medium)", c.roll, got, c.want)
		}
	}
}
