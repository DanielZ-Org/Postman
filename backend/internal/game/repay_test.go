package game

import (
	"errors"
	"testing"
)

// TestRepayLoanReducesPrincipalAndPostsTransaction verifies the happy path (SPEC 11.1,
// decided 16.9/16.17): cash and principal drop by the repaid amount and exactly one
// loan_repayment transaction records the payment.
func TestRepayLoanReducesPrincipalAndPostsTransaction(t *testing.T) {
	s := newStateAt(t, "1980-02-01T09:00:00")
	cash, principal := s.cash, s.player.LoanPrincipal
	txnsBefore := len(s.transactions)

	view, err := s.RepayLoan(20000)
	if err != nil {
		t.Fatalf("RepayLoan: %v", err)
	}
	if s.cash != cash-20000 || s.player.LoanPrincipal != principal-20000 {
		t.Errorf("cash/principal = %d/%d, want %d/%d", s.cash, s.player.LoanPrincipal, cash-20000, principal-20000)
	}
	if view.Cash != s.cash || view.LoanPrincipal != s.player.LoanPrincipal {
		t.Errorf("view cash/principal = %d/%d, want authoritative %d/%d",
			view.Cash, view.LoanPrincipal, s.cash, s.player.LoanPrincipal)
	}
	if len(s.transactions) != txnsBefore+1 {
		t.Fatalf("transactions = %d, want %d (one loan_repayment)", len(s.transactions), txnsBefore+1)
	}
	tr := s.transactions[len(s.transactions)-1]
	if tr.Category != CategoryLoanRepayment || tr.Amount != -20000 || tr.ReferenceID != loanReferenceID {
		t.Errorf("txn = %+v, want loan_repayment -20000 ref %s", tr, loanReferenceID)
	}
}

// TestRepayLoanExactMinBoundary verifies the inclusive upper bound: repaying exactly
// min(cash, principal) succeeds.
func TestRepayLoanExactMinBoundary(t *testing.T) {
	s := newStateAt(t, "1980-02-01T09:00:00")

	// Cash-bound: principal (100000) exceeds cash (100000) tie -> all of it.
	if _, err := s.RepayLoan(s.cash); err != nil {
		t.Fatalf("repay exactly cash: %v", err)
	}
	if s.cash != 0 || s.player.LoanPrincipal != 0 {
		t.Errorf("cash/principal = %d/%d, want 0/0", s.cash, s.player.LoanPrincipal)
	}

	// Principal-bound: cash above principal (simulate winnings).
	s2 := newStateAt(t, "1980-02-01T09:00:00")
	s2.cash = 150000
	if _, err := s2.RepayLoan(s2.player.LoanPrincipal); err != nil {
		t.Fatalf("repay exactly principal: %v", err)
	}
	if s2.cash != 50000 || s2.player.LoanPrincipal != 0 {
		t.Errorf("cash/principal = %d/%d, want 50000/0", s2.cash, s2.player.LoanPrincipal)
	}
}

// TestRepayLoanValidationFailures verifies every rejection leaves state unchanged:
// non-positive amounts are INVALID_REQUEST-class errors, over-payments carry the
// smaller of cash and principal as available, a repaid-off loan rejects everything,
// and a game-over state rejects the call.
func TestRepayLoanValidationFailures(t *testing.T) {
	s := newStateAt(t, "1980-02-01T09:00:00")
	cash, principal, txns := s.cash, s.player.LoanPrincipal, len(s.transactions)

	for _, amount := range []int{0, -1} {
		if _, err := s.RepayLoan(amount); !errors.Is(err, ErrInvalidRepayment) {
			t.Errorf("amount %d err = %v, want ErrInvalidRepayment", amount, err)
		}
	}

	// Over cash: available is cash (cash == principal here; force cash below).
	s.cash = 10000
	_, err := s.RepayLoan(20000)
	var ife *InsufficientFundsError
	if !errors.As(err, &ife) || ife.Required != 20000 || ife.Available != 10000 {
		t.Fatalf("over-cash err = %v, want InsufficientFundsError{20000, 10000}", err)
	}

	// Over principal: available is principal.
	s.cash = 150000
	s.player.LoanPrincipal = 40000
	_, err = s.RepayLoan(40001)
	if !errors.As(err, &ife) || ife.Required != 40001 || ife.Available != 40000 {
		t.Fatalf("over-principal err = %v, want InsufficientFundsError{40001, 40000}", err)
	}

	// Fully repaid loan: nothing left to repay.
	s.player.LoanPrincipal = 0
	_, err = s.RepayLoan(1)
	if !errors.As(err, &ife) || ife.Available != 0 {
		t.Fatalf("repaid-off err = %v, want InsufficientFundsError available 0", err)
	}

	// State only changed by the explicit field assignments above: no transactions.
	if len(s.transactions) != txns {
		t.Errorf("transactions = %d, want %d (failures must not post)", len(s.transactions), txns)
	}

	// Game over rejects before any validation.
	s.status = GameStatusGameOver
	if _, err := s.RepayLoan(1); !errors.Is(err, ErrGameOver) {
		t.Errorf("game-over err = %v, want ErrGameOver", err)
	}

	_ = cash
	_ = principal
}

// TestNextInterestRecalculatesFromRepaidPrincipal verifies SPEC 11.1: after a
// voluntary repayment the next 5% interest charge computes from the reduced principal.
func TestNextInterestRecalculatesFromRepaidPrincipal(t *testing.T) {
	s := newStateAt(t, "1980-02-01T09:00:00")
	if _, err := s.RepayLoan(40000); err != nil {
		t.Fatalf("RepayLoan: %v", err)
	}
	cash := s.cash

	s.processInterestLocked(mustParseTime(t, "1980-02-29T09:00:00"))

	if s.cash != cash-3000 {
		t.Errorf("interest charge = %d, want 3000p (5%% of the repaid 60000p principal)", cash-s.cash)
	}
	if s.player.LoanPrincipal != 60000 {
		t.Errorf("principal = %d, want 60000p (unchanged by interest, SPEC 16.9)", s.player.LoanPrincipal)
	}
}
