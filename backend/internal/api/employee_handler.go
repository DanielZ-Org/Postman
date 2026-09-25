package api

import (
	"errors"
	"net/http"

	"github.com/DanielZ-Org/Postman/backend/internal/game"
)

// employeeHandler serves GET /api/v1/employees (SPEC 7.5/8) and POST
// /api/v1/employees/hire (SPEC 8).
type employeeHandler struct {
	state *game.GameState
}

func newEmployeeHandler(state *game.GameState) *employeeHandler {
	return &employeeHandler{state: state}
}

// employeeJSON is the wire shape for one employee (SPEC 7.5 example).
type employeeJSON struct {
	ID                        string   `json:"id"`
	Name                      string   `json:"name"`
	SpeedTrait                string   `json:"speed_trait"`
	Skills                    []string `json:"skills"`
	Mood                      string   `json:"mood"`
	CurrentDeliveryMode       string   `json:"current_delivery_mode"`
	PackagesDeliveredThisWeek int      `json:"packages_delivered_this_week"`
	AccruedWages              int      `json:"accrued_wages"`
	Status                    string   `json:"status"`
	RunsToday                 int      `json:"runs_today"`
}

func employeeFrom(e game.Employee) employeeJSON {
	return employeeJSON{
		ID:                        e.ID,
		Name:                      e.Name,
		SpeedTrait:                e.SpeedTrait,
		Skills:                    e.Skills,
		Mood:                      e.Mood,
		CurrentDeliveryMode:       e.CurrentDeliveryMode,
		PackagesDeliveredThisWeek: e.PackagesDeliveredThisWeek,
		AccruedWages:              e.AccruedWages,
		Status:                    e.Status,
		RunsToday:                 e.RunsToday,
	}
}

// hiringJSON is the wire shape for the SPEC 8 hiring state.
type hiringJSON struct {
	CurrentEmployeeCount int `json:"current_employee_count"`
	TotalHiresLifetime   int `json:"total_hires_lifetime"`
	NextHiringFee        int `json:"next_hiring_fee"`
}

// employeesResponse wraps the employee list plus hiring state (the shape the
// frontend client parses).
type employeesResponse struct {
	Employees []employeeJSON `json:"employees"`
	Hiring    hiringJSON     `json:"hiring"`
}

// hireResponse is the successful hire payload: the new employee plus updated
// hiring state.
type hireResponse struct {
	Employee employeeJSON `json:"employee"`
	Hiring   hiringJSON   `json:"hiring"`
}

// handleList serves GET /api/v1/employees with the employee list and hiring state.
func (h *employeeHandler) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	employees, hiring := h.state.EmployeesView()
	resp := employeesResponse{
		Employees: make([]employeeJSON, 0, len(employees)),
		Hiring: hiringJSON{
			CurrentEmployeeCount: hiring.CurrentEmployeeCount,
			TotalHiresLifetime:   hiring.TotalHiresLifetime,
			NextHiringFee:        hiring.NextHiringFee,
		},
	}
	for _, e := range employees {
		resp.Employees = append(resp.Employees, employeeFrom(e))
	}
	writeJSON(w, http.StatusOK, resp)
}

// handleHire serves POST /api/v1/employees/hire with an empty body ({} or
// zero-length). Business rules live in the game layer; this maps its errors to
// canonical JSON codes.
func (h *employeeHandler) handleHire(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeMethodNotAllowed(w)
		return
	}

	body, code := readBody(r, true) // empty body accepted (no fields are sent)
	if code != "" {
		writeAPIError(w, http.StatusBadRequest, code, "request body must be a single valid JSON value", nil)
		return
	}
	if len(body) > 0 {
		obj, ok := decodeObject(body)
		if !ok {
			writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "request body must be an empty JSON object {}", nil)
			return
		}
		for k := range obj {
			writeAPIError(w, http.StatusBadRequest, "INVALID_REQUEST", "hire request must not contain fields", map[string]any{"field": k})
			return
		}
	}

	emp, hiring, err := h.state.HireEmployee()
	if err != nil {
		h.writeHireError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, hireResponse{
		Employee: employeeFrom(*emp),
		Hiring: hiringJSON{
			CurrentEmployeeCount: hiring.CurrentEmployeeCount,
			TotalHiresLifetime:   hiring.TotalHiresLifetime,
			NextHiringFee:        hiring.NextHiringFee,
		},
	})
}

// writeHireError maps hire failures to canonical codes (SPEC 14.1 envelope).
func (h *employeeHandler) writeHireError(w http.ResponseWriter, err error) {
	switch {
	case err == game.ErrGameOver:
		writeAPIError(w, http.StatusConflict, "GAME_OVER", "the game has ended", nil)
	case err == game.ErrNoOffice:
		writeAPIError(w, http.StatusConflict, "NO_OFFICE", "select a head office before hiring", nil)
	case err == game.ErrOfficeFull:
		writeAPIError(w, http.StatusConflict, "OFFICE_FULL", "employee capacity reached", nil)
	default:
		var ife *game.InsufficientFundsError
		if errors.As(err, &ife) {
			writeAPIError(w, http.StatusConflict, "INSUFFICIENT_FUNDS", "not enough cash for the hiring fee",
				map[string]any{"required": ife.Required, "available": ife.Available})
			return
		}
		writeAPIError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "unexpected hiring failure", nil)
	}
}

// packageHandler serves GET /api/v1/packages (SPEC 5.5).
type packageHandler struct {
	state *game.GameState
}

func newPackageHandler(state *game.GameState) *packageHandler {
	return &packageHandler{state: state}
}

// packagesResponse wraps the package list (the shape the frontend client parses).
type packagesResponse struct {
	Packages []game.Package `json:"packages"`
}

// handleList serves GET /api/v1/packages with every package (delivered included, so
// clients can show history).
func (h *packageHandler) handleList(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		writeMethodNotAllowed(w)
		return
	}
	pkgs := h.state.PackagesView()
	if pkgs == nil {
		pkgs = []game.Package{}
	}
	writeJSON(w, http.StatusOK, packagesResponse{Packages: pkgs})
}
