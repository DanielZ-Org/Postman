package game

import "time"

// Delivery-run phases (SPEC 9.1): packing (1 game hour) then out_for_delivery
// (3 game hours), after which the employee returns to ready.
const (
	RunPhasePacking        = "packing"
	RunPhaseOutForDelivery = "out_for_delivery"
)

// Run is one in-flight delivery cycle for an employee. PhaseEnd is the authoritative
// game instant the current phase finishes; completions are processed against that
// instant so skipped intervals still deliver at the correct game time (SPEC 14.4).
type Run struct {
	EmployeeID string    `json:"employee_id"`
	PackageIDs []string  `json:"package_ids"`
	Phase      string    `json:"phase"`
	PhaseEnd   time.Time `json:"-"`
}

// Tick advances the clock by real elapsed time and then processes every game event
// that became due up to the new now. It is the single entry point used by the
// background simulation loop. Lock order: GameState.mu, then the Clock lock (via
// Advance/Now); never the reverse. Returns true when any state changed (used to
// decide whether a persistence snapshot is needed).
func (s *GameState) Tick(elapsed time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Clock.Advance(elapsed)
	return s.processLocked(s.Clock.Now())
}

// SkipToNextOpening advances the clock to the earliest upcoming opening (SPEC 2.4)
// and then processes the jumped-over interval through normal simulation semantics,
// as required by SPEC 14.4 (skipped time must not bypass game effects).
func (s *GameState) SkipToNextOpening() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Clock.SkipToNextOpening()
	s.processLocked(s.Clock.Now())
}

// processLocked runs every due event up to now. Callers must hold s.mu. Events are
// timestamp-driven (run phase ends, generation cursor, payroll/interest schedules,
// rent due date), so a single call safely catches up over an arbitrarily long gap.
func (s *GameState) processLocked(now time.Time) bool {
	if s.status == GameStatusGameOver {
		return false // the game has ended: all scheduled events are frozen (labelled)
	}
	changed := false
	changed = s.rolloverDayLocked(now) || changed
	changed = s.processRunsLocked(now) || changed
	changed = s.processPayrollLocked(now) || changed
	changed = s.processRentLocked(now) || changed
	changed = s.processInterestLocked(now) || changed
	changed = s.generatePackagesLocked(now) || changed
	changed = s.checkGameOverLocked(now) || changed
	return changed
}

// rolloverDayLocked detects a game-calendar day change: it settles the previous
// day's financial-trait revenue bonus (SPEC 3.1 applies +10% at end-of-day
// settlement), resets daily counters, and resets each employee's runs-per-day budget
// (SPEC 9.3).
func (s *GameState) rolloverDayLocked(now time.Time) bool {
	key := dayKey(now)
	if key == s.deliveredTodayKey && key == s.dailyRevenueKey {
		return false
	}
	changed := false

	// Settle the previous day's aggregate revenue bonus before clearing it (SPEC 3.1).
	if s.dailyRevenue > 0 && s.player.Trait == "financial" {
		bonus := percentOf(s.dailyRevenue, 10)
		if bonus > 0 {
			s.postTransactionLocked(now, CategoryTraitBonus, bonus, "Financial trait end-of-day revenue bonus", "trait-financial")
			changed = true
		}
	}
	if s.deliveredTodayKey != key {
		s.deliveredToday = 0
		s.deliveredTodayKey = key
		changed = true
	}
	if s.dailyRevenueKey != key {
		s.dailyRevenue = 0
		s.dailyRevenueKey = key
		changed = true
	}
	for _, e := range s.employees {
		if e.RunsTodayKey != key {
			e.RunsToday = 0
			e.RunsTodayKey = key
			changed = true
		}
	}
	return changed
}

// processRunsLocked advances in-flight delivery runs. Each run that reached its
// phase end either transitions packing -> out_for_delivery or completes: packages
// deliver, revenue posts, wages accrue and the employee returns to ready. A long gap
// is processed phase by phase at the phase-end instants, so catch-up never skips a
// phase. Employee operational status is re-synced to its active run afterwards.
func (s *GameState) processRunsLocked(now time.Time) bool {
	changed := false
	kept := s.runs[:0]
	for _, run := range s.runs {
		finished := false
		// A skip can jump past both phases; drain them in order.
		for !now.Before(run.PhaseEnd) {
			switch run.Phase {
			case RunPhasePacking:
				run.Phase = RunPhaseOutForDelivery
				run.PhaseEnd = run.PhaseEnd.Add(deliveryDuration)
				for _, id := range run.PackageIDs {
					if p := s.findPackageLocked(id); p != nil && p.Status == PackageAssigned {
						p.Status = PackageOutForDelivery
					}
				}
				changed = true
			case RunPhaseOutForDelivery:
				s.completeRunLocked(run, run.PhaseEnd)
				changed = true
				finished = true
			default:
				finished = true // defensive: unknown phase never blocks the loop
			}
			if finished {
				break
			}
		}
		if !finished {
			kept = append(kept, run)
		}
	}
	s.runs = kept
	if len(s.runs) != len(kept) {
		changed = true
	}

	// Sync operational status: running a phase shows that phase; otherwise ready.
	for _, e := range s.employees {
		want := EmployeeReady
		for _, run := range s.runs {
			if run.EmployeeID == e.ID {
				want = run.Phase
				break
			}
		}
		if e.Status != want {
			e.Status = want
			changed = true
		}
	}
	return changed
}

