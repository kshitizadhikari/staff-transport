package trips

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"staff-transport/internal/audit"
	"staff-transport/internal/drivers"
	"staff-transport/internal/staff"
	"staff-transport/internal/users"
	"staff-transport/internal/vehicles"
)

// Trip types.
const (
	TypeStaffPickup   = "staff_pickup"
	TypeStaffDropoff  = "staff_dropoff"
	TypeOfficeToEvent = "office_to_event"
	TypeEventToHome   = "event_to_home"
	TypeCustom        = "custom"
)

// Trip states.
const (
	StatusScheduled  = "scheduled"
	StatusAssigned   = "assigned"
	StatusInProgress = "in_progress"
	StatusCompleted  = "completed"
	StatusCancelled  = "cancelled"
)

// Stop types.
const (
	StopPickup  = "pickup"
	StopDropoff = "dropoff"
	StopEvent   = "event"
	StopOffice  = "office"
	StopCustom  = "custom"
)

// Passenger states.
const (
	PassengerAssigned  = "assigned"
	PassengerPickedUp  = "picked_up"
	PassengerNoShow    = "no_show"
	PassengerCompleted = "completed"
	PassengerCancelled = "cancelled"
)

// Stop execution states.
const (
	StopPending  = "pending"
	StopArrived  = "arrived"
	StopDeparted = "departed"
	StopSkipped  = "skipped"
)

var (
	ErrNotFound             = errors.New("trip not found")
	ErrNotEditable          = errors.New("trip cannot be modified in its current state")
	ErrNotCancellable       = errors.New("trip cannot be cancelled in its current state")
	ErrInvalidType          = errors.New("invalid trip type")
	ErrInvalidStatus        = errors.New("invalid trip status")
	ErrInvalidDate          = errors.New("invalid date")
	ErrInvalidScheduledTime = errors.New("invalid scheduled time")
	ErrStopsRequired        = errors.New("at least one stop is required")
	ErrInvalidStopType      = errors.New("invalid stop type")
	ErrInvalidPassengerStop = errors.New("passenger references an invalid stop")
	ErrDuplicatePassenger   = errors.New("duplicate passenger in trip")
	ErrDriverNotFound       = errors.New("driver not found")
	ErrDriverInactive       = errors.New("driver is not active")
	ErrVehicleNotFound      = errors.New("vehicle not found")
	ErrVehicleUnavailable   = errors.New("vehicle is not available")
	ErrStaffNotFound        = errors.New("staff not found")
	ErrStaffInactive        = errors.New("staff member is not active")
	ErrCapacityExceeded     = errors.New("passenger count exceeds vehicle capacity")
	ErrEventNotFound        = errors.New("event not found")
	ErrForbidden            = errors.New("not permitted for this trip")
	ErrInvalidTransition    = errors.New("action not allowed in the trip's current state")
	ErrStopNotFound         = errors.New("stop not found on this trip")
	ErrPassengerNotFound    = errors.New("passenger not found on this trip")
)

// Stop is an ordered trip stop.
type Stop struct {
	ID          string
	Sequence    int
	Type        string
	Address     *string
	ScheduledAt *time.Time
	Status      string
}

// Passenger is a staff member assigned to a trip.
type Passenger struct {
	ID            string
	StaffID       string
	Name          string
	PickupStopID  *string
	DropoffStopID *string
	Status        string
}

// Trip is the primary operational entity.
type Trip struct {
	ID               string
	Type             string
	ScheduledStartAt time.Time
	DriverID         *string
	DriverName       *string
	VehicleID        *string
	VehicleReg       *string
	EventID          *string
	Status           string
	Notes            *string
	StartedAt        *time.Time
	CompletedAt      *time.Time
	CancelledAt      *time.Time
	CreatedAt        time.Time
	UpdatedAt        time.Time
	Stops            []Stop
	Passengers       []Passenger
}

// StopInput is an ordered stop supplied by the client.
type StopInput struct {
	Type        string
	Address     *string
	ScheduledAt *time.Time
}

// PassengerInput assigns a staff member to a trip, optionally referencing stop
// indexes within the trip's ordered stop list.
type PassengerInput struct {
	StaffID          string
	PickupStopIndex  *int
	DropoffStopIndex *int
}

// CreateInput carries validated create values.
type CreateInput struct {
	Type             string
	ScheduledStartAt time.Time
	DriverID         *string
	VehicleID        *string
	EventID          *string
	Notes            *string
	Stops            []StopInput
	Passengers       []PassengerInput
}

// UpdateInput carries partial update values.
//
// A non-nil DriverID/VehicleID/EventID/Notes sets the field; an empty string
// clears it to NULL. A nil pointer leaves the field unchanged. A non-nil
// Passengers slice replaces the passenger set.
type UpdateInput struct {
	Type             *string
	ScheduledStartAt *time.Time
	DriverID         *string
	VehicleID        *string
	EventID          *string
	Notes            *string
	Passengers       *[]PassengerInput
}

// ListFilter controls trip listing. From/To are UTC bounds [From, To).
type ListFilter struct {
	Status    string
	DriverID  string
	VehicleID string
	From      *time.Time
	To        *time.Time
	Offset    int
	Limit     int
}

