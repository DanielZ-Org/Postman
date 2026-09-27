package game

// Employee operational states (SPEC 7.4): ready -> packing -> out_for_delivery ->
// ready. Only the backend transitions these; a requested assignment is valid solely
// when every rule passes.
const (
	EmployeeReady          = "ready"
	EmployeePacking        = "packing"
	EmployeeOutForDelivery = "out_for_delivery"
)

// Employee delivery modes (SPEC 9) and mood defaults (SPEC 7.3/7.4).
const (
	ModeFoot    = "foot"
	ModeBicycle = "bicycle"
	ModeCar     = "car"

	MoodNeutral = "neutral"
)

// Employee skills (SPEC 7.2): an extensible list; vehicle modes are gated on the
// matching skill, foot delivery always works without one.
const (
	SkillBicycle        = "bicycle"
	SkillDrivingLicence = "driving_licence"
)

// Employee is one delivery worker. Wages accrue per delivered package and settle at
// the weekly payroll; PackagesDeliveredThisWeek and RunsToday reset on their own
// schedules (payroll day and calendar day respectively).
type Employee struct {
	ID                        string   `json:"id"`
	Name                      string   `json:"name"`
	SpeedTrait                string   `json:"speed_trait"`
	Skills                    []string `json:"skills"`
	Mood                      string   `json:"mood"`
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
// Speed-trait numeric modifiers are OPEN (SPEC 16.5), so the trait is stored but has
// no mechanical effect on delivery speed. Skills follow the deterministic hire-time
// distribution (SPEC 7.2, labelled cycle in rules.go) so vehicle modes are reachable
// without a training system.
func newEmployee(id string, hireNumber int) *Employee {
	// A non-nil empty slice so the wire contract serialises skills as [] (never null).
	skills := append([]string{}, hireSkillCycle[(hireNumber-1)%len(hireSkillCycle)]...)
	return &Employee{
		ID:                        id,
		Name:                      hireNames[(hireNumber-1)%len(hireNames)],
		SpeedTrait:                speedTraits[(hireNumber-1)%len(speedTraits)],
		Skills:                    skills,
		Mood:                      MoodNeutral,
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
