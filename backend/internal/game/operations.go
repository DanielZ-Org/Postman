package game

import (
	"errors"
	"fmt"
)

// Game-layer operation errors. The API layer maps each to its canonical machine
// error code and HTTP status (SPEC 14.1); every failed operation leaves state
// unchanged.
var (
	ErrNoOffice            = errors.New("no active head office")                          // NO_OFFICE 409
	ErrOfficeFull          = errors.New("employee capacity reached")                      // OFFICE_FULL 409
	ErrEmployeeNotFound    = errors.New("employee not found")                             // EMPLOYEE_NOT_FOUND 404
	ErrEmployeeBusy        = errors.New("employee is not available")                      // EMPLOYEE_BUSY 409
	ErrNoStoredPackages    = errors.New("no stored packages")                             // NO_STORED_PACKAGES 409
	ErrRunsLimitReached    = errors.New("daily run limit reached")                        // RUNS_LIMIT_REACHED 409
	ErrCycleWouldNotFinish = errors.New("delivery cycle would not finish before closing") // CYCLE_WOULD_NOT_FINISH 409
	ErrGameOver            = errors.New("game is over")                                   // GAME_OVER 409 (labelled)
)

// CapacityError reports that a requested assignment batch does not fit the foot
// delivery capacity or the stored inventory (INSUFFICIENT_DELIVERY_CAPACITY 400).
type CapacityError struct {
	EmployeeID      string
	Available       int // usable capacity units for this run
	Requested       int // capacity units the selection would consume
	StoredAvailable int
}

func (e *CapacityError) Error() string {
	return fmt.Sprintf("delivery capacity exceeded: %d requested of %d available", e.Requested, e.Available)
}

// CycleError reports that a full 4-hour walking cycle starting now would not finish
// before today's closing time (CYCLE_WOULD_NOT_FINISH 409).
type CycleError struct {
	RequestedStart string
	ClosingTime    string // empty when the office has no opening today
}

func (e *CycleError) Error() string {
	return fmt.Sprintf("cycle from %s would not finish before closing", e.RequestedStart)
}

// HiringState is the read-only hiring view exposed by the employees API (SPEC 8).
type HiringState struct {
	CurrentEmployeeCount int `json:"current_employee_count"`
	TotalHiresLifetime   int `json:"total_hires_lifetime"`
	NextHiringFee        int `json:"next_hiring_fee"`
}

// HireEmployee validates and commits one hiring action under s.mu: the game must be
// running, an active head office must exist (NO_OFFICE), headcount must be below the
// office employee capacity (OFFICE_FULL), and cash must cover the historical
// high-water hiring fee (INSUFFICIENT_FUNDS). On success it deducts the fee with one
// hiring_bonus transaction, bumps the lifetime counter and appends a ready employee.
func (s *GameState) HireEmployee() (*Employee, HiringState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == GameStatusGameOver {
		return nil, HiringState{}, ErrGameOver
	}
	office := s.selectedOffice
	if office == nil || office.ContractStatus != ContractActive {
		return nil, HiringState{}, ErrNoOffice
	}
	if len(s.employees) >= office.EmployeeCapacity {
		return nil, HiringState{}, ErrOfficeFull
	}
	fee := nextHiringFee(s.totalHires)
	if s.cash < fee {
		return nil, HiringState{}, &InsufficientFundsError{Required: fee, Available: s.cash}
	}

	now := s.Clock.Now()
	s.postTransactionLocked(now, CategoryHiringBonus, -fee,
		"Hiring fee #"+itoa(s.totalHires+1), "")
	s.totalHires++
	s.nextEmpSeq++
	emp := newEmployee(employeeID(s.nextEmpSeq), s.totalHires)
	emp.RunsTodayKey = dayKey(now)
	s.employees = append(s.employees, emp)

	return emp, s.hiringStateLocked(), nil
}

// hiringStateLocked builds the current hiring view. Callers must hold s.mu.
func (s *GameState) hiringStateLocked() HiringState {
	return HiringState{
		CurrentEmployeeCount: len(s.employees),
		TotalHiresLifetime:   s.totalHires,
		NextHiringFee:        nextHiringFee(s.totalHires),
	}
}