// Repository persists trips, their stops, passengers, and audit records.
type Repository interface {
	List(ctx context.Context, f ListFilter) ([]Trip, int64, error)
	ListForStaff(ctx context.Context, staffID string, f ListFilter) ([]Trip, int64, error)
	Get(ctx context.Context, id string) (*Trip, error)
	Create(ctx context.Context, in CreateInput, status, actorID string) (*Trip, error)
	Update(ctx context.Context, id string, in UpdateInput, status, actorID string) (*Trip, error)
	Cancel(ctx context.Context, id, actorID string) error
	EventExists(ctx context.Context, id string) (bool, error)

	// Driver execution transitions. Each is idempotent and returns the current
	// trip. Callers must authorize that the actor is the assigned driver.
	StartTrip(ctx context.Context, tripID, actorID string) (*Trip, error)
	CompleteTrip(ctx context.Context, tripID, actorID string) (*Trip, error)
	ArriveStop(ctx context.Context, tripID, stopID, actorID string) (*Trip, error)
	DepartStop(ctx context.Context, tripID, stopID, actorID string) (*Trip, error)
	PickupPassenger(ctx context.Context, tripID, passengerID, actorID string) (*Trip, error)
	NoShowPassenger(ctx context.Context, tripID, passengerID, actorID string) (*Trip, error)
}

type tripRow struct {
	ID               string     `gorm:"column:id"`
	Type             string     `gorm:"column:type"`
	ScheduledStartAt time.Time  `gorm:"column:scheduled_start_at"`
	DriverID         *string    `gorm:"column:driver_id"`
	DriverName       *string    `gorm:"column:driver_name"`
	VehicleID        *string    `gorm:"column:vehicle_id"`
	VehicleReg       *string    `gorm:"column:vehicle_reg"`
	EventID          *string    `gorm:"column:event_id"`
	Status           string     `gorm:"column:status"`
	Notes            *string    `gorm:"column:notes"`
	StartedAt        *time.Time `gorm:"column:started_at"`
	CompletedAt      *time.Time `gorm:"column:completed_at"`
	CancelledAt      *time.Time `gorm:"column:cancelled_at"`
	CreatedAt        time.Time  `gorm:"column:created_at"`
	UpdatedAt        time.Time  `gorm:"column:updated_at"`
}

type stopRow struct {
	ID          string     `gorm:"column:id"`
	Sequence    int        `gorm:"column:sequence"`
	Type        string     `gorm:"column:stop_type"`
	Address     *string    `gorm:"column:address"`
	ScheduledAt *time.Time `gorm:"column:scheduled_at"`
	Status      string     `gorm:"column:status"`
}

type passengerRow struct {
	ID            string  `gorm:"column:id"`
	StaffID       string  `gorm:"column:staff_id"`
	Name          string  `gorm:"column:name"`
	PickupStopID  *string `gorm:"column:pickup_stop_id"`
	DropoffStopID *string `gorm:"column:dropoff_stop_id"`
	Status        string  `gorm:"column:status"`
}

func (r *tripRow) toTrip() *Trip {
	return &Trip{
		ID:               r.ID,
		Type:             r.Type,
		ScheduledStartAt: r.ScheduledStartAt,
		DriverID:         r.DriverID,
		DriverName:       r.DriverName,
		VehicleID:        r.VehicleID,
		VehicleReg:       r.VehicleReg,
		EventID:          r.EventID,
		Status:           r.Status,
		Notes:            r.Notes,
		StartedAt:        r.StartedAt,
		CompletedAt:      r.CompletedAt,
		CancelledAt:      r.CancelledAt,
		CreatedAt:        r.CreatedAt,
		UpdatedAt:        r.UpdatedAt,
	}
}

const tripSelect = `t.id, t.type, t.scheduled_start_at, t.driver_id, u.name AS driver_name,
	t.vehicle_id, v.registration_number AS vehicle_reg, t.event_id, t.status, t.notes,
	t.started_at, t.completed_at, t.cancelled_at, t.created_at, t.updated_at`

const tripJoins = `LEFT JOIN drivers d ON d.id = t.driver_id
	LEFT JOIN users u ON u.id = d.user_id
	LEFT JOIN vehicles v ON v.id = t.vehicle_id`

type gormRepository struct {
	db *gorm.DB
}

// NewRepository returns a GORM-backed trip repository.
func NewRepository(gdb *gorm.DB) Repository {
	return &gormRepository{db: gdb}
}

