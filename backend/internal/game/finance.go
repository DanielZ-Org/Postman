package game

import (
	"errors"
	"time"
)

// Transaction is the SPEC 11.3 finance record: every monetary mutation appends one
// instead of silently changing cash. Amounts are signed integer pence (negative =
// expense). GameDatetime is authoritative fictional game time, never the host wall
// clock.
type Transaction struct {
	ID           string `json:"id"`
	GameDatetime string `json:"game_datetime"`
	Category     string `json:"category"`
	Amount       int    `json:"amount"`
	Description  string `json:"description"`
	ReferenceID  string `json:"reference_id"`
}

// Finance categories (SPEC 11.3).
const (
	CategoryLoanDisbursement   = "loan_disbursement"
	CategoryPackageRevenue     = "package_revenue"
	CategoryTraitBonus         = "financial_trait_bonus"
	CategoryOfficeDownPayment  = "office_down_payment"
	CategoryRent               = "rent"
	CategoryRentLateFee        = "rent_late_fee"
	CategoryHiringBonus        = "hiring_bonus"
	CategoryEmployeeWages      = "employee_wages"
	CategoryLoanInterest       = "loan_interest"
	CategoryLoanRepayment      = "loan_repayment"
	CategoryVehiclePurchase    = "vehicle_purchase"
	CategoryVehicleFuel        = "vehicle_fuel"
	CategoryVehicleMaintenance = "vehicle_maintenance"
	CategoryUpgrade            = "upgrade"
)

// postTransactionLocked applies a money mutation: adjusts cash by amount and appends
// exactly one transaction. Callers must hold s.mu. The clock time passed in is the
// authoritative game instant of the mutation.
func (s *GameState) postTransactionLocked(now time.Time, category string, amount int, description, ref string) {
	s.cash += amount
	s.nextTxnID++
	s.transactions = append(s.transactions, Transaction{
		ID:           txnID(s.nextTxnID),
		GameDatetime: now.Format(GameTimeFormat),
		Category:     category,
		Amount:       amount,
		Description:  description,
		ReferenceID:  ref,
	})
}

// txnID formats a 1-based transaction counter as the canonical "txn-000001" id.
func txnID(n int) string {
	return padID("txn-", n, 6)
}

// packageID formats a 1-based package counter as the canonical "pkg-000001" id.
func packageID(n int) string {
	return padID("pkg-", n, 6)
}

// employeeID formats a 1-based employee counter as the canonical "emp-0001" id.
func employeeID(n int) string {
	return padID("emp-", n, 4)
}

// padID renders n as a zero-padded decimal with the given prefix and width.
func padID(prefix string, n, width int) string {
	s := itoa(n)
	for len(s) < width {
		s = "0" + s
	}
	return prefix + s
}