// AssignDelivery validates and commits one delivery assignment under s.mu. The
// backend picks the actual packages by canonical eligibility and priority (SPEC 9):
// stored only, express before normal, earliest due first (the priority rule is OPEN
// in SPEC 16.4; express-then-due is the labelled choice). Validation order:
// game over; employee exists (EMPLOYEE_NOT_FOUND); employee ready (EMPLOYEE_BUSY);
// stored inventory (NO_STORED_PACKAGES); batch fits capacity
// (INSUFFICIENT_DELIVERY_CAPACITY); daily run budget (RUNS_LIMIT_REACHED);
// full 4-hour cycle finishes before closing (CYCLE_WOULD_NOT_FINISH).
//
// On success the batch moves to assigned, the employee enters packing and a Run is
// appended; every package is chosen atomically so state never half-commits.
func (s *GameState) AssignDelivery(employeeID string, count int) (*Employee, []string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == GameStatusGameOver {
		return nil, nil, ErrGameOver
	}
	emp := s.findEmployeeLocked(employeeID)
	if emp == nil {
		return nil, nil, ErrEmployeeNotFound
	}
	if emp.Status != EmployeeReady {
		return nil, nil, ErrEmployeeBusy
	}

	// Eligibility + canonical priority: express first, then earliest due (SPEC 16.4
	// labelled assumption).
	var stored []*Package
	for _, p := range s.packages {
		if p.Status == PackageStored {
			stored = append(stored, p)
		}
	}
	if len(stored) == 0 {
		return nil, nil, ErrNoStoredPackages
	}
	sortPackagesByPriority(stored)
	batchCount := count
	if batchCount > len(stored) {
		batchCount = len(stored)
	}
	batch := stored[:batchCount]

	usable := footDeliveryCapacity
	if s.player.Trait == "logistics" {
		usable = footDeliveryCapacityWithLogistics // +10%, floor (SPEC 16.6 labelled)
	}
	units := 0
	for _, p := range batch {
		units += p.DeliveryCapacityUnits
	}
	if len(batch) < count || units > usable {
		return nil, nil, &CapacityError{
			EmployeeID:      emp.ID,
			Available:       usable,
			Requested:       units,
			StoredAvailable: len(stored),
		}
	}

	now := s.Clock.Now()
	// The per-day run budget resets on the game date (SPEC 9.3).
	if emp.RunsTodayKey != dayKey(now) {
		emp.RunsTodayKey = dayKey(now)
		emp.RunsToday = 0
	}
	if emp.RunsToday >= footRunsPerDay {
		return nil, nil, ErrRunsLimitReached
	}

	// A full walking cycle must finish before today's closing (SPEC 9.1).
	closeAt, hasClosing := closingInstantToday(now)
	if !hasClosing || now.Add(assignmentDuration).After(closeAt) {
		closing := ""
		if hasClosing {
			closing = closeAt.Format(GameTimeFormat)
		}
		return nil, nil, &CycleError{RequestedStart: now.Format(GameTimeFormat), ClosingTime: closing}
	}

	// Commit: move the batch, start the run and consume the run slot atomically.
	ids := make([]string, 0, len(batch))
	for _, p := range batch {
		p.Status = PackageAssigned
		p.AssignedEmployeeID = strPtr(emp.ID)
		ids = append(ids, p.ID)
	}
	emp.Status = EmployeePacking
	emp.RunsToday++
	s.runs = append(s.runs, &Run{
		EmployeeID: emp.ID,
		PackageIDs: ids,
		Phase:      RunPhasePacking,
		PhaseEnd:   now.Add(packingDuration),
	})

	return emp, ids, nil
}

// sortPackagesByPriority orders stored packages by the canonical batch priority:
// express before normal, then earliest due date (SPEC 16.4 labelled assumption).
func sortPackagesByPriority(packages []*Package) {
	// Insertion sort keeps the small in-memory batches simple and dependency-free.
	for i := 1; i < len(packages); i++ {
		for j := i; j > 0 && packagePriorityLess(packages[j], packages[j-1]); j-- {
			packages[j], packages[j-1] = packages[j-1], packages[j]
		}
	}
}

func packagePriorityLess(a, b *Package) bool {
	if a.ServiceType != b.ServiceType {
		return a.ServiceType == ServiceExpress
	}
	return a.DueAt < b.DueAt
}

// GameView is the read-only aggregate consumed by GET /api/v1/game (SPEC 14.2). It
// is assembled under the state lock and carries only plain values, so no lock is
// held during JSON encoding.
type GameView struct {
	Status       string
	GameDatetime string
	Speed        int
	Paused       bool

	PlayerID      string
	Trait         string
	Cash          int
	LoanPrincipal int
	HeadOfficeID  string

	HasOffice        bool
	OfficeID         string
	StorageUsed      int
	StorageCapacity  int
	EmployeeCount    int
	EmployeeCapacity int

	StoredPackages int
	OutForDelivery int
	DeliveredToday int

	AccruedWages int
	NextRent     int
}

// GameView assembles the read-only /game projection. Read-only: it never seeds or
// mutates state (SPEC 14.2).
func (s *GameState) GameView() GameView {
	s.mu.Lock()
	defer s.mu.Unlock()

	snap := s.Clock.Snapshot() // nested clock read lock; order state -> clock
	v := GameView{
		Status:        s.status,
		GameDatetime:  snap.GameDatetime,
		Speed:         snap.Speed,
		Paused:        snap.Paused,
		PlayerID:      s.player.ID,
		Trait:         s.player.Trait,
		Cash:          s.cash,
		LoanPrincipal: s.player.LoanPrincipal,
		HeadOfficeID:  s.player.HeadOfficeID,
	}
	if s.selectedOffice != nil && s.selectedOffice.ContractStatus == ContractActive {
		v.HasOffice = true
		v.OfficeID = s.selectedOffice.ID
		v.StorageCapacity = s.usableStorageLocked(s.selectedOffice)
		v.EmployeeCapacity = s.selectedOffice.EmployeeCapacity
		v.NextRent = s.selectedOffice.WeeklyRent
	}
	v.StorageUsed = s.storageUsedLocked()
	v.EmployeeCount = len(s.employees)
	for _, p := range s.packages {
		switch p.Status {
		case PackageStored:
			v.StoredPackages++
		case PackageOutForDelivery:
			v.OutForDelivery++
		}
	}
	v.DeliveredToday = s.deliveredToday
	for _, e := range s.employees {
		v.AccruedWages += e.AccruedWages
	}
	return v
}