func (r *gormRepository) List(ctx context.Context, f ListFilter) ([]Trip, int64, error) {
	query := func() *gorm.DB {
		q := r.db.WithContext(ctx).Table("trips AS t").Joins(tripJoins)
		if f.Status != "" {
			q = q.Where("t.status = ?", f.Status)
		}
		if f.DriverID != "" {
			q = q.Where("t.driver_id = ?", f.DriverID)
		}
		if f.VehicleID != "" {
			q = q.Where("t.vehicle_id = ?", f.VehicleID)
		}
		if f.From != nil {
			q = q.Where("t.scheduled_start_at >= ?", *f.From)
		}
		if f.To != nil {
			q = q.Where("t.scheduled_start_at < ?", *f.To)
		}
		return q
	}

	var total int64
	if err := query().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []tripRow
	if err := query().
		Select(tripSelect).
		Order("t.scheduled_start_at DESC").
		Limit(f.Limit).
		Offset(f.Offset).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	out := make([]Trip, 0, len(rows))
	for i := range rows {
		out = append(out, *rows[i].toTrip())
	}
	return out, total, nil
}

func (r *gormRepository) ListForStaff(ctx context.Context, staffID string, f ListFilter) ([]Trip, int64, error) {
	query := func() *gorm.DB {
		q := r.db.WithContext(ctx).
			Table("trips AS t").
			Joins(tripJoins).
			Joins("JOIN trip_passengers tp ON tp.trip_id = t.id AND tp.staff_id = ?", staffID)
		if f.Status != "" {
			q = q.Where("t.status = ?", f.Status)
		}
		if f.From != nil {
			q = q.Where("t.scheduled_start_at >= ?", *f.From)
		}
		if f.To != nil {
			q = q.Where("t.scheduled_start_at < ?", *f.To)
		}
		return q
	}

	var total int64
	if err := query().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []tripRow
	if err := query().
		Select(tripSelect).
		Order("t.scheduled_start_at ASC").
		Limit(f.Limit).
		Offset(f.Offset).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	out := make([]Trip, 0, len(rows))
	for i := range rows {
		out = append(out, *rows[i].toTrip())
	}
	return out, total, nil
}

func (r *gormRepository) Get(ctx context.Context, id string) (*Trip, error) {
	var row tripRow
	err := r.db.WithContext(ctx).
		Table("trips AS t").
		Select(tripSelect).
		Joins(tripJoins).
		Where("t.id = ?", id).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	trip := row.toTrip()
	if trip.Stops, err = r.stops(ctx, id); err != nil {
		return nil, err
	}
	if trip.Passengers, err = r.passengers(ctx, id); err != nil {
		return nil, err
	}
	return trip, nil
}

func (r *gormRepository) stops(ctx context.Context, tripID string) ([]Stop, error) {
	var rows []stopRow
	if err := r.db.WithContext(ctx).
		Table("trip_stops").
		Select(`id, "sequence", stop_type, address, scheduled_at, status`).
		Where("trip_id = ?", tripID).
		Order(`"sequence" ASC`).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]Stop, 0, len(rows))
	for _, row := range rows {
		out = append(out, Stop{
			ID:          row.ID,
			Sequence:    row.Sequence,
			Type:        row.Type,
			Address:     row.Address,
			ScheduledAt: row.ScheduledAt,
			Status:      row.Status,
		})
	}
	return out, nil
}

func (r *gormRepository) passengers(ctx context.Context, tripID string) ([]Passenger, error) {
	var rows []passengerRow
	if err := r.db.WithContext(ctx).
		Table("trip_passengers AS tp").
		Select("tp.id, tp.staff_id, u.name AS name, tp.pickup_stop_id, tp.dropoff_stop_id, tp.status").
		Joins("JOIN staff s ON s.id = tp.staff_id").
		Joins("JOIN users u ON u.id = s.user_id").
		Where("tp.trip_id = ?", tripID).
		Order("u.name ASC").
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make([]Passenger, 0, len(rows))
	for _, row := range rows {
		out = append(out, Passenger{
			ID:            row.ID,
			StaffID:       row.StaffID,
			Name:          row.Name,
			PickupStopID:  row.PickupStopID,
			DropoffStopID: row.DropoffStopID,
			Status:        row.Status,
		})
	}
	return out, nil
}

func (r *gormRepository) EventExists(ctx context.Context, id string) (bool, error) {
	var count int64
	err := r.db.WithContext(ctx).Table("events").Where("id = ?", id).Count(&count).Error
	return count > 0, err
}

