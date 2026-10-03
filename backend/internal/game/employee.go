package game

import "encoding/json"

// Employee operational states (SPEC 7.4): ready -> packing -> out_for_delivery ->
// ready. Only the backend transitions these; a requested assignment is valid solely
// when every rule passes.
const (
	EmployeeReady          = "ready"
	EmployeePacking        = "packing"
	EmployeeOutForDelivery = "out_for_delivery"
)

// Employee delivery modes (SPEC 9).
const (
	ModeFoot    = "foot"
	ModeBicycle = "bicycle"
	ModeCar     = "car"
)

// Employee skills (SPEC 7.2): an extensible list; vehicle modes are gated on the
// matching skill, foot delivery always works without one.
const (
	SkillBicycle        = "bicycle"
	SkillDrivingLicence = "driving_licence"
)

// Employee is one delivery worker. Wages accrue per delivered package and settle at
// the weekly payroll; PackagesDeliveredThisWeek and RunsToday reset on their own
// schedules (payroll day and calendar day respectively). Mood is the 0-100 retention
// value (SPEC 10, decided SPEC 16.15): payroll moves it, and a ready employee at or
// below the quit threshold may leave.
type Employee struct {
	ID                        string   `json:"id"`
	Name                      string   `json:"name"`
	SpeedTrait                string   `json:"speed_trait"`
	Skills                    []string `json:"skills"`
	Mood                      int      `json:"mood"`
	CurrentDeliveryMode       string   `json:"current_delivery_mode"`
	PackagesDeliveredThisWeek int      `json:"packages_delivered_this_week"`
	AccruedWages              int      `json:"accrued_wages"`
	Status                    string   `json:"status"`

	// RunsToday counts local delivery runs started for RunsTodayKey (the game date);
	// the mode-specific daily budget (foot and car: two per working day, SPEC 9.3)
	// is enforced against it.
	RunsToday    int    `json:"runs_today"`
	RunsTodayKey string `json:"runs_today_key"`
}

// hireNames is the deterministic name pool used for new hires; the mock frontend and
// SPEC 7 example use the same pool style (names derive from the speed-trait cycle).
var hireNames = []string{
	"Bob Snail",
	"Sally Snail",
	"Chuck Chicken",
	"Rita Chicken",
	"Chester Cheetah",
	"Cleo Cheetah",
	"Marty Snail",
	"Clara Chicken",
	"Sam Cheetah",
}

// speedTraits cycles through the SPEC 7.1 archetype labels in hire order.
var speedTraits = []string{"snail", "chicken", "cheetah"}

// newEmployee builds a ready-to-work employee for the given 1-based hire number.
// Speed-trait modifiers are decided (SPEC 16.5): Snail 0.8x, Chicken 1.0x, Cheetah
// 1.2x — applied as speed multipliers on the out-for-delivery phase (SPEC 9.1).
// Skills follow the deterministic hire-time distribution (SPEC 7.2, labelled
// cycle in rules.go) so vehicle modes are reachable without a training system.
func newEmployee(id string, hireNumber int) *Employee {
	// A non-nil empty slice so the wire contract serialises skills as [] (never null).
	skills := append([]string{}, hireSkillCycle[(hireNumber-1)%len(hireSkillCycle)]...)
	return &Employee{
		ID:                        id,
		Name:                      hireNames[(hireNumber-1)%len(hireNames)],
		SpeedTrait:                speedTraits[(hireNumber-1)%len(speedTraits)],
		Skills:                    skills,
		Mood:                      moodInitial,
		CurrentDeliveryMode:       ModeFoot,
		PackagesDeliveredThisWeek: 0,
		AccruedWages:              0,
		Status:                    EmployeeReady,
		RunsToday:                 0,
		RunsTodayKey:              "",
	}
}

// hiringState is the read-only hiring view (SPEC 8): current headcount, the lifetime
// hire counter that drives the fee, and the next fee.
type hiringState struct {
	CurrentEmployeeCount int
	TotalHiresLifetime   int
	NextHiringFee        int
}

// nextHiringFee returns the fee for the next hire from the historical high-water
// counter (SPEC 8): hire #n costs n * £50 regardless of current headcount.
func nextHiringFee(totalHires int) int {
	return hireBaseFee + totalHires*hireFeeStep
}

// UnmarshalJSON keeps old saves loadable after mood became a number (SPEC 10): earlier
// versions persisted a fixed label ("neutral" - the only value ever written), so a
// legacy string decodes to the initial mood while numeric values parse as-is and a
// missing field defaults to the initial mood. The next save writes the number back.
func (e *Employee) UnmarshalJSON(data []byte) error {
	type employeeAlias Employee
	aux := struct {
		Mood json.RawMessage `json:"mood"`
		*employeeAlias
	}{employeeAlias: (*employeeAlias)(e)}
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}
	e.Mood = moodInitial
	if len(aux.Mood) == 0 || string(aux.Mood) == "null" {
		return nil
	}
	var numeric int
	if err := json.Unmarshal(aux.Mood, &numeric); err == nil {
		e.Mood = numeric
		return nil
	}
	// Legacy label (e.g. "neutral"): keep the initial mood. A malformed value fails.
	var label string
	if err := json.Unmarshal(aux.Mood, &label); err != nil {
		return err
	}
	return nil
}