// completeRunLocked settles one finished delivery cycle at time at: each package
// pays its base fee (reduced 25% normal / 75% express when late, SPEC 5.3), revenue
// posts as one package_revenue transaction, wages accrue per package (SPEC 10: £2
// foot, express counts once), and daily counters update. The financial-trait bonus
// is NOT applied here — it settles at the day rollover (SPEC 3.1).
func (s *GameState) completeRunLocked(run *Run, at time.Time) {
	var revenue, delivered int
	for _, id := range run.PackageIDs {
		p := s.findPackageLocked(id)
		if p == nil || p.Status == PackageDelivered {
			continue
		}
		late := false
		if due, err := time.Parse(GameTimeFormat, p.DueAt); err == nil && at.After(due) {
			late = true
		}
		factor := 100
		if late {
			if p.ServiceType == ServiceExpress {
				factor = 25 // 75% reduction (SPEC 5.3)
			} else {
				factor = 75 // 25% reduction
			}
		}
		paid := roundPounds(p.BaseFee * factor)
		p.Status = PackageDelivered
		p.DeliveredAt = strPtr(at.Format(GameTimeFormat))
		p.FinalRevenue = intPtr(paid)
		revenue += paid
		delivered++
	}
	if revenue > 0 {
		s.postTransactionLocked(at, CategoryPackageRevenue, revenue, itoa(delivered)+" package(s) delivered", run.EmployeeID)
		s.dailyRevenue += revenue
	}
	if emp := s.findEmployeeLocked(run.EmployeeID); emp != nil {
		if delivered > 0 {
			emp.PackagesDeliveredThisWeek += delivered
			emp.AccruedWages += delivered * footWagePerPackage
		}
		emp.Status = EmployeeReady
	}
	if dayKey(at) == s.deliveredTodayKey {
		s.deliveredToday += delivered
	}
}

// processPayrollLocked settles due Tuesday payrolls (SPEC 10/11.2): every accrued
// wage posts as one employee_wages transaction, accrued wages clear and the weekly
// per-employee delivery counters reset. Missed-payroll consequences are OPEN
// (SPEC 16.8); the labelled rule is a plain settlement whenever cash allows the
// deduction (cash may go negative — SPEC does not forbid it).
func (s *GameState) processPayrollLocked(now time.Time) bool {
	changed := false
	for !now.Before(s.payrollDue) {
		var total int
		for _, e := range s.employees {
			total += e.AccruedWages
		}
		if total > 0 {
			s.postTransactionLocked(s.payrollDue, CategoryEmployeeWages, -total, "Tuesday payroll", "")
		}
		for _, e := range s.employees {
			e.AccruedWages = 0
			e.PackagesDeliveredThisWeek = 0
		}
		s.payrollDue = s.payrollDue.AddDate(0, 0, 7)
		changed = true
	}
	return changed
}

// processRentLocked charges due weekly rent (SPEC 4.1/4.2/11.2). The first due
// instant is the prepaid period's end (midnight of the fourth Friday). A payment
// succeeds when cash covers the rent; otherwise the miss counter escalates: first
// miss adds a one-off 20% late fee, second miss terminates the contract. Events
// process at the due instant even when the office is closed (calendar events are not
// gated by opening hours).
func (s *GameState) processRentLocked(now time.Time) bool {
	office := s.selectedOffice
	if office == nil || office.ContractStatus != ContractActive {
		return false
	}
	due, err := time.Parse(GameTimeFormat, office.NextRentDue)
	if err != nil {
		return false // defensive: NextRentDue is always written in canonical format
	}
	changed := false
	for office.ContractStatus == ContractActive && !now.Before(due) {
		if s.cash >= office.WeeklyRent {
			s.postTransactionLocked(due, CategoryRent, -office.WeeklyRent, office.Type+" office weekly rent", office.ID)
			office.MissedRentPayments = 0
		} else {
			office.MissedRentPayments++
			if office.MissedRentPayments >= maxMissedRentPayments {
				office.ContractStatus = ContractTerminated
				changed = true
				break
			}
			fee := percentOf(office.WeeklyRent, rentLateFeePercent)
			s.postTransactionLocked(due, CategoryRentLateFee, -fee, "Missed rent - "+itoa(rentLateFeePercent)+"% late fee", office.ID)
		}
		next, err := time.Parse(GameTimeFormat, office.NextRentDue)
		if err != nil {
			break
		}
		next = next.AddDate(0, 0, 7)
		office.NextRentDue = next.Format(GameTimeFormat)
		due = next
		changed = true
	}
	return changed
}