func (r *gormRepository) Create(ctx context.Context, in CreateInput, status, actorID string) (*Trip, error) {
	tripID := uuid.NewString()
	now := time.Now().UTC()
	stopIDs := make([]string, len(in.Stops))
	for i := range in.Stops {
		stopIDs[i] = uuid.NewString()
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("trips").Create(map[string]any{
			"id":                 tripID,
			"type":               in.Type,
			"scheduled_start_at": in.ScheduledStartAt.UTC(),
			"driver_id":          in.DriverID,
			"vehicle_id":         in.VehicleID,
			"event_id":           in.EventID,
			"status":             status,
			"notes":              in.Notes,
			"created_at":         now,
			"updated_at":         now,
		}).Error; err != nil {
			return err
		}

		for i, stop := range in.Stops {
			if err := tx.Table("trip_stops").Create(map[string]any{
				"id":           stopIDs[i],
				"trip_id":      tripID,
				"sequence":     i,
				"stop_type":    stop.Type,
				"address":      stop.Address,
				"scheduled_at": stop.ScheduledAt,
				"status":       StopPending,
				"created_at":   now,
				"updated_at":   now,
			}).Error; err != nil {
				return err
			}
		}

		for _, p := range in.Passengers {
			if err := tx.Table("trip_passengers").Create(map[string]any{
				"id":              uuid.NewString(),
				"trip_id":         tripID,
				"staff_id":        p.StaffID,
				"pickup_stop_id":  stopIDAt(stopIDs, p.PickupStopIndex),
				"dropoff_stop_id": stopIDAt(stopIDs, p.DropoffStopIndex),
				"status":          PassengerAssigned,
				"created_at":      now,
				"updated_at":      now,
			}).Error; err != nil {
				return err
			}
		}

		if err := audit.Record(ctx, tx, audit.Entry{
			ActorUserID: audit.StrPtr(actorID),
			Action:      "trip.created",
			EntityType:  "trip",
			EntityID:    &tripID,
			Metadata: map[string]any{
				"type":            in.Type,
				"status":          status,
				"stop_count":      len(in.Stops),
				"passenger_count": len(in.Passengers),
			},
		}); err != nil {
			return err
		}

		return r.recordAssignmentAudit(ctx, tx, tripID, actorID, nil, nil, in.DriverID, in.VehicleID)
	})
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, tripID)
}

func (r *gormRepository) Update(ctx context.Context, id string, in UpdateInput, status, actorID string) (*Trip, error) {
	existing, err := r.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()

	updates := map[string]any{"status": status, "updated_at": now}
	if in.Type != nil {
		updates["type"] = *in.Type
	}
	if in.ScheduledStartAt != nil {
		updates["scheduled_start_at"] = in.ScheduledStartAt.UTC()
	}
	if in.DriverID != nil {
		updates["driver_id"] = nullable(*in.DriverID)
	}
	if in.VehicleID != nil {
		updates["vehicle_id"] = nullable(*in.VehicleID)
	}
	if in.EventID != nil {
		updates["event_id"] = nullable(*in.EventID)
	}
	if in.Notes != nil {
		updates["notes"] = nullable(*in.Notes)
	}

	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("trips").Where("id = ?", id).Updates(updates).Error; err != nil {
			return err
		}

		if in.Passengers != nil {
			if err := r.replacePassengers(ctx, tx, id, existing, *in.Passengers, actorID, now); err != nil {
				return err
			}
		}

		if err := audit.Record(ctx, tx, audit.Entry{
			ActorUserID: audit.StrPtr(actorID),
			Action:      "trip.updated",
			EntityType:  "trip",
			EntityID:    &id,
			Metadata:    map[string]any{"status": status},
		}); err != nil {
			return err
		}

		return r.recordAssignmentAudit(ctx, tx, id, actorID, existing.DriverID, existing.VehicleID, in.DriverID, in.VehicleID)
	})
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, id)
}

func (r *gormRepository) replacePassengers(ctx context.Context, tx *gorm.DB, tripID string, existing *Trip, passengers []PassengerInput, actorID string, now time.Time) error {
	stopIDs := make([]string, len(existing.Stops))
	for i, stop := range existing.Stops {
		stopIDs[i] = stop.ID
	}

	before := make([]string, 0, len(existing.Passengers))
	for _, p := range existing.Passengers {
		before = append(before, p.StaffID)
	}

	if err := tx.Exec("DELETE FROM trip_passengers WHERE trip_id = ?", tripID).Error; err != nil {
		return err
	}
	for _, p := range passengers {
		if err := tx.Table("trip_passengers").Create(map[string]any{
			"id":              uuid.NewString(),
			"trip_id":         tripID,
			"staff_id":        p.StaffID,
			"pickup_stop_id":  stopIDAt(stopIDs, p.PickupStopIndex),
			"dropoff_stop_id": stopIDAt(stopIDs, p.DropoffStopIndex),
			"status":          PassengerAssigned,
			"created_at":      now,
			"updated_at":      now,
		}).Error; err != nil {
			return err
		}
	}

	after := make([]string, 0, len(passengers))
	for _, p := range passengers {
		after = append(after, p.StaffID)
	}

	return audit.Record(ctx, tx, audit.Entry{
		ActorUserID: audit.StrPtr(actorID),
		Action:      "trip.passengers_updated",
		EntityType:  "trip",
		EntityID:    &tripID,
		Metadata:    map[string]any{"before": before, "after": after},
	})
}

