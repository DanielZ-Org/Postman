package game

// Player is the persistent player identity and non-cash attributes. The authoritative
// cash balance lives on GameState (guarded by its lock together with transactions, so
// money mutations stay atomic); this type carries only the fields SPEC 3 models on the
// player itself. A new game starts with a £1,000 loan whose principal never amortises
// (SPEC 11.1; repayment decided SPEC 16.9 — interest-only for now, voluntary repayment
// is M3 scope).
type Player struct {
	ID            string
	Name          string
	LogoID        string // backend stores asset references only; empty until one exists
	AvatarID      string
	Trait         string // "financial" | "storage" | "logistics"; default "financial"
	LoanPrincipal int    // integer pence
	HeadOfficeID  string // set on first office selection; empty before that
}

// Trait values selectable per SPEC 3 ("one player trait may be selected"). A new
// game starts with the SPEC 3 example trait as its labelled temporary default.
const (
	TraitFinancial = "financial"
	TraitStorage   = "storage"
	TraitLogistics = "logistics"
)

// validTraits is the canonical set of selectable traits in deterministic order. It
// drives both validation and the INVALID_TRAIT error details, so the wire contract
// never drifts from the accepted values.
var validTraits = []string{TraitFinancial, TraitStorage, TraitLogistics}

// ValidTraits returns a copy of the allowed trait values in canonical order (for API
// error details). Callers cannot mutate the canonical set through the returned slice.
func ValidTraits() []string {
	return append([]string(nil), validTraits...)
}

// isAllowedTrait reports whether trait is one of the SPEC 3 selectable traits.
func isAllowedTrait(trait string) bool {
	for _, v := range validTraits {
		if v == trait {
			return true
		}
	}
	return false
}

// Default player identity for a new game. The labelled temporary default is the SPEC 3
// example trait; SelectTrait (SPEC 3) replaces it with the player's one-time choice.
var defaultPlayer = Player{
	ID:            "player-1",
	Name:          "Daniel",
	Trait:         TraitFinancial,
	LoanPrincipal: StartingCash,
}
