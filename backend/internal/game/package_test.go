package game

import "testing"

// TestBaseFeeForDecidedPrices pins the SPEC 5.3 price table including the decided
// large prices (SPEC 16.2): large must never fall through to the medium default.
func TestBaseFeeForDecidedPrices(t *testing.T) {
	cases := []struct {
		size, service string
		want          int
	}{
		{"small", ServiceNormal, 500},
		{"small", ServiceExpress, 1200},
		{"medium", ServiceNormal, 700},
		{"medium", ServiceExpress, 1500},
		{"large", ServiceNormal, 1000},  // SPEC 16.2: £10
		{"large", ServiceExpress, 2000}, // SPEC 16.2: £20
	}
	for _, c := range cases {
		if got := baseFeeFor(c.size, c.service); got != c.want {
			t.Errorf("baseFeeFor(%s, %s) = %d, want %dp", c.size, c.service, got, c.want)
		}
	}
}