func (r *gormRepository) recordAssignmentAudit(ctx context.Context, tx *gorm.DB, tripID, actorID string, oldDriver, oldVehicle, newDriver, newVehicle *string) error {
	actor := audit.StrPtr(actorID)

	if newDriver != nil {
		old, next := deref(oldDriver), deref(newDriver)
		if old != next {
			action := "trip.driver_assigned"
			if next == "" {
				action = "trip.driver_unassigned"
			}
			if err := audit.Record(ctx, tx, audit.Entry{
				ActorUserID: actor, Action: action, EntityType: "trip", EntityID: &tripID,
				Metadata: map[string]any{"from": old, "to": next},
			}); err != nil {
				return err
			}
		}
	}

	if newVehicle != nil {
		old, next := deref(oldVehicle), deref(newVehicle)
		if old != next {
			action := "trip.vehicle_assigned"
			if next == "" {
				action = "trip.vehicle_unassigned"
			}
			if err := audit.Record(ctx, tx, audit.Entry{
				ActorUserID: actor, Action: action, EntityType: "trip", EntityID: &tripID,
				Metadata: map[string]any{"from": old, "to": next},
			}); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *gormRepository) Cancel(ctx context.Context, id, actorID string) error {
	now := time.Now().UTC()
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Table("trips").
			Where("id = ? AND status NOT IN ?", id, []string{StatusCompleted, StatusCancelled}).
			Updates(map[string]any{"status": StatusCancelled, "cancelled_at": now, "updated_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrNotCancellable
		}
		return audit.Record(ctx, tx, audit.Entry{
			ActorUserID: audit.StrPtr(actorID),
			Action:      "trip.cancelled",
			EntityType:  "trip",
			EntityID:    &id,
		})
	})
}

func (r *gormRepository) StartTrip(ctx context.Context, tripID, actorID string) (*Trip, error) {
	trip, err := r.Get(ctx, tripID)
	if err != nil {
		return nil, err
	}
	if trip.Status == StatusInProgress {
		return trip, nil
	}
	if trip.Status != StatusAssigned {
		return nil, ErrInvalidTransition
	}

	now := time.Now().UTC()
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Table("trips").
			Where("id = ? AND status = ?", tripID, StatusAssigned).
			Updates(map[string]any{"status": StatusInProgress, "started_at": now, "updated_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrInvalidTransition
		}
		return audit.Record(ctx, tx, audit.Entry{
			ActorUserID: audit.StrPtr(actorID),
			Action:      "trip.started",
			EntityType:  "trip",
			EntityID:    &tripID,
		})
	})
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, tripID)
}

func (r *gormRepository) CompleteTrip(ctx context.Context, tripID, actorID string) (*Trip, error) {
	trip, err := r.Get(ctx, tripID)
	if err != nil {
		return nil, err
	}
	if trip.Status == StatusCompleted {
		return trip, nil
	}
	if trip.Status != StatusInProgress {
		return nil, ErrInvalidTransition
	}

	now := time.Now().UTC()
	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Table("trips").
			Where("id = ? AND status = ?", tripID, StatusInProgress).
			Updates(map[string]any{"status": StatusCompleted, "completed_at": now, "updated_at": now})
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrInvalidTransition
		}
		return audit.Record(ctx, tx, audit.Entry{
			ActorUserID: audit.StrPtr(actorID),
			Action:      "trip.completed",
			EntityType:  "trip",
			EntityID:    &tripID,
		})
	})
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, tripID)
}

func (r *gormRepository) ArriveStop(ctx context.Context, tripID, stopID, actorID string) (*Trip, error) {
	return r.setStopStatus(ctx, tripID, stopID, actorID, StopArrived)
}

func (r *gormRepository) DepartStop(ctx context.Context, tripID, stopID, actorID string) (*Trip, error) {
	return r.setStopStatus(ctx, tripID, stopID, actorID, StopDeparted)
}

func (r *gormRepository) setStopStatus(ctx context.Context, tripID, stopID, actorID, status string) (*Trip, error) {
	trip, err := r.Get(ctx, tripID)
	if err != nil {
		return nil, err
	}
	if trip.Status != StatusInProgress {
		return nil, ErrInvalidTransition
	}

	stop := findStop(trip, stopID)
	if stop == nil {
		return nil, ErrStopNotFound
	}
	if stop.Status == status {
		return trip, nil
	}
	if status == StopDeparted && stop.Status != StopArrived {
		return nil, ErrInvalidTransition
	}
	if status == StopArrived && stop.Status != StopPending {
		return trip, nil
	}

	now := time.Now().UTC()
	updates := map[string]any{"status": status, "updated_at": now}
	if status == StopArrived {
		updates["arrived_at"] = now
	} else {
		updates["departed_at"] = now
	}

	res := r.db.WithContext(ctx).
		Table("trip_stops").
		Where("id = ? AND trip_id = ? AND status = ?", stopID, tripID, stop.Status).
		Updates(updates)
	if res.Error != nil {
		return nil, res.Error
	}
	return r.Get(ctx, tripID)
}

func (r *gormRepository) PickupPassenger(ctx context.Context, tripID, passengerID, actorID string) (*Trip, error) {
	return r.setPassengerStatus(ctx, tripID, passengerID, actorID, PassengerPickedUp)
}

func (r *gormRepository) NoShowPassenger(ctx context.Context, tripID, passengerID, actorID string) (*Trip, error) {
	return r.setPassengerStatus(ctx, tripID, passengerID, actorID, PassengerNoShow)
}

