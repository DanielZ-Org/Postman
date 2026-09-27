package game

import "time"

// Store is the persistence abstraction owned by the game/application layer (SPEC
// 13): the game layer defines it and never imports an adapter; infrastructure
// (e.g. the SQLite package) implements it. It stays focused on what the application
// needs — one complete snapshot — rather than per-entity repositories.
type Store interface {
	// Load returns the saved snapshot, or (nil, nil) when no saved game exists.
	Load() (*Snapshot, error)
	// Save persists the given snapshot durably.
	Save(snapshot *Snapshot) error
}

// Snapshot is the complete persisted game state (SPEC 13): clock/calendar, player,
// office contract, packages, employees, finance transactions and the scheduling
// counters the simulation resumes from. Save/load preserves game time exactly; no
// offline catch-up occurs (SPEC 2.3).
type Snapshot struct {
	Version           int            `json:"version"`
	GameTime          string         `json:"game_time"`
	Speed             int            `json:"speed"`
	Paused            bool           `json:"paused"`
	Status            string         `json:"status"`
	Player            Player         `json:"player"`
	TraitSelected     bool           `json:"trait_selected"`
	Cash              int            `json:"cash"`
	Office            *RuntimeOffice `json:"office"`
	Packages          []*Package     `json:"packages"`
	Employees         []*Employee    `json:"employees"`
	Runs              []SnapshotRun  `json:"runs"`
	Transactions      []Transaction  `json:"transactions"`
	NextTxnID         int            `json:"next_txn_id"`
	NextPkgSeq        int            `json:"next_pkg_seq"`
	NextEmpSeq        int            `json:"next_emp_seq"`
	TotalHires        int            `json:"total_hires"`
	BicyclesOwned     int            `json:"bicycles_owned"`
	CarsOwned         int            `json:"cars_owned"`
	LastGeneration    string         `json:"last_generation"`
	InterestDue       string         `json:"interest_due"`
	PayrollDue        string         `json:"payroll_due"`
	DeliveredToday    int            `json:"delivered_today"`
	DeliveredTodayKey string         `json:"delivered_today_key"`
	DailyRevenue      int            `json:"daily_revenue"`
	DailyRevenueKey   string         `json:"daily_revenue_key"`
}

// SnapshotRun is a persisted delivery run; PhaseEnd is stored in canonical game-time
// format so a reload resumes phases at the exact fictional instant.
type SnapshotRun struct {
	EmployeeID string   `json:"employee_id"`
	PackageIDs []string `json:"package_ids"`
	Phase      string   `json:"phase"`
	PhaseEnd   string   `json:"phase_end"`
}

// snapshotVersion identifies the current snapshot schema. v2 moved every monetary
// field from integer pounds to integer pence (board decision, SPEC 4.3); a v1 save is
// migrated at load time by migrateV1ToV2 and saved forward as v2. Any other version is
// treated as "no saved game" by adapters.
const snapshotVersion = 2

// Snapshot builds a consistent copy of the entire game state for persistence. It
// holds the state lock for the whole copy so a concurrent tick or mutation cannot
// interleave.
func (s *GameState) Snapshot() *Snapshot {
	s.mu.Lock()
	defer s.mu.Unlock()

	now, speed, paused := s.Clock.parts()
	snap := &Snapshot{
		Version:           snapshotVersion,
		GameTime:          now.Format(GameTimeFormat),
		Speed:             speed,
		Paused:            paused,
		Status:            s.status,
		Player:            s.player,
		TraitSelected:     s.traitSelected,
		Cash:              s.cash,
		Transactions:      append([]Transaction(nil), s.transactions...),
		NextTxnID:         s.nextTxnID,
		NextPkgSeq:        s.nextPkgSeq,
		NextEmpSeq:        s.nextEmpSeq,
		TotalHires:        s.totalHires,
		BicyclesOwned:     s.bicyclesOwned,
		CarsOwned:         s.carsOwned,
		LastGeneration:    s.lastGeneration.Format(GameTimeFormat),
		InterestDue:       s.interestDue.Format(GameTimeFormat),
		PayrollDue:        s.payrollDue.Format(GameTimeFormat),
		DeliveredToday:    s.deliveredToday,
		DeliveredTodayKey: s.deliveredTodayKey,
		DailyRevenue:      s.dailyRevenue,
		DailyRevenueKey:   s.dailyRevenueKey,
	}
	if s.selectedOffice != nil {
		office := *s.selectedOffice
		office.AcceptedPackageSizes = append([]string{}, s.selectedOffice.AcceptedPackageSizes...)
		snap.Office = &office
	}
	snap.Packages = make([]*Package, 0, len(s.packages))
	for _, p := range s.packages {
		cp := *p
		if p.AssignedEmployeeID != nil {
			cp.AssignedEmployeeID = strPtr(*p.AssignedEmployeeID)
		}
		if p.DeliveredAt != nil {
			cp.DeliveredAt = strPtr(*p.DeliveredAt)
		}
		if p.FinalRevenue != nil {
			cp.FinalRevenue = intPtr(*p.FinalRevenue)
		}
		snap.Packages = append(snap.Packages, &cp)
	}
	snap.Employees = make([]*Employee, 0, len(s.employees))
	for _, e := range s.employees {
		cp := *e
		cp.Skills = append([]string{}, e.Skills...)
		snap.Employees = append(snap.Employees, &cp)
	}
	snap.Runs = make([]SnapshotRun, 0, len(s.runs))
	for _, r := range s.runs {
		snap.Runs = append(snap.Runs, SnapshotRun{
			EmployeeID: r.EmployeeID,
			PackageIDs: append([]string{}, r.PackageIDs...),
			Phase:      r.Phase,
			PhaseEnd:   r.PhaseEnd.Format(GameTimeFormat),
		})
	}
	return snap
}

