package game

import (
	"encoding/json"
	"errors"
	"testing"
)

// TestSelectTraitCommits verifies a valid selection commits atomically: the trait is set, the
// one-time flag is raised, cash and transactions are untouched (SPEC defines no cost), and the
// returned projection matches the authoritative state.
func TestSelectTraitCommits(t *testing.T) {
	s := NewInitialState()
	if s.player.Trait != TraitFinancial || s.traitSelected {
		t.Fatalf("initial state = %v/%v, want default financial/not-selected", s.player.Trait, s.traitSelected)
	}

	view, err := s.SelectTrait(TraitLogistics)
	if err != nil {
		t.Fatalf("SelectTrait(logistics): %v", err)
	}
	if s.player.Trait != TraitLogistics || !s.traitSelected {
		t.Errorf("state = %v/%v, want logistics/selected", s.player.Trait, s.traitSelected)
	}
	if view.Trait != TraitLogistics || view.ID != "player-1" {
		t.Errorf("returned view = %+v, want trait logistics id player-1", view)
	}
	if got := s.Cash(); got != StartingCash {
		t.Errorf("cash = %d after selection, want unchanged %d (no cost)", got, StartingCash)
	}
	if len(s.transactions) != 1 {
		t.Errorf("transactions = %d after selection, want unchanged 1 (no transaction posted)", len(s.transactions))
	}

	// The returned projection matches the authoritative read.
	want := s.PlayerView()
	if view.Trait != want.Trait || view.Cash != want.Cash || view.ID != want.ID {
		t.Errorf("view = %+v, want %+v", view, want)
	}
}

// TestSelectTraitInvalidValueRejected verifies an out-of-set value is rejected with state
// unchanged: the default trait stays and the one-time flag is not raised.
func TestSelectTraitInvalidValueRejected(t *testing.T) {
	s := NewInitialState()
	for _, bad := range []string{"", "Financial", "FINANCIAL", "bogus"} {
		if _, err := s.SelectTrait(bad); !errors.Is(err, ErrInvalidTrait) {
			t.Errorf("SelectTrait(%q) err = %v, want ErrInvalidTrait", bad, err)
		}
	}
	if s.player.Trait != TraitFinancial || s.traitSelected {
		t.Errorf("state after rejections = %v/%v, want financial/not-selected", s.player.Trait, s.traitSelected)
	}
}

// TestSelectTraitAlreadySelected verifies the one-time rule: once a trait is explicitly chosen,
// any further selection (including the same value) is rejected with state unchanged.
func TestSelectTraitAlreadySelected(t *testing.T) {
	s := NewInitialState()
	if _, err := s.SelectTrait(TraitStorage); err != nil {
		t.Fatalf("first SelectTrait(storage): %v", err)
	}
	for _, trait := range []string{TraitFinancial, TraitStorage, TraitLogistics} {
		if _, err := s.SelectTrait(trait); !errors.Is(err, ErrTraitAlreadySelected) {
			t.Errorf("second SelectTrait(%q) err = %v, want ErrTraitAlreadySelected", trait, err)
		}
	}
	if s.player.Trait != TraitStorage || !s.traitSelected {
		t.Errorf("state after rejection = %v/%v, want storage/selected", s.player.Trait, s.traitSelected)
	}
}

// TestSelectTraitAfterOfficeRejected verifies the window closes at the first office selection:
// trait selection is rejected with ErrAlreadySelected and state stays unchanged.
func TestSelectTraitAfterOfficeRejected(t *testing.T) {
	s := NewInitialState()
	selectSmall(t, s)

	if _, err := s.SelectTrait(TraitLogistics); !errors.Is(err, ErrAlreadySelected) {
		t.Fatalf("SelectTrait after office selection err = %v, want ErrAlreadySelected", err)
	}
	if s.player.Trait != TraitFinancial || s.traitSelected {
		t.Errorf("state after rejection = %v/%v, want financial/not-selected", s.player.Trait, s.traitSelected)
	}
}

// TestSelectTraitGameOverRejected verifies game over takes precedence in the deterministic
// validation order: no trait selection (valid or not) is possible once the game has ended.
func TestSelectTraitGameOverRejected(t *testing.T) {
	s := NewInitialState()
	s.status = GameStatusGameOver

	if _, err := s.SelectTrait(TraitStorage); !errors.Is(err, ErrGameOver) {
		t.Fatalf("SelectTrait after game over err = %v, want ErrGameOver", err)
	}
	// Deterministic order: the game-over check precedes trait-value validation.
	if _, err := s.SelectTrait("bogus"); !errors.Is(err, ErrGameOver) {
		t.Errorf("game-over precedence err = %v, want ErrGameOver (not ErrInvalidTrait)", err)
	}
	if s.player.Trait != TraitFinancial || s.traitSelected {
		t.Errorf("state after rejection = %v/%v, want financial/not-selected", s.player.Trait, s.traitSelected)
	}
}

// TestSelectTraitPersistenceRoundTrip verifies trait_selected survives save/load: a selected
// trait and its one-time flag restore exactly; the JSON round-trip (the persistence encoding)
// preserves both; and a snapshot without the field (a pre-trait-selection save) loads as not
// explicitly selected via Go's zero value on unmarshal.
func TestSelectTraitPersistenceRoundTrip(t *testing.T) {
	s := NewInitialState()
	if _, err := s.SelectTrait(TraitLogistics); err != nil {
		t.Fatalf("SelectTrait(logistics): %v", err)
	}

	snap := s.Snapshot()
	if !snap.TraitSelected || snap.Player.Trait != TraitLogistics {
		t.Fatalf("snapshot = trait_selected %v / trait %q, want true/logistics", snap.TraitSelected, snap.Player.Trait)
	}

	restored := NewInitialState()
	if err := restored.Restore(snap); err != nil {
		t.Fatalf("Restore: %v", err)
	}
	if !restored.traitSelected || restored.player.Trait != TraitLogistics {
		t.Errorf("restored = trait_selected %v / trait %q, want true/logistics", restored.traitSelected, restored.player.Trait)
	}

	// JSON round-trip (the persistence encoding): the flag and trait survive marshal/unmarshal.
	data, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}
	var decoded Snapshot
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if !decoded.TraitSelected || decoded.Player.Trait != TraitLogistics {
		t.Errorf("decoded = trait_selected %v / trait %q, want true/logistics", decoded.TraitSelected, decoded.Player.Trait)
	}

	// A save written before trait selection existed lacks the field; it must load as not
	// explicitly selected (Go zero value on unmarshal).
	var legacy Snapshot
	if err := json.Unmarshal([]byte(`{"version":1,"player":{"id":"player-1","name":"Daniel","trait":"financial"}}`), &legacy); err != nil {
		t.Fatalf("Unmarshal legacy: %v", err)
	}
	if legacy.TraitSelected {
		t.Errorf("legacy snapshot trait_selected = true, want false (zero value)")
	}
}