func (r *gormRepository) setPassengerStatus(ctx context.Context, tripID, passengerID, actorID, status string) (*Trip, error) {
	trip, err := r.Get(ctx, tripID)
	if err != nil {
		return nil, err
	}
	if trip.Status != StatusInProgress {
		return nil, ErrInvalidTransition
	}

	passenger := findPassenger(trip, passengerID)
	if passenger == nil {
		return nil, ErrPassengerNotFound
	}
	if passenger.Status == status {
		return trip, nil
	}
	if passenger.Status != PassengerAssigned {
		return nil, ErrInvalidTransition
	}

	now := time.Now().UTC()
	updates := map[string]any{"status": status, "updated_at": now}
	action := "trip.passenger_picked_up"
	if status == PassengerPickedUp {
		updates["picked_up_at"] = now
	} else {
		action = "trip.passenger_no_show"
	}

	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		res := tx.Table("trip_passengers").
			Where("id = ? AND trip_id = ? AND status = ?", passengerID, tripID, PassengerAssigned).
			Updates(updates)
		if res.Error != nil {
			return res.Error
		}
		if res.RowsAffected == 0 {
			return ErrInvalidTransition
		}
		return audit.Record(ctx, tx, audit.Entry{
			ActorUserID: audit.StrPtr(actorID),
			Action:      action,
			EntityType:  "trip",
			EntityID:    &tripID,
			Metadata:    map[string]any{"passenger_id": passengerID, "staff_id": passenger.StaffID},
		})
	})
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, tripID)
}

func findStop(trip *Trip, stopID string) *Stop {
	for i := range trip.Stops {
		if trip.Stops[i].ID == stopID {
			return &trip.Stops[i]
		}
	}
	return nil
}

func findPassenger(trip *Trip, passengerID string) *Passenger {
	for i := range trip.Passengers {
		if trip.Passengers[i].ID == passengerID {
			return &trip.Passengers[i]
		}
	}
	return nil
}

// DriverReader reads driver profiles for validation.
type DriverReader interface {
	Get(ctx context.Context, id string) (*drivers.Driver, error)
	FindByUserID(ctx context.Context, userID string) (*drivers.Driver, error)
}

// VehicleReader reads vehicle records for validation.
type VehicleReader interface {
	Get(ctx context.Context, id string) (*vehicles.Vehicle, error)
}

// StaffReader reads staff profiles for validation.
type StaffReader interface {
	Get(ctx context.Context, id string) (*staff.Staff, error)
	FindByUserID(ctx context.Context, userID string) (*staff.Staff, error)
}

// Service holds trip business behavior.
type Service struct {
	repo     Repository
	drivers  DriverReader
	vehicles VehicleReader
	staff    StaffReader
	orgTZ    string
}

// NewService returns a trip service. orgTZ is the organization timezone used
// for date filtering and user-facing scheduling.
func NewService(repo Repository, driverSvc DriverReader, vehicleSvc VehicleReader, staffSvc StaffReader, orgTZ string) *Service {
	return &Service{repo: repo, drivers: driverSvc, vehicles: vehicleSvc, staff: staffSvc, orgTZ: orgTZ}
}

// List returns a page of trips. When date is provided it is interpreted in the
// organization timezone.
func (s *Service) List(ctx context.Context, f ListFilter, date string) ([]Trip, int64, error) {
	if f.Status != "" && !ValidStatus(f.Status) {
		return nil, 0, ErrInvalidStatus
	}
	if date != "" {
		from, to, err := s.dayRange(date)
		if err != nil {
			return nil, 0, err
		}
		f.From, f.To = from, to
	}
	return s.repo.List(ctx, f)
}

// Get loads a single trip with stops and passengers.
func (s *Service) Get(ctx context.Context, id string) (*Trip, error) {
	return s.repo.Get(ctx, id)
}

// Create validates and stores a new trip.
func (s *Service) Create(ctx context.Context, in CreateInput, actorID string) (*Trip, error) {
	in.Type = strings.TrimSpace(in.Type)
	if !ValidType(in.Type) {
		return nil, ErrInvalidType
	}
	if in.ScheduledStartAt.IsZero() {
		return nil, ErrInvalidScheduledTime
	}
	if len(in.Stops) == 0 {
		return nil, ErrStopsRequired
	}
	for i := range in.Stops {
		in.Stops[i].Type = strings.TrimSpace(in.Stops[i].Type)
		if !ValidStopType(in.Stops[i].Type) {
			return nil, ErrInvalidStopType
		}
		in.Stops[i].Address = cleanOptional(in.Stops[i].Address)
	}

	in.DriverID = cleanOptional(in.DriverID)
	in.VehicleID = cleanOptional(in.VehicleID)
	in.EventID = cleanOptional(in.EventID)
	in.Notes = cleanOptional(in.Notes)

	if err := s.validatePassengers(ctx, in.Passengers, len(in.Stops)); err != nil {
		return nil, err
	}
	capacity, err := s.validateDriverVehicle(ctx, in.DriverID, in.VehicleID)
	if err != nil {
		return nil, err
	}
	if capacity > 0 && len(in.Passengers) > capacity {
		return nil, ErrCapacityExceeded
	}
	if in.EventID != nil {
		if err := s.validateEvent(ctx, *in.EventID); err != nil {
			return nil, err
		}
	}

	return s.repo.Create(ctx, in, statusFor(in.DriverID, in.VehicleID), actorID)
}

