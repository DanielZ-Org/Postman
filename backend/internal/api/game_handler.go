package api

import (
	"net/http"

	"github.com/DanielZ-Org/Postman/backend/internal/game"
)

// gameHandler serves the read-only game projections: GET /api/v1/game (SPEC 14.2),
// GET /api/v1/player (SPEC 3) and GET /api/v1/office (SPEC 4.1 runtime office). All
// three are read-only: they never seed or mutate state.
type gameHandler struct {
	state *game.GameState
}

func newGameHandler(state *game.GameState) *gameHandler {
	return &gameHandler{state: state}
}

// gameStateBody is the /game projection nested under the canonical "game-state"
// wrapper the frontend consumes (SPEC 14.2).
type gameStateBody struct {
	Game       gameStateGame    `json:"game"`
	Player     gameStatePlayer  `json:"player"`
	Office     *gameStateOffice `json:"office"`
	Operations gameStateOps     `json:"operations"`
	Finance    gameStateFinance `json:"finance"`
}

type gameStateGame struct {
	Status       string `json:"status"`
	GameDatetime string `json:"game_datetime"`
	Speed        int    `json:"speed"`
}

type gameStatePlayer struct {
	ID    string `json:"id"`
	Cash  int    `json:"cash"`
	Trait string `json:"trait"`
}

type gameStateOffice struct {
	ID               string `json:"id"`
	StorageUsed      int    `json:"storage_used"`
	StorageCapacity  int    `json:"storage_capacity"`
	EmployeeCount    int    `json:"employee_count"`
	EmployeeCapacity int    `json:"employee_capacity"`
}

type gameStateOps struct {
	StoredPackages int `json:"stored_packages"`
	OutForDelivery int `json:"out_for_delivery"`
	DeliveredToday int `json:"delivered_today"`
}

type gameStateFinance struct {
	AccruedWages  int `json:"accrued_wages"`
	NextRent      int `json:"next_rent"`
	LoanPrincipal int `json:"loan_principal"`
}

// gameOverStateResponse wraps the projection under the canonical "game-state" key.
type gameOverStateResponse struct {
	GameState gameStateBody `json:"game-state"`
}

// playerResponse wraps the SPEC 3 player object under a "player" key, mirroring the
// clock/offices wrapper convention.
type playerResponse struct {
	Player game.PlayerView `json:"player"`
}

// officeResponse wraps the selected runtime office under an "office" key; the value
// is null before an office is selected.
type officeResponse struct {
	Office *runtimeOfficeJSON `json:"office"`
}

// handleGame serves GET /api/v1/game with the SPEC 14.2 "game-state" wrapper.
func (h *gameHandler) handleGame(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	v := h.state.GameView()
	body := gameStateBody{
		Game:   gameStateGame{Status: v.Status, GameDatetime: v.GameDatetime, Speed: v.Speed},
		Player: gameStatePlayer{ID: v.PlayerID, Cash: v.Cash, Trait: v.Trait},
		Operations: gameStateOps{
			StoredPackages: v.StoredPackages,
			OutForDelivery: v.OutForDelivery,
			DeliveredToday: v.DeliveredToday,
		},
		Finance: gameStateFinance{
			AccruedWages:  v.AccruedWages,
			NextRent:      v.NextRent,
			LoanPrincipal: v.LoanPrincipal,
		},
	}
	if v.HasOffice {
		body.Office = &gameStateOffice{
			ID:               v.OfficeID,
			StorageUsed:      v.StorageUsed,
			StorageCapacity:  v.StorageCapacity,
			EmployeeCount:    v.EmployeeCount,
			EmployeeCapacity: v.EmployeeCapacity,
		}
	}
	writeJSON(w, http.StatusOK, gameOverStateResponse{GameState: body})
}

// handlePlayer serves GET /api/v1/player with the SPEC 3 player object.
func (h *gameHandler) handlePlayer(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	writeJSON(w, http.StatusOK, playerResponse{Player: h.state.PlayerView()})
}

// handleOffice serves GET /api/v1/office with the selected runtime office (SPEC 4.1
// representation) or null before a selection. A terminated contract is returned with
// its contract_status so clients can see it ended.
func (h *gameHandler) handleOffice(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	resp := officeResponse{}
	if office := h.state.SelectedOfficeView(); office != nil {
		json := runtimeFrom(office)
		resp.Office = &json
	}
	writeJSON(w, http.StatusOK, resp)
}
