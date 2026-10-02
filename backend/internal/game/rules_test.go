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