// Update applies a partial update to a trip.
func (s *Service) Update(ctx context.Context, id string, in UpdateInput, actorID string) (*Trip, error) {
	existing, err := s.repo.Get(ctx, id)
	if err != nil {
		return nil, err
	}
	if existing.Status == StatusCompleted || existing.Status == StatusCancelled {
		return nil, ErrNotEditable
	}

	if in.Type != nil {
		trimmed := strings.TrimSpace(*in.Type)
		if !ValidType(trimmed) {
			return nil, ErrInvalidType
		}
		in.Type = &trimmed
	}
	if in.ScheduledStartAt != nil && in.ScheduledStartAt.IsZero() {
		return nil, ErrInvalidScheduledTime
	}

	// Normalize provided assignment fields; empty string clears the field.
	newDriver := normalizeProvided(in.DriverID)
	newVehicle := normalizeProvided(in.VehicleID)
	if newEvent := normalizeProvided(in.EventID); in.EventID != nil && newEvent != nil {
		if err := s.validateEvent(ctx, *newEvent); err != nil {
			return nil, err
		}
	}

	capacity, err := s.validateDriverVehicle(ctx, newDriver, newVehicle)
	if err != nil {
		return nil, err
	}

	passengerCount := len(existing.Passengers)
	if in.Passengers != nil {
		if err := s.validatePassengers(ctx, *in.Passengers, len(existing.Stops)); err != nil {
			return nil, err
		}
		passengerCount = len(*in.Passengers)
	}
	if capacity > 0 && passengerCount > capacity {
		return nil, ErrCapacityExceeded
	}

	driverID := existing.DriverID
	if in.DriverID != nil {
		driverID = newDriver
	}
	vehicleID := existing.VehicleID
	if in.VehicleID != nil {
		vehicleID = newVehicle
	}

	return s.repo.Update(ctx, id, in, statusFor(driverID, vehicleID), actorID)
}

// Cancel cancels a trip unless it is already completed or cancelled.
func (s *Service) Cancel(ctx context.Context, id, actorID string) error {
	existing, err := s.repo.Get(ctx, id)
	if err != nil {
		return err
	}
	if existing.Status == StatusCompleted {
		return ErrNotCancellable
	}
	if existing.Status == StatusCancelled {
		return nil
	}
	return s.repo.Cancel(ctx, id, actorID)
}

// MyTrips returns the caller's own transportation: trips assigned to them as a
// driver, or trips they are a passenger on as a staff member.
func (s *Service) MyTrips(ctx context.Context, userID, role string, f ListFilter, date string) ([]Trip, int64, error) {
	if f.Status != "" && !ValidStatus(f.Status) {
		return nil, 0, ErrInvalidStatus
	}
	if date != "" {
		from, to, err := s.dayRange(date)
		if err != nil {
			return nil, 0, err
		}
		f.From, f.To = from, to
	}

	switch role {
	case string(users.RoleDriver):
		driver, err := s.drivers.FindByUserID(ctx, userID)
		if errors.Is(err, drivers.ErrNotFound) {
			return []Trip{}, 0, nil
		}
		if err != nil {
			return nil, 0, err
		}
		f.DriverID = driver.ID
		return s.repo.List(ctx, f)
	case string(users.RoleStaff):
		member, err := s.staff.FindByUserID(ctx, userID)
		if errors.Is(err, staff.ErrNotFound) {
			return []Trip{}, 0, nil
		}
		if err != nil {
			return nil, 0, err
		}
		return s.repo.ListForStaff(ctx, member.ID, f)
	default:
		return nil, 0, ErrForbidden
	}
}

// StartTrip begins execution of a trip the caller is assigned to.
func (s *Service) StartTrip(ctx context.Context, tripID, userID string) (*Trip, error) {
	if err := s.authorizeDriver(ctx, tripID, userID); err != nil {
		return nil, err
	}
	return s.repo.StartTrip(ctx, tripID, userID)
}

// CompleteTrip finishes execution of a trip the caller is assigned to.
func (s *Service) CompleteTrip(ctx context.Context, tripID, userID string) (*Trip, error) {
	if err := s.authorizeDriver(ctx, tripID, userID); err != nil {
		return nil, err
	}
	return s.repo.CompleteTrip(ctx, tripID, userID)
}

// ArriveStop records the caller's arrival at a stop.
func (s *Service) ArriveStop(ctx context.Context, tripID, stopID, userID string) (*Trip, error) {
	if err := s.authorizeDriver(ctx, tripID, userID); err != nil {
		return nil, err
	}
	return s.repo.ArriveStop(ctx, tripID, stopID, userID)
}

// DepartStop records the caller's departure from a stop.
func (s *Service) DepartStop(ctx context.Context, tripID, stopID, userID string) (*Trip, error) {
	if err := s.authorizeDriver(ctx, tripID, userID); err != nil {
		return nil, err
	}
	return s.repo.DepartStop(ctx, tripID, stopID, userID)
}

