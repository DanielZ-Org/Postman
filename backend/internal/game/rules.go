package game

import (
	"math/rand"
	"time"
)

// This file collects the numeric game rules used by simulation and operations. SPEC 16
// marks several of these as OPEN or exact behaviour as undecided; each such value is
// labelled so temporary assumptions stay visible and isolated in one place.

// Generation timing: SPEC 6 defines the canonical rate as 2 packages per open working
// hour (one per 30 game minutes). The exact tick-boundary semantics are OPEN (SPEC
// 16.12), so a fixed 30-minute cursor is used as the labelled temporary implementation.
const generationInterval = 30 * time.Minute

// generationGuardLimit caps how many generation ticks a single catch-up pass may
// process (200 game hours), so a pathological clock jump cannot spin the ticker.
const generationGuardLimit = 400

// smallSizeProbability is the percent chance that a generated package is small rather
// than medium. SPEC 16.1 leaves the small/medium distribution OPEN; 70/30 is the
// labelled temporary assumption (mirrors the reference frontend mock).
const smallSizeProbability = 70

// expressChancePercent is the post-first-month chance that a generated package is
// express (SPEC 6: 0% during the first four weeks, 3% afterwards).
const expressChancePercent = 3

// expressFreeDuration is the first four weeks of game time during which no express
// packages generate (SPEC 6).
const expressFreeDuration = 28 * 24 * time.Hour

// Delivery-run geometry (SPEC 9.1): a walking cycle is 1 game hour of packing followed
// by 3 game hours out for delivery. Bicycle and car reuse this same geometry as a
// labelled temporary assumption until SPEC defines per-mode timing (SPEC 9.1: bicycle
// and car timing "can be defined separately later without changing this model").
const (
	packingDuration    = time.Hour
	deliveryDuration   = 3 * time.Hour
	assignmentDuration = packingDuration + deliveryDuration
)

// Delivery capacities per mode (SPEC 9.2): foot 10, bicycle 20, car 50 units per run.
// Normal packages consume 1 unit, express 2 — the same consumption in every mode.
const (
	footDeliveryCapacity    = 10
	bicycleDeliveryCapacity = 20 // SPEC 9.2: bicycle capacity per run (M2-3)
	carDeliveryCapacity     = 50 // SPEC 9.2: car capacity per run (M2-4)
)

// Local runs per working day per mode (SPEC 9.3): foot and car 2, bicycle 3. Far runs
// stay deferred (SPEC 9.3/16.11), so every budget is local-only.
const (
	footRunsPerDay    = 2
	bicycleRunsPerDay = 3 // SPEC 9.3: three local bicycle runs per working day (M2-3)
	carRunsPerDay     = 2 // SPEC 9.3: two local car runs per working day (M2-4)
)

// Logistics-trait capacity bonus: SPEC 3.1 gives +10% delivery capacity; SPEC 16.6
// leaves the rounding OPEN. The labelled temporary rule is floor(base * 1.1), applied
// per mode: foot floor(10 * 1.1) = 11, bicycle floor(20 * 1.1) = 22,
// car floor(50 * 1.1) = 55.
const (
	footDeliveryCapacityWithLogistics    = 11
	bicycleDeliveryCapacityWithLogistics = 22 // floor(20 * 1.1), SPEC 3.1/16.6 labelled
	carDeliveryCapacityWithLogistics     = 55
)

// Storage-trait capacity bonus: SPEC 3.1 gives +10% usable storage capacity. The
// catalogue capacities are exact multiples of 10, so the bonus needs no rounding rule.
const storageTraitBonusPercent = 10

// Wages per delivered package by mode (SPEC 10), integer pence: foot £2.00, bicycle
// £3.00, car exactly £2.50 (board decision — integer pence makes it exact). An express
// package counts as one package for wages despite consuming two capacity units.
// Monetary values are integer pence (board decision, M2 money-unit migration).
const (
	footWagePerPackage    = 200
	bicycleWagePerPackage = 300 // SPEC 10: £3.00 per delivered package (M2-3)
	carWagePerPackage     = 250 // board decision: exactly £2.50 per delivered package (M2-4)
)

