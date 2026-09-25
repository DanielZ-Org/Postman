package game

// PlayerView is the read-only player projection for GET /api/v1/player (SPEC 3).
// LogoID/AvatarID are nil until the backend stores an asset reference; SPEC 3 allows
// them to be absent initially.
type PlayerView struct {
	ID            string  `json:"id"`
	Name          string  `json:"name"`
	LogoID        *string `json:"logo_id"`
	AvatarID      *string `json:"avatar_id"`
	Trait         string  `json:"trait"`
	Cash          int     `json:"cash"`
	LoanPrincipal int     `json:"loan_principal"`
	HeadOfficeID  *string `json:"head_office_id"`
}

// PlayerView assembles the read-only player projection.
func (s *GameState) PlayerView() PlayerView {
	s.mu.Lock()
	defer s.mu.Unlock()
	v := PlayerView{
		ID:            s.player.ID,
		Name:          s.player.Name,
		Trait:         s.player.Trait,
		Cash:          s.cash,
		LoanPrincipal: s.player.LoanPrincipal,
	}
	if s.player.LogoID != "" {
		v.LogoID = strPtr(s.player.LogoID)
	}
	if s.player.AvatarID != "" {
		v.AvatarID = strPtr(s.player.AvatarID)
	}
	if s.player.HeadOfficeID != "" {
		v.HeadOfficeID = strPtr(s.player.HeadOfficeID)
	}
	return v
}

// PackagesView returns deep copies of every package (delivered ones included, so the
// UI can show history). Pointer fields are re-boxed so callers cannot mutate
// authoritative state through the returned values.
func (s *GameState) PackagesView() []Package {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Package, 0, len(s.packages))
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
		out = append(out, cp)
	}
	return out
}

// EmployeesView returns deep copies of employees plus the current hiring state (SPEC
// 7.5 / 8). Skills slices are re-copied so callers cannot mutate authoritative data.
func (s *GameState) EmployeesView() ([]Employee, HiringState) {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Employee, 0, len(s.employees))
	for _, e := range s.employees {
		cp := *e
		cp.Skills = append([]string{}, e.Skills...)
		out = append(out, cp)
	}
	return out, s.hiringStateLocked()
}

// FinanceView returns the current-week finance statement (SPEC 11.3) for GET
// /api/v1/finance. The window comes from the authoritative game clock, never the
// host wall clock.
func (s *GameState) FinanceView() FinanceStatement {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.financeStatementLocked(s.Clock.Now())
}

// TransactionsView returns transactions newest-first for GET
// /api/v1/finance/transactions.
func (s *GameState) TransactionsView() []Transaction {
	s.mu.Lock()
	defer s.mu.Unlock()
	out := make([]Transaction, 0, len(s.transactions))
	for i := len(s.transactions) - 1; i >= 0; i-- {
		out = append(out, s.transactions[i])
	}
	return out
}

// SelectedOfficeView returns a deep copy of the selected runtime office, or nil when
// no office has been selected. A terminated contract is still returned (its
// contract_status tells the client), unlike the /game projection which shows an
// active office only.
func (s *GameState) SelectedOfficeView() *RuntimeOffice {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.selectedOffice == nil {
		return nil
	}
	cp := *s.selectedOffice
	cp.AcceptedPackageSizes = append([]string{}, s.selectedOffice.AcceptedPackageSizes...)
	return &cp
}