// PickupPassenger marks a passenger as picked up.
func (s *Service) PickupPassenger(ctx context.Context, tripID, passengerID, userID string) (*Trip, error) {
	if err := s.authorizeDriver(ctx, tripID, userID); err != nil {
		return nil, err
	}
	return s.repo.PickupPassenger(ctx, tripID, passengerID, userID)
}

// NoShowPassenger marks a passenger as a no-show.
func (s *Service) NoShowPassenger(ctx context.Context, tripID, passengerID, userID string) (*Trip, error) {
	if err := s.authorizeDriver(ctx, tripID, userID); err != nil {
		return nil, err
	}
	return s.repo.NoShowPassenger(ctx, tripID, passengerID, userID)
}

func (s *Service) authorizeDriver(ctx context.Context, tripID, userID string) error {
	trip, err := s.repo.Get(ctx, tripID)
	if err != nil {
		return err
	}
	driver, err := s.drivers.FindByUserID(ctx, userID)
	if errors.Is(err, drivers.ErrNotFound) {
		return ErrForbidden
	}
	if err != nil {
		return err
	}
	if trip.DriverID == nil || *trip.DriverID != driver.ID {
		return ErrForbidden
	}
	return nil
}

func (s *Service) validatePassengers(ctx context.Context, passengers []PassengerInput, stopCount int) error {
	seen := make(map[string]struct{}, len(passengers))
	for _, p := range passengers {
		if p.StaffID == "" {
			return ErrStaffNotFound
		}
		if _, ok := seen[p.StaffID]; ok {
			return ErrDuplicatePassenger
		}
		seen[p.StaffID] = struct{}{}
		if !validStopIndex(p.PickupStopIndex, stopCount) || !validStopIndex(p.DropoffStopIndex, stopCount) {
			return ErrInvalidPassengerStop
		}
		st, err := s.staff.Get(ctx, p.StaffID)
		if errors.Is(err, staff.ErrNotFound) {
			return ErrStaffNotFound
		}
		if err != nil {
			return err
		}
		if !st.Active {
			return ErrStaffInactive
		}
	}
	return nil
}

func (s *Service) validateDriverVehicle(ctx context.Context, driverID, vehicleID *string) (int, error) {
	capacity := 0
	if vehicleID != nil {
		v, err := s.vehicles.Get(ctx, *vehicleID)
		if errors.Is(err, vehicles.ErrNotFound) {
			return 0, ErrVehicleNotFound
		}
		if err != nil {
			return 0, err
		}
		if v.Status == vehicles.StatusInactive || v.Status == vehicles.StatusMaintenance {
			return 0, ErrVehicleUnavailable
		}
		capacity = v.Capacity
	}
	if driverID != nil {
		d, err := s.drivers.Get(ctx, *driverID)
		if errors.Is(err, drivers.ErrNotFound) {
			return 0, ErrDriverNotFound
		}
		if err != nil {
			return 0, err
		}
		if d.Status == drivers.StatusInactive || d.Status == drivers.StatusLeave {
			return 0, ErrDriverInactive
		}
	}
	return capacity, nil
}

func (s *Service) validateEvent(ctx context.Context, id string) error {
	exists, err := s.repo.EventExists(ctx, id)
	if err != nil {
		return err
	}
	if !exists {
		return ErrEventNotFound
	}
	return nil
}

func (s *Service) dayRange(date string) (*time.Time, *time.Time, error) {
	loc, err := time.LoadLocation(s.orgTZ)
	if err != nil {
		return nil, nil, fmt.Errorf("load org timezone: %w", err)
	}
	day, err := time.ParseInLocation("2006-01-02", date, loc)
	if err != nil {
		return nil, nil, ErrInvalidDate
	}
	start := day.UTC()
	end := day.AddDate(0, 0, 1).UTC()
	return &start, &end, nil
}

// ValidType reports whether t is a recognized trip type.
func ValidType(t string) bool {
	switch t {
	case TypeStaffPickup, TypeStaffDropoff, TypeOfficeToEvent, TypeEventToHome, TypeCustom:
		return true
	default:
		return false
	}
}

// ValidStatus reports whether s is a recognized trip status.
func ValidStatus(s string) bool {
	switch s {
	case StatusScheduled, StatusAssigned, StatusInProgress, StatusCompleted, StatusCancelled:
		return true
	default:
		return false
	}
}

// ValidStopType reports whether t is a recognized stop type.
func ValidStopType(t string) bool {
	switch t {
	case StopPickup, StopDropoff, StopEvent, StopOffice, StopCustom:
		return true
	default:
		return false
	}
}

func statusFor(driverID, vehicleID *string) string {
	if driverID != nil && vehicleID != nil {
		return StatusAssigned
	}
	return StatusScheduled
}

func normalizeProvided(v *string) *string {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func validStopIndex(index *int, stopCount int) bool {
	if index == nil {
		return true
	}
	return *index >= 0 && *index < stopCount
}

func stopIDAt(stopIDs []string, index *int) any {
	if index == nil {
		return nil
	}
	return stopIDs[*index]
}

func cleanOptional(v *string) *string {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func nullable(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return strings.TrimSpace(v)
}

func deref(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