// processInterestLocked charges due four-week loan interest (SPEC 11.1: 5% of
// principal; principal does not amortise — repayment is OPEN, SPEC 16.9).
func (s *GameState) processInterestLocked(now time.Time) bool {
	changed := false
	for !now.Before(s.interestDue) {
		interest := percentOf(s.player.LoanPrincipal, interestPercent)
		if interest > 0 {
			s.postTransactionLocked(s.interestDue, CategoryLoanInterest, -interest, "Four-week loan interest", loanReferenceID)
		}
		s.interestDue = s.interestDue.AddDate(0, 0, intervalWeeks*7)
		changed = true
	}
	return changed
}

// generatePackagesLocked advances the 30-minute generation cursor up to now and
// spawns one package per open-hours tick (SPEC 6: 2 packages per open working hour,
// only while open, only with an active contract). The cursor ticks through closed
// periods without spawning, so no packages appear outside opening hours and no gap
// is missed when the clock jumps. Express appears only after the first four weeks
// (3% chance). Storage-full behaviour is OPEN (SPEC 16.3); the labelled rule is to
// skip generation rather than overfill.
func (s *GameState) generatePackagesLocked(now time.Time) bool {
	changed := false
	guard := 0
	for {
		next := s.lastGeneration.Add(generationInterval)
		if next.After(now) {
			break
		}
		s.lastGeneration = next
		guard++
		if guard > generationGuardLimit {
			break
		}
		office := s.selectedOffice
		if office == nil || office.ContractStatus != ContractActive || !officeOpen(next) {
			continue
		}
		if !s.hasStorageForLocked(office, 1) {
			continue // labelled: skip generation when storage is full (SPEC 16.3 OPEN)
		}
		size := "medium"
		if randIntn(100) < smallSizeProbability {
			size = "small"
		}
		service := ServiceNormal
		if !next.Before(startTime.Add(expressFreeDuration)) && randIntn(100) < expressChancePercent {
			service = ServiceExpress
		}
		s.nextPkgSeq++
		s.packages = append(s.packages, newPackage(packageID(s.nextPkgSeq), size, service, next))
		changed = true
	}
	return changed
}

// checkGameOverLocked ends the game (SPEC 4.2): if the contract is not active and
// the player cannot afford the cheapest new office contract, the game ends — status
// becomes game_over, all events freeze and the clock pauses.
func (s *GameState) checkGameOverLocked(now time.Time) bool {
	if s.status != GameStatusRunning {
		return false
	}
	office := s.selectedOffice
	if office == nil || office.ContractStatus == ContractActive {
		return false
	}
	if s.cash >= minOfficeDownPayment {
		return false // the player can still afford a new office; the game continues
	}
	s.status = GameStatusGameOver
	s.Clock.SetPaused(true)
	return true
}

// findPackageLocked returns the package with the given id, or nil.
func (s *GameState) findPackageLocked(id string) *Package {
	for _, p := range s.packages {
		if p.ID == id {
			return p
		}
	}
	return nil
}

// findEmployeeLocked returns the employee with the given id, or nil.
func (s *GameState) findEmployeeLocked(id string) *Employee {
	for _, e := range s.employees {
		if e.ID == id {
			return e
		}
	}
	return nil
}

// storageUsedLocked sums storage units of every package not yet delivered
// (stored, assigned and out_for_delivery all occupy storage — SPEC 5.1).
func (s *GameState) storageUsedLocked() int {
	var used int
	for _, p := range s.packages {
		if p.Status != PackageDelivered {
			used += p.StorageUnits
		}
	}
	return used
}

// usableStorageLocked returns the office's usable capacity, including the player's
// storage trait +10% bonus (SPEC 3.1). Labelled: floor semantics need no rounding
// because catalogue capacities are multiples of 10.
func (s *GameState) usableStorageLocked(office *RuntimeOffice) int {
	capacity := office.Storage.Current
	if s.player.Trait == "storage" {
		capacity += capacity * storageTraitBonusPercent / 100
	}
	return capacity
}

// hasStorageForLocked reports whether units more storage fits in the office.
func (s *GameState) hasStorageForLocked(office *RuntimeOffice, units int) bool {
	return s.storageUsedLocked()+units <= s.usableStorageLocked(office)
}

func strPtr(v string) *string { return &v }
func intPtr(v int) *int       { return &v }
