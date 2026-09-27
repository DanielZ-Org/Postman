package game

import "errors"

// Trait-selection errors (SPEC 3). The API layer maps each to its canonical machine
// error code and HTTP status; every failed selection leaves state unchanged.
var (
	ErrInvalidTrait         = errors.New("invalid trait value")               // INVALID_TRAIT 400
	ErrTraitAlreadySelected = errors.New("a trait has already been selected") // TRAIT_ALREADY_SELECTED 409
)

// SelectTrait atomically selects the player's one-time trait (SPEC 3: "one player
// trait may be selected"). It performs one atomic game operation under s.mu in the
// deterministic validation order required by SPEC 3: (1) game over; (2) trait value
// out of the allowed set; (3) trait already explicitly selected; (4) head office
// already selected. On success it sets player.Trait and raises the one-time
// selection flag — no cash change, since SPEC defines no cost for trait selection.
// If any validation fails, no state is changed (no partial mutation).
//
// Labelled default per the approved plan: one-time selection, only before the first
// office selection. Once an office has been selected the window closes even if its
// contract later terminates; the player's trait stays at whatever was chosen (or the
// default) and cannot be changed afterwards.
//
// Lock ordering: SelectTrait holds s.mu while assembling the returned projection via
// playerViewLocked, which does not take any further lock. All locks are released
// before returning, so no game lock is held during downstream HTTP JSON encoding.
func (s *GameState) SelectTrait(trait string) (PlayerView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Game-over check first: the game has ended, so nothing may be selected anymore.
	if s.status == GameStatusGameOver {
		return PlayerView{}, ErrGameOver
	}

	// (2) trait value must be one of the SPEC 3 allowed values; a missing or
	// wrong-type value never reaches here (the API layer rejects it as INVALID_REQUEST).
	if !isAllowedTrait(trait) {
		return PlayerView{}, ErrInvalidTrait
	}

	// (3) one-time selection: an explicitly chosen trait cannot be changed. The flag,
	// not the trait value, is authoritative — re-selecting the default "financial"
	// counts as an explicit selection too.
	if s.traitSelected {
		return PlayerView{}, ErrTraitAlreadySelected
	}

	// (4) a trait must be selected before the first office selection; once an office
	// exists (active or terminated) the window has closed.
	if s.selectedOffice != nil {
		return PlayerView{}, ErrAlreadySelected
	}

	// Commit atomically: set the trait and raise the one-time flag under the same lock
	// so a concurrent selection cannot interleave. No cash change — SPEC defines no
	// cost for trait selection, so no transaction is posted.
	s.player.Trait = trait
	s.traitSelected = true

	return s.playerViewLocked(), nil
}