// itoa is a tiny decimal formatter (avoids pulling strconv into this file's callers).
func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	neg := n < 0
	if neg {
		n = -n
	}
	var buf [20]byte
	i := len(buf)
	for n > 0 {
		i--
		buf[i] = byte('0' + n%10)
		n /= 10
	}
	if neg {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// FinanceStatement is the SPEC 11.3 statement view for GET /api/v1/finance: current
// balance plus an income/expense breakdown for one game week, the liability overview,
// and the previous week's totals for the week-over-week comparison (issue #27). All
// amounts are integer pence.
type FinanceStatement struct {
	CashBalance    int                 `json:"cash_balance"`
	Period         FinancePeriod       `json:"period"`
	Income         FinanceIncome       `json:"income"`
	Expenses       FinanceExpense      `json:"expenses"`
	NetChange      int                 `json:"net_change"`
	Liabilities    FinanceLiabs        `json:"liabilities"`
	PreviousPeriod FinancePeriodTotals `json:"previous_period"`
}

// FinancePeriod is the inclusive [from, to] game-time window of a statement.
type FinancePeriod struct {
	From string `json:"from"`
	To   string `json:"to"`
}

// FinanceIncome aggregates positive income categories for the period (SPEC 11.3).
type FinanceIncome struct {
	PackageRevenue int `json:"package_revenue"`
	TraitBonus     int `json:"trait_bonus"`
	Total          int `json:"total"`
}

// FinanceExpense aggregates expense categories for the period (SPEC 11.3).
type FinanceExpense struct {
	EmployeeWages      int `json:"employee_wages"`
	Rent               int `json:"rent"`
	LoanInterest       int `json:"loan_interest"`
	Hiring             int `json:"hiring"`
	VehicleFuel        int `json:"vehicle_fuel"`
	VehicleMaintenance int `json:"vehicle_maintenance"`
	Other              int `json:"other"`
	Total              int `json:"total"`
}

// FinanceLiabs lists outstanding obligations (SPEC 11.3). NextInterestEstimate is 5%
// of principal; principal never amortises (decided SPEC 16.9 — interest-only for now,
// voluntary repayment is M3 scope).
type FinanceLiabs struct {
	LoanPrincipal        int `json:"loan_principal"`
	AccruedEmployeeWages int `json:"accrued_employee_wages"`
	NextRentAmount       int `json:"next_rent_amount"`
	NextInterestEstimate int `json:"next_interest_estimate"`
}

// FinancePeriodTotals is the income/expense/net breakdown for one statement window
// without the balance or liability context. It backs previous_period: the same
// aggregation for the immediately preceding week so the finance UI can show a
// week-over-week comparison without re-bucketing transactions client-side (issue #27).
type FinancePeriodTotals struct {
	From      string         `json:"from"`
	To        string         `json:"to"`
	Income    FinanceIncome  `json:"income"`
	Expenses  FinanceExpense `json:"expenses"`
	NetChange int            `json:"net_change"`
}

// accumulateFinancePeriod buckets every transaction whose game datetime falls in the
// inclusive [fromStr, toStr] window into the SPEC 11.3 income/expense totals. Positive
// amounts outside the income categories and non-transaction cash movements do not
// appear; every other negative amount lands in expenses' default "other" bucket.
func accumulateFinancePeriod(transactions []Transaction, fromStr, toStr string) (FinanceIncome, FinanceExpense) {
	var inc FinanceIncome
	var exp FinanceExpense
	for _, t := range transactions {
		if t.GameDatetime < fromStr || t.GameDatetime > toStr {
			continue
		}
		switch {
		case t.Amount > 0 && t.Category == CategoryPackageRevenue:
			inc.PackageRevenue += t.Amount
		case t.Amount > 0 && t.Category == CategoryTraitBonus:
			inc.TraitBonus += t.Amount
		case t.Amount < 0:
			abs := -t.Amount
			switch t.Category {
			case CategoryEmployeeWages:
				exp.EmployeeWages += abs
			case CategoryRent:
				exp.Rent += abs
			case CategoryLoanInterest:
				exp.LoanInterest += abs
			case CategoryHiringBonus:
				exp.Hiring += abs
			case CategoryVehicleFuel:
				exp.VehicleFuel += abs
			case CategoryVehicleMaintenance:
				exp.VehicleMaintenance += abs
			default:
				exp.Other += abs
			}
		}
	}
	inc.Total = inc.PackageRevenue + inc.TraitBonus
	exp.Total = exp.EmployeeWages + exp.Rent + exp.LoanInterest + exp.Hiring +
		exp.VehicleFuel + exp.VehicleMaintenance + exp.Other
	return inc, exp
}

// financeStatementLocked builds the statement for the game week containing now. The
// period runs from the canonical start (SPEC 2.1) in 7-day windows: week 0 is
// 1980-02-01T00:00:00 to 1980-02-07T23:59:59, matching the SPEC 11.3 example. The
// previous period is the identical window shifted back 7 days — before the canonical
// start it is a zero-activity window on the same weekly grid. Callers must hold s.mu.
func (s *GameState) financeStatementLocked(now time.Time) FinanceStatement {
	week := 0
	if now.After(startTime) {
		week = int(now.Sub(startTime) / (7 * 24 * time.Hour))
	}
	fromBase := startTime.AddDate(0, 0, week*7)
	from := time.Date(fromBase.Year(), fromBase.Month(), fromBase.Day(), 0, 0, 0, 0, fromBase.Location())
	to := from.Add(7*24*time.Hour - time.Second)
	fromStr := from.Format(GameTimeFormat)
	toStr := to.Format(GameTimeFormat)

	inc, exp := accumulateFinancePeriod(s.transactions, fromStr, toStr)

	var accruedWages int
	for _, e := range s.employees {
		accruedWages += e.AccruedWages
	}
	nextRent := 0
	if s.selectedOffice != nil && s.selectedOffice.ContractStatus == ContractActive {
		nextRent = s.selectedOffice.WeeklyRent
	}

	prevFrom := from.AddDate(0, 0, -7)
	prevTo := prevFrom.Add(7*24*time.Hour - time.Second)
	pinc, pexp := accumulateFinancePeriod(s.transactions, prevFrom.Format(GameTimeFormat), prevTo.Format(GameTimeFormat))

	return FinanceStatement{
		CashBalance: s.cash,
		Period:      FinancePeriod{From: fromStr, To: toStr},
		Income:      inc,
		Expenses:    exp,
		NetChange:   inc.Total - exp.Total,
		Liabilities: FinanceLiabs{
			LoanPrincipal:        s.player.LoanPrincipal,
			AccruedEmployeeWages: accruedWages,
			NextRentAmount:       nextRent,
			NextInterestEstimate: percentOf(s.player.LoanPrincipal, interestPercent),
		},
		PreviousPeriod: FinancePeriodTotals{
			From:      prevFrom.Format(GameTimeFormat),
			To:        prevTo.Format(GameTimeFormat),
			Income:    pinc,
			Expenses:  pexp,
			NetChange: pinc.Total - pexp.Total,
		},
	}
}

// ErrInvalidRepayment reports a repayment amount outside the SPEC 11.1 contract
// (amount must be a positive integer): INVALID_REQUEST 400 at the API boundary.
var ErrInvalidRepayment = errors.New("repayment amount must be a positive integer")

// RepayLoan validates and commits one voluntary loan repayment under s.mu (SPEC 11.1,
// decided 16.9/16.17): the amount must satisfy 0 < amount <= min(cash, principal). On
// success the principal drops by the amount and exactly one loan_repayment transaction
// posts for the paid amount; the next 5% interest charge recalculates from the new
// principal. Every failure (game over, invalid amount, unaffordable) leaves state
// unchanged.
func (s *GameState) RepayLoan(amount int) (PlayerView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == GameStatusGameOver {
		return PlayerView{}, ErrGameOver
	}
	if amount <= 0 {
		return PlayerView{}, ErrInvalidRepayment
	}
	available := s.cash
	if s.player.LoanPrincipal < available {
		available = s.player.LoanPrincipal
	}
	if amount > available {
		return PlayerView{}, &InsufficientFundsError{Required: amount, Available: available}
	}
	s.player.LoanPrincipal -= amount
	s.postTransactionLocked(s.Clock.Now(), CategoryLoanRepayment, -amount, "Voluntary loan repayment", loanReferenceID)
	return s.playerViewLocked(), nil
}
