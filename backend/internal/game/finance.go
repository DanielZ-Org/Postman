package game

// Transaction is a minimal in-memory finance record created by a monetary mutation, preserving the
// SPEC 11.3 invariant that monetary mutations create a finance transaction rather than silently
// changing cash. M1D only creates office_down_payment transactions; it does not expose a finance
// API, assign transaction IDs, or persist records — those belong to later bounded Acts.
type Transaction struct {
	Type         string // e.g. "office_down_payment"
	Amount       int    // signed integer pounds (negative = expense)
	GameDatetime string // authoritative fictional clock time at the mutation (never wall clock)
	BalanceAfter int    // cash balance after this transaction, integer pounds
	OfficeID     string // related office ID
}
