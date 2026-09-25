package game

import "time"

// Package lifecycle states (SPEC 5.5): stored -> assigned -> out_for_delivery ->
// delivered. The backend is authoritative for legal transitions; nothing jumps
// non-adjacent states.
const (
	PackageStored         = "stored"
	PackageAssigned       = "assigned"
	PackageOutForDelivery = "out_for_delivery"
	PackageDelivered      = "delivered"
)

// Package service types and destinations (SPEC 5.2/5.4).
const (
	ServiceNormal  = "normal"
	ServiceExpress = "express"

	DestinationLocal = "local" // MVP generates local packages only; far is DEFERRED
)

// Package is one parcel in storage or delivery. Monetary fields are integer pounds
// (SPEC 4.3). Pointer fields serialise as JSON null before they are set, matching the
// SPEC 5.5 example shape. DueAt/DeliveredAt/ReceivedAt use the canonical game-time
// format; the backend never stamps host wall-clock time.
type Package struct {
	ID                    string  `json:"id"`
	Size                  string  `json:"size"`
	ServiceType           string  `json:"service_type"`
	DestinationType       string  `json:"destination_type"`
	StorageUnits          int     `json:"storage_units"`
	DeliveryCapacityUnits int     `json:"delivery_capacity_units"`
	BaseFee               int     `json:"base_fee"`
	ReceivedAt            string  `json:"received_at"`
	DueAt                 string  `json:"due_at"`
	Status                string  `json:"status"`
	AssignedEmployeeID    *string `json:"assigned_employee_id"`
	DeliveredAt           *string `json:"delivered_at"`
	FinalRevenue          *int    `json:"final_revenue"`
}

// storageUnitsFor returns the storage consumed by a size (SPEC 5.1: small/medium 1,
// large 2). The first slice never generates large packages.
func storageUnitsFor(size string) int {
	if size == "large" {
		return 2
	}
	return 1
}

// baseFeeFor returns the known revenue for a size/service combination (SPEC 5.3).
// Large prices are OPEN in SPEC 16.2 and unreachable until office upgrades exist.
func baseFeeFor(size, service string) int {
	switch {
	case size == "small" && service == ServiceExpress:
		return 12
	case size == "medium" && service == ServiceExpress:
		return 15
	case size == "small":
		return 5
	default:
		return 7
	}
}

// deadlineDaysFor returns the delivery deadline in days after receipt (SPEC 5.2).
func deadlineDaysFor(service string) int {
	if service == ServiceExpress {
		return 2
	}
	return 5
}

// capacityUnitsFor returns the delivery-capacity consumed by a package (SPEC 5.2:
// normal 1, express 2).
func capacityUnitsFor(service string) int {
	if service == ServiceExpress {
		return 2
	}
	return 1
}

// newPackage builds a stored package received at the given game instant.
func newPackage(id, size, service string, receivedAt time.Time) *Package {
	deadline := receivedAt.AddDate(0, 0, deadlineDaysFor(service))
	return &Package{
		ID:                    id,
		Size:                  size,
		ServiceType:           service,
		DestinationType:       DestinationLocal,
		StorageUnits:          storageUnitsFor(size),
		DeliveryCapacityUnits: capacityUnitsFor(service),
		BaseFee:               baseFeeFor(size, service),
		ReceivedAt:            receivedAt.Format(GameTimeFormat),
		DueAt:                 deadline.Format(GameTimeFormat),
		Status:                PackageStored,
	}
}
