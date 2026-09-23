// Package state establishes the backend-owned application boundary for M1 game
// state.
//
// It intentionally contains no gameplay rules, package generation, simulation
// or persistence — those belong to later bounded Acts. The only value seeded is
// the canonical start time defined by SPEC (2.1).
package game

// InitialGameTime is the canonical start time for a new M1 game (SPEC 2.1):
// games begin on 1 February 1980 at 09:00 local game time.
const InitialGameTime = "1980-02-01T09:00:00"

// GameState is the minimal in-memory representation created at backend startup
// when no persisted game exists yet. It holds only SPEC-defined values and
// carries no rule logic.
type GameState struct {
	StartTime string
}

// NewInitialState returns the default M1 game state seeded from the canonical
// start time defined by SPEC (SPEC 2.1).
func NewInitialState() GameState {
	return GameState{StartTime: InitialGameTime}
}