// Restore replaces the authoritative state with the snapshot's contents (used when
// loading a saved game at startup). It validates every stored instant and fails
// without touching state if the snapshot is unusable, so a corrupt save cannot leave
// a half-restored game behind.
func (s *GameState) Restore(snap *Snapshot) error {
	if snap == nil {
		return nil
	}
	switch snap.Version {
	case snapshotVersion:
		// current schema; nothing to migrate
	case 1:
		migrateV1ToV2(snap)
	default:
		return &snapshotError{"unsupported snapshot version " + itoa(snap.Version)}
	}

	gameTime, err := time.Parse(GameTimeFormat, snap.GameTime)
	if err != nil {
		return &snapshotError{"invalid game_time: " + err.Error()}
	}
	lastGen, err := time.Parse(GameTimeFormat, snap.LastGeneration)
	if err != nil {
		return &snapshotError{"invalid last_generation: " + err.Error()}
	}
	interestDue, err := time.Parse(GameTimeFormat, snap.InterestDue)
	if err != nil {
		return &snapshotError{"invalid interest_due: " + err.Error()}
	}
	payrollDue, err := time.Parse(GameTimeFormat, snap.PayrollDue)
	if err != nil {
		return &snapshotError{"invalid payroll_due: " + err.Error()}
	}
	runs := make([]*Run, 0, len(snap.Runs))
	for _, r := range snap.Runs {
		end, err := time.Parse(GameTimeFormat, r.PhaseEnd)
		if err != nil {
			return &snapshotError{"invalid run phase_end: " + err.Error()}
		}
		runs = append(runs, &Run{
			EmployeeID: r.EmployeeID,
			PackageIDs: append([]string{}, r.PackageIDs...),
			Phase:      r.Phase,
			PhaseEnd:   end,
		})
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	s.Clock.restore(gameTime, snap.Speed, snap.Paused)
	s.status = snap.Status
	s.player = snap.Player
	// A save written before trait selection existed lacks the field; Go's zero value on
	// unmarshal defaults it to false (not explicitly selected), which is correct.
	s.traitSelected = snap.TraitSelected
	s.cash = snap.Cash
	s.selectedOffice = snap.Office
	if s.selectedOffice != nil {
		office := *snap.Office
		office.AcceptedPackageSizes = append([]string{}, snap.Office.AcceptedPackageSizes...)
		s.selectedOffice = &office
	}
	s.packages = snap.Packages
	s.employees = snap.Employees
	s.runs = runs
	s.transactions = append([]Transaction(nil), snap.Transactions...)
	s.nextTxnID = snap.NextTxnID
	s.nextPkgSeq = snap.NextPkgSeq
	s.nextEmpSeq = snap.NextEmpSeq
	s.totalHires = snap.TotalHires
	// Saves written before vehicle inventory existed lack the fields; Go's zero value on
	// unmarshal defaults both to 0 (nothing owned), which is correct.
	s.bicyclesOwned = snap.BicyclesOwned
	s.carsOwned = snap.CarsOwned
	s.lastGeneration = lastGen
	s.interestDue = interestDue
	s.payrollDue = payrollDue
	s.deliveredToday = snap.DeliveredToday
	s.deliveredTodayKey = snap.DeliveredTodayKey
	s.dailyRevenue = snap.DailyRevenue
	s.dailyRevenueKey = snap.DailyRevenueKey
	return nil
}

// migrateV1ToV2 converts a v1 snapshot (monetary fields in integer pounds) to the
// current v2 schema (integer pence): every monetary field — Cash, Player.LoanPrincipal,
// Office.DownPayment/WeeklyRent, Package.BaseFee/FinalRevenue, Employee.AccruedWages,
// Transaction.Amount and Snapshot.DailyRevenue — is multiplied by 100 and the version
// is bumped, so the next save persists the migrated state forward as v2. This keeps
// existing dev saves valid across the unit change without losing their values.
func migrateV1ToV2(snap *Snapshot) {
	snap.Cash *= 100
	snap.Player.LoanPrincipal *= 100
	if snap.Office != nil {
		snap.Office.DownPayment *= 100
		snap.Office.WeeklyRent *= 100
	}
	for _, p := range snap.Packages {
		p.BaseFee *= 100
		if p.FinalRevenue != nil {
			*p.FinalRevenue *= 100
		}
	}
	for _, e := range snap.Employees {
		e.AccruedWages *= 100
	}
	for i := range snap.Transactions {
		snap.Transactions[i].Amount *= 100
	}
	snap.DailyRevenue *= 100
	snap.Version = snapshotVersion
}

// snapshotError reports an unusable persisted snapshot.
type snapshotError struct{ detail string }

func (e *snapshotError) Error() string { return "game: unusable snapshot: " + e.detail }
