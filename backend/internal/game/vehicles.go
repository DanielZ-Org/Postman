package game

import (
	"errors"
	"fmt"
)

// Vehicle-layer operation errors. The API layer maps each to its canonical machine
// error code and HTTP status; every failed purchase or mode switch leaves state
// unchanged.
var (
	ErrUnknownVehicle         = errors.New("unknown vehicle type") // INVALID_VEHICLE 400
	ErrVehicleCapacityReached = errors.New("vehicle capacity reached")
)

// VehicleCapacityError reports that the office's per-type vehicle limit is already
// reached (VEHICLE_CAPACITY_REACHED 409). It carries the limit and owned count for
// API error details.
type VehicleCapacityError struct {
	Vehicle string
	Limit   int
	Owned   int
}

func (e *VehicleCapacityError) Error() string {
	return fmt.Sprintf("vehicle capacity reached: %d of %d %s owned", e.Owned, e.Limit, e.Vehicle)
}

// validVehicles is the canonical set of purchasable vehicle types in deterministic
// order. It drives both validation and the INVALID_VEHICLE error details, so the
// wire contract never drifts from the accepted values.
var validVehicles = []string{"bicycle", "car"}

// ValidVehicles returns a copy of the allowed vehicle types in canonical order (for
// API error details). Callers cannot mutate the canonical set through the returned
// slice.
func ValidVehicles() []string {
	return append([]string(nil), validVehicles...)
}

// isAllowedVehicle reports whether vehicle is one of the purchasable types.
func isAllowedVehicle(vehicle string) bool {
	for _, v := range validVehicles {
		if v == vehicle {
			return true
		}
	}
	return false
}

// VehiclesView is the read-only vehicle projection for GET /api/v1/vehicles: owned
// counts plus the selected office's per-type capacity limits (zero when no active
// head office exists, mirroring how /office reports nothing before selection).
type VehiclesView struct {
	BicyclesOwned   int `json:"bicycles_owned"`
	CarsOwned       int `json:"cars_owned"`
	BicycleCapacity int `json:"bicycle_capacity"`
	VehicleCapacity int `json:"vehicle_capacity"`
}

// vehiclesViewLocked assembles the vehicle projection; the caller must hold s.mu.
func (s *GameState) vehiclesViewLocked() VehiclesView {
	v := VehiclesView{BicyclesOwned: s.bicyclesOwned, CarsOwned: s.carsOwned}
	if office := s.selectedOffice; office != nil && office.ContractStatus == ContractActive {
		v.BicycleCapacity = office.BicycleCapacity
		v.VehicleCapacity = office.VehicleCapacity
	}
	return v
}

// VehiclesView returns the read-only vehicle projection. Read-only: it never mutates
// state.
func (s *GameState) VehiclesView() VehiclesView {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.vehiclesViewLocked()
}

// PurchaseVehicle validates and commits one vehicle purchase under s.mu in the
// deterministic validation order required by the approved plan section 3.3:
// (1) game over; (2) active head office exists; (3) per-type capacity limit;
// (4) sufficient cash for the purchase price. On success it increments the owned
// count, deducts the price and appends exactly one vehicle_purchase transaction —
// all committed together so a failed validation never leaves partial state. The
// price is the decided constant in rules.go (SPEC 12/16.7).
func (s *GameState) PurchaseVehicle(vehicle string) (VehiclesView, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == GameStatusGameOver {
		return VehiclesView{}, ErrGameOver
	}
	office := s.selectedOffice
	if office == nil || office.ContractStatus != ContractActive {
		return VehiclesView{}, ErrNoOffice
	}

	var owned *int
	var limit, price int
	switch vehicle {
	case "bicycle":
		owned = &s.bicyclesOwned
		limit = office.BicycleCapacity
		price = bicyclePurchasePrice
	case "car":
		owned = &s.carsOwned
		limit = office.VehicleCapacity
		price = carPurchasePrice
	default:
		// The API layer rejects unknown types before this point (strict request
		// validation); the defensive branch keeps the game layer safe on its own.
		return VehiclesView{}, ErrUnknownVehicle
	}

	if *owned >= limit {
		return VehiclesView{}, &VehicleCapacityError{Vehicle: vehicle, Limit: limit, Owned: *owned}
	}
	if s.cash < price {
		return VehiclesView{}, &InsufficientFundsError{Required: price, Available: s.cash}
	}

	now := s.Clock.Now()
	s.postTransactionLocked(now, CategoryVehiclePurchase, -price, vehicle+" purchase", vehicle)
	*owned++
	return s.vehiclesViewLocked(), nil
}

// ErrInvalidMode is returned by SetEmployeeMode for a mode outside the allowed set;
// the API layer rejects unknown values before this point (strict request
// validation), so it is defensive on its own.
var ErrInvalidMode = errors.New("invalid delivery mode")

// MissingSkillError reports that an employee lacks the skill required for a non-foot
// delivery mode (MISSING_SKILL 409). It carries the employee id and the missing
// skill for API error details.
type MissingSkillError struct {
	EmployeeID    string
	RequiredSkill string
}

func (e *MissingSkillError) Error() string {
	return fmt.Sprintf("employee %s lacks required skill %s", e.EmployeeID, e.RequiredSkill)
}

// validModes is the canonical set of delivery modes in deterministic order. It drives
// both validation and the INVALID_MODE error details.
var validModes = []string{ModeFoot, ModeBicycle, ModeCar}

// ValidModes returns a copy of the allowed delivery modes in canonical order (for API
// error details). Callers cannot mutate the canonical set through the returned slice.
func ValidModes() []string {
	return append([]string(nil), validModes...)
}

// isAllowedMode reports whether mode is one of foot/bicycle/car.
func isAllowedMode(mode string) bool {
	for _, m := range validModes {
		if m == mode {
			return true
		}
	}
	return false
}

// requiredSkillForMode returns the skill a non-foot delivery mode needs; foot always
// works without one (SPEC 7.2).
func requiredSkillForMode(mode string) string {
	switch mode {
	case ModeBicycle:
		return SkillBicycle
	case ModeCar:
		return SkillDrivingLicence
	default:
		return ""
	}
}

// hasSkill reports whether the employee possesses skill (SPEC 7.2 extensible list).
func hasSkill(emp *Employee, skill string) bool {
	for _, s := range emp.Skills {
		if s == skill {
			return true
		}
	}
	return false
}

// SetEmployeeMode validates and commits one delivery-mode switch under s.mu in the
// deterministic validation order required by the approved plan section 3.3:
// (1) game over; (2) employee exists; (3) mode value valid; (4) the mode's skill is
// present for non-foot modes; (5) the employee is ready, not mid-run. Foot is always
// allowed regardless of skills. On success it sets current_delivery_mode and returns
// the updated employee; every failed validation leaves state unchanged.
func (s *GameState) SetEmployeeMode(employeeID string, mode string) (*Employee, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.status == GameStatusGameOver {
		return nil, ErrGameOver
	}
	emp := s.findEmployeeLocked(employeeID)
	if emp == nil {
		return nil, ErrEmployeeNotFound
	}
	if !isAllowedMode(mode) {
		return nil, ErrInvalidMode
	}
	if mode != ModeFoot {
		required := requiredSkillForMode(mode)
		if !hasSkill(emp, required) {
			return nil, &MissingSkillError{EmployeeID: emp.ID, RequiredSkill: required}
		}
	}
	if emp.Status != EmployeeReady {
		return nil, ErrEmployeeBusy
	}

	emp.CurrentDeliveryMode = mode
	return emp, nil
}
