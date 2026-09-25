package game

// Player is the persistent player identity and non-cash attributes. The authoritative
// cash balance lives on GameState (guarded by its lock together with transactions, so
// money mutations stay atomic); this type carries only the fields SPEC 3 models on the
// player itself. A new game starts with a £1,000 loan whose principal never amortises
// (SPEC 11.1: repayment mechanics are OPEN, so interest does not reduce principal).
type Player struct {
	ID            string
	Name          string
	LogoID        string // backend stores asset references only; empty until one exists
	AvatarID      string
	Trait         string // "financial" | "storage" | "logistics"; default "financial"
	LoanPrincipal int    // integer pounds
	HeadOfficeID  string // set on first office selection; empty before that
}

// Default player identity for a new game. The trait selection UI/API is not part of
// the first slice, so the labelled temporary default is the SPEC 3 example trait.
var defaultPlayer = Player{
	ID:            "player-1",
	Name:          "Daniel",
	Trait:         "financial",
	LoanPrincipal: StartingCash,
}
