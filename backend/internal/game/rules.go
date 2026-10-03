package game

import (
	"math/rand"
	"time"
)

// This file collects the numeric game rules used by simulation and operations.
// SPEC 16 decisions (2026-09-29 board decision) are recorded in SPEC.md §16 and
// mirrored here; values still labelled temporary are outside §16 (e.g. per-mode
// delivery timing under SPEC 9.1).

// Generation timing (SPEC 6, decided 16.12): the canonical rate is 2 packages per
// open working hour, materialised as a fixed 30-minute cursor — deterministic,
// exactly two ticks per open working hour.
const generationInterval = 30 * time.Minute

// generationGuardLimit caps how many generation ticks a single catch-up pass may
// process (200 game hours), so a pathological clock jump cannot spin the ticker.
const generationGuardLimit = 400

// smallSizeProbability is the percent chance that a generated package is small rather
// than medium (SPEC 16.1, decided 2026-09-29): 70/30, matching the reference mock.
const smallSizeProbability = 70

// expressChancePercent is the post-first-month chance that a generated package is
// express (SPEC 6: 0% during the first four weeks, 3% afterwards).
const expressChancePercent = 3

// expressFreeDuration is the first four weeks of game time during which no express
// packages generate (SPEC 6).
const expressFreeDuration = 28 * 24 * time.Hour

// Delivery-run geometry (SPEC 9.1, decided 16.13): every mode packs for 1 game hour,
// then is out for delivery for a mode-specific base duration — foot 3h (180m),
// bicycle 1.5h (90m), car 1h (60m) — which matches the runs-per-day budgets of
// SPEC 9.3 (foot 2×4h, bicycle 3×2.5h, car 2×2h within the weekday).
const packingDuration = time.Hour

// outPhaseBaseMinutes returns the mode's base out-for-delivery duration in whole game
// minutes (SPEC 9.1 table).
func outPhaseBaseMinutes(mode string) int {
	switch mode {
	case ModeBicycle:
		return 90
	case ModeCar:
		return 60
	default: // foot
		return 180
	}
}

// speedTraitRatio returns the speed-trait multiplier as an exact rational num/den
// (SPEC 16.5): snail 0.8 = 4/5, chicken 1.0, cheetah 1.2 = 6/5. Integer arithmetic
// keeps durations exact — no floating point anywhere in the timing path.
func speedTraitRatio(trait string) (num, den int) {
	switch trait {
	case "snail":
		return 4, 5
	case "cheetah":
		return 6, 5
	default: // chicken or unknown
		return 1, 1
	}
}

// roundHalfAwayDiv rounds n/d half away from zero with integer math, matching the
// SPEC 4.3 percentage rule.
func roundHalfAwayDiv(n, d int) int {
	if d < 0 {
		n, d = -n, -d
	}
	if n >= 0 {
		return (n + d/2) / d
	}
	return (n - d/2) / d
}

// outPhaseMinutes returns the trait-adjusted out-for-delivery duration in whole game
// minutes: base ÷ speed modifier, rounded half away from zero (SPEC 9.1/16.5). Snail
// lengthens the phase (bicycle 90 → 113), Cheetah shortens it (bicycle 90 → 75).
func outPhaseMinutes(mode, trait string) int {
	num, den := speedTraitRatio(trait)
	return roundHalfAwayDiv(outPhaseBaseMinutes(mode)*den, num)
}

// cycleMinutes is packing plus the trait-adjusted out phase (SPEC 9.1).
func cycleMinutes(mode, trait string) int {
	return 60 + outPhaseMinutes(mode, trait)
}

// cycleDuration is the full cycle length used by the closing-time validation when a
// batch is assigned (SPEC 9.1).
func cycleDuration(mode, trait string) time.Duration {
	return time.Duration(cycleMinutes(mode, trait)) * time.Minute
}

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

// Logistics-trait capacity bonus (SPEC 3.1, decided 16.6): +10% rounded half away
// from zero to whole units, consistent with the SPEC 4.3 percentage rule. Current
// bases: foot 10 -> 11, bicycle 20 -> 22, car 50 -> 55.
const (
	footDeliveryCapacityWithLogistics    = 11 // +10% rounded half away (SPEC 16.6)
	bicycleDeliveryCapacityWithLogistics = 22 // +10% rounded half away (SPEC 16.6)
	carDeliveryCapacityWithLogistics     = 55 // +10% rounded half away (SPEC 16.6)
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
// (SPEC 9.2): foot 10, bicycle 20, car 50. The logistics trait adds +10% rounded half
// away from zero (SPEC 3.1, decided SPEC 16.6).
func deliveryCapacityUnits(mode string, hasLogistics bool) int {
	switch mode {
	case ModeCar:
		if hasLogistics {
			return carDeliveryCapacityWithLogistics // +10%, round half away (SPEC 16.6)
		}
		return carDeliveryCapacity
	case ModeBicycle:
		if hasLogistics {
			return bicycleDeliveryCapacityWithLogistics // +10%, round half away (SPEC 16.6)
		}
		return bicycleDeliveryCapacity
	default: // foot
		if hasLogistics {
			return footDeliveryCapacityWithLogistics // +10%, round half away (SPEC 16.6)
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

// Vehicle purchase prices (SPEC 12, decided 16.7): £50 per bicycle, £500 per car,
// integer pence. Running costs (decided 16.7, M3): fuel 150p per completed car run;
// maintenance every Friday per owned vehicle, 100p per bicycle and 500p per car.
const (
	bicyclePurchasePrice = 5000  // £50
	carPurchasePrice     = 50000 // £500

	carFuelPerRun            = 150 // £1.50 per completed car run; bicycle and foot cost no fuel
	bicycleMaintenanceWeekly = 100 // £1 per owned bicycle, charged every Friday
	carMaintenanceWeekly     = 500 // £5 per owned car, charged every Friday
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

// Employee mood and retention (SPEC 10, decided 16.15): mood runs 0-100 starting at
// 100. Tuesday payroll moves every employee's mood: +5 when the payroll left cash
// non-negative, -25 when it left cash negative (floored at 0; +5 capped at 100).
// After the update each ready employee at or below the quit threshold rolls a 20%
// chance to leave; mid-run employees are skipped and re-checked next payroll.
const (
	moodInitial           = 100
	moodMax               = 100
	moodPayrollUp         = 5
	moodPayrollDown       = 25
	moodQuitThreshold     = 30
	moodQuitChancePercent = 20
)

// clampMood keeps a mood movement inside the 0..moodMax range.
func clampMood(mood int) int {
	if mood < 0 {
		return 0
	}
	if mood > moodMax {
		return moodMax
	}
	return mood
}

// Rent misses (SPEC 4.2, decided 16.10): the first missed payment adds a one-off 20%
// late fee deducted regardless of cash (cash may go negative); the fee does not
// escalate further, and the second miss terminates the contract.
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