// deliveryCapacityUnits returns the usable capacity units for one run in the given mode
// (SPEC 9.2): foot 10, bicycle 20, car 50. The logistics trait adds +10% with floor
// rounding (SPEC 3.1, SPEC 16.6 OPEN — labelled temporary rule).
func deliveryCapacityUnits(mode string, hasLogistics bool) int {
	switch mode {
	case ModeCar:
		if hasLogistics {
			return carDeliveryCapacityWithLogistics // +10%, floor (SPEC 3.1/16.6 labelled)
		}
		return carDeliveryCapacity
	case ModeBicycle:
		if hasLogistics {
			return bicycleDeliveryCapacityWithLogistics // +10%, floor (SPEC 3.1/16.6 labelled)
		}
		return bicycleDeliveryCapacity
	default: // foot
		if hasLogistics {
			return footDeliveryCapacityWithLogistics // +10%, floor (SPEC 16.6 labelled)
		}
		return footDeliveryCapacity
	}
}

// localRunsPerDay returns the local runs allowed per working day in the given mode
// (SPEC 9.3): foot and car 2, bicycle 3. Far runs stay deferred (SPEC 9.3/16.11), so
// the budget is local-only.
func localRunsPerDay(mode string) int {
	switch mode {
	case ModeCar:
		return carRunsPerDay
	case ModeBicycle:
		return bicycleRunsPerDay
	default: // foot
		return footRunsPerDay
	}
}

// wagePerPackageFor returns the wage accrued per delivered package in integer pence for
// one mode (SPEC 10): foot £2.00, bicycle £3.00, car exactly £2.50 (board decision).
func wagePerPackageFor(mode string) int {
	switch mode {
	case ModeCar:
		return carWagePerPackage
	case ModeBicycle:
		return bicycleWagePerPackage
	default: // foot
		return footWagePerPackage
	}
}

// Hiring (SPEC 8): the fee follows a historical high-water counter (+£50 per hire), not
// current headcount: hire #1 costs £50, hire #2 £100, and so on. Values are integer pence.
const (
	hireBaseFee = 5000
	hireFeeStep = 5000
)

// Vehicle purchase prices (SPEC 12/16.7 OPEN): labelled placeholder values in pence,
// isolated here so a later board decision changes one line.
const (
	bicyclePurchasePrice = 5000  // £50 placeholder
	carPurchasePrice     = 50000 // £500 placeholder
)

// Hire-time skill distribution (SPEC 7.2): the player may hire employees who already
// possess skills and training is DEFERRED, so a deterministic cycle makes vehicle modes
// reachable without a training system. Labelled temporary assumption: none -> bicycle
// -> bicycle + driving licence, repeating.
var hireSkillCycle = [][]string{
	{},
	{SkillBicycle},
	{SkillBicycle, SkillDrivingLicence},
}

// Finance schedules (SPEC 11): payroll settles every Tuesday, rent every Friday after
// the prepaid weeks, and loan interest every four game weeks (5% of principal).
const (
	payrollWeekday        = time.Tuesday
	payrollHour           = 9 // SPEC does not pin the hour; Tuesday 09:00 is the labelled choice
	intervalWeeks         = 4
	interestPercent       = 5
	rentLateFeePercent    = 20
	maxMissedRentPayments = 2
	loanReferenceID       = "loan-1"
)

// Rent misses (SPEC 4.2): the first missed payment adds a one-off 20% late fee; the
// second terminates the contract. Whether unpaid late fees themselves escalate is OPEN
// (SPEC 16.8); the labelled temporary rule deducts the fee regardless of cash.
const minOfficeDownPayment = 35000 // cheapest new-office entry cost (small), pence; game-over check

// randIntn is indirected so generation tests can substitute a deterministic source.
// Production uses the auto-seeded global source.
var randIntn = rand.Intn

// All monetary state and constants are integer pence (board decision recorded in SPEC 4.3,
// M2 money-unit migration): no floating point anywhere in money paths, and percentage
// results round half away from zero to whole pence.

// roundToWholePence rounds value/100 to the nearest whole number of pence, half away from
// zero. It is the single rounding rule for every percentage-derived monetary amount (late-
// delivery penalties SPEC 5.3, trait bonus SPEC 3.1, loan interest SPEC 11.1).
func roundToWholePence(v int) int {
	if v < 0 {
		return -roundToWholePence(-v)
	}
	return (v + 50) / 100
}

// percentOf returns value * percent / 100 in pence, rounded half away from zero to whole
// pence.
func percentOf(value, percent int) int {
	return roundToWholePence(value * percent)
}
