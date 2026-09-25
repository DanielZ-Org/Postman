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
// by 3 game hours out for delivery.
const (
	packingDuration    = time.Hour
	deliveryDuration   = 3 * time.Hour
	assignmentDuration = packingDuration + deliveryDuration
)

// Delivery capacities and limits for foot delivery (SPEC 9.2/9.3): 10 capacity units
// per run and 2 local runs per working day. Normal packages consume 1 unit, express 2.
const (
	footDeliveryCapacity = 10
	footRunsPerDay       = 2
)

// Logistics-trait capacity bonus: SPEC 3.1 gives +10% delivery capacity; SPEC 16.6
// leaves the rounding OPEN. The labelled temporary rule is floor(10 * 1.1) = 11.
const footDeliveryCapacityWithLogistics = 11

// Storage-trait capacity bonus: SPEC 3.1 gives +10% usable storage capacity. The
// catalogue capacities are exact multiples of 10, so the bonus needs no rounding rule.
const storageTraitBonusPercent = 10

// Wages (SPEC 10): foot delivery accrues a fixed wage per successfully delivered
// package; an express package counts as one package for wages.
const footWagePerPackage = 2

// Hiring (SPEC 8): the fee follows a historical high-water counter (+£50 per hire), not
// current headcount: hire #1 costs £50, hire #2 £100, and so on.
const (
	hireBaseFee = 50
	hireFeeStep = 50
)

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
const minOfficeDownPayment = 350 // cheapest new-office entry cost (small); game-over check

// randIntn is indirected so generation tests can substitute a deterministic source.
// Production uses the auto-seeded global source.
var randIntn = rand.Intn

// roundPounds rounds a fractional pound amount to the nearest integer pound, half away
// from zero. SPEC 4.3 keeps money as integer pounds for M1, while late-delivery
// penalties (SPEC 5.3) produce fractional amounts that must be settled somewhere.
func roundPounds(v int) int {
	if v < 0 {
		return -roundPounds(-v)
	}
	return (v + 50) / 100
}

// percentOf returns value * percent / 100, rounded half away from zero.
func percentOf(value, percent int) int {
	return roundPounds(value * percent)
}
