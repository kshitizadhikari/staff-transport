package events

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"staff-transport/internal/audit"
	"staff-transport/internal/staff"
)

var (
	ErrNotFound           = errors.New("event not found")
	ErrNameRequired       = errors.New("name is required")
	ErrInvalidTimeRange   = errors.New("event end must be after start")
	ErrStaffNotFound      = errors.New("staff not found")
	ErrParticipantMissing = errors.New("participant not found")
)

// Participant is a staff member taking part in an event.
type Participant struct {
	StaffID string
	Name    string
}

// EventTrip summarizes a trip serving an event.
type EventTrip struct {
	ID               string
	Type             string
	Status           string
	ScheduledStartAt time.Time
	DriverName       *string
	VehicleReg       *string
}

// Event is a transportation event such as an office outing or venue transfer.
type Event struct {
	ID           string
	Name         string
	Address      *string
	StartsAt     *time.Time
	EndsAt       *time.Time
	Notes        *string
	Participants []Participant
	Trips        []EventTrip
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// CreateInput carries validated create values.
type CreateInput struct {
	Name      string
	Address   *string
	StartsAt  *time.Time
	EndsAt    *time.Time
	Notes     *string
	Latitude  *float64
	Longitude *float64
}

// UpdateInput carries partial update values.
type UpdateInput struct {
	Name      *string
	Address   *string
	StartsAt  *time.Time
	EndsAt    *time.Time
	Notes     *string
	Latitude  *float64
	Longitude *float64
}

// Repository persists events, participants, and reads associated trips.
type Repository interface {
	List(ctx context.Context, offset, limit int) ([]Event, int64, error)
	Get(ctx context.Context, id string) (*Event, error)
	Create(ctx context.Context, in CreateInput, actorID string) (*Event, error)
	Update(ctx context.Context, id string, in UpdateInput, actorID string) (*Event, error)
	Delete(ctx context.Context, id, actorID string) error
	AddParticipant(ctx context.Context, eventID, staffID, actorID string) (bool, error)
	RemoveParticipant(ctx context.Context, eventID, staffID, actorID string) error
}

type eventRow struct {
	ID        string     `gorm:"column:id"`
	Name      string     `gorm:"column:name"`
	Address   *string    `gorm:"column:address"`
	StartsAt  *time.Time `gorm:"column:starts_at"`
	EndsAt    *time.Time `gorm:"column:ends_at"`
	Notes     *string    `gorm:"column:notes"`
	CreatedAt time.Time  `gorm:"column:created_at"`
	UpdatedAt time.Time  `gorm:"column:updated_at"`
}

func (r *eventRow) toEvent() *Event {
	return &Event{
		ID:        r.ID,
		Name:      r.Name,
		Address:   r.Address,
		StartsAt:  r.StartsAt,
		EndsAt:    r.EndsAt,
		Notes:     r.Notes,
		CreatedAt: r.CreatedAt,
		UpdatedAt: r.UpdatedAt,
	}
}

const eventSelect = `id, name, address, starts_at, ends_at, notes, created_at, updated_at`

type gormRepository struct {
	db *gorm.DB
}

// NewRepository returns a GORM-backed event repository.
func NewRepository(gdb *gorm.DB) Repository {
	return &gormRepository{db: gdb}
}

func (r *gormRepository) List(ctx context.Context, offset, limit int) ([]Event, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Table("events").Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []eventRow
	if err := r.db.WithContext(ctx).
		Table("events").
		Select(eventSelect).
		Order("COALESCE(starts_at, created_at) DESC").
		Limit(limit).
		Offset(offset).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	out := make([]Event, 0, len(rows))
	for i := range rows {
		out = append(out, *rows[i].toEvent())
	}
	return out, total, nil
}

func (r *gormRepository) Get(ctx context.Context, id string) (*Event, error) {
	var row eventRow
	err := r.db.WithContext(ctx).
		Table("events").
		Select(eventSelect).
		Where("id = ?", id).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	event := row.toEvent()
	if event.Participants, err = r.participants(ctx, id); err != nil {
		return nil, err
	}
	if event.Trips, err = r.trips(ctx, id); err != nil {
		return nil, err
	}
	return event, nil
}

func (r *gormRepository) participants(ctx context.Context, eventID string) ([]Participant, error) {
	var rows []Participant
	err := r.db.WithContext(ctx).
		Table("event_staff AS es").
		Select("es.staff_id, u.name AS name").
		Joins("JOIN staff s ON s.id = es.staff_id").
		Joins("JOIN users u ON u.id = s.user_id").
		Where("es.event_id = ?", eventID).
		Order("u.name ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *gormRepository) trips(ctx context.Context, eventID string) ([]EventTrip, error) {
	var rows []EventTrip
	err := r.db.WithContext(ctx).
		Table("trips AS t").
		Select(`t.id, t.type, t.status, t.scheduled_start_at,
			u.name AS driver_name, v.registration_number AS vehicle_reg`).
		Joins("LEFT JOIN drivers d ON d.id = t.driver_id").
		Joins("LEFT JOIN users u ON u.id = d.user_id").
		Joins("LEFT JOIN vehicles v ON v.id = t.vehicle_id").
		Where("t.event_id = ?", eventID).
		Order("t.scheduled_start_at ASC").
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

func (r *gormRepository) Create(ctx context.Context, in CreateInput, actorID string) (*Event, error) {
	id := uuid.NewString()
	row := map[string]any{
		"id":         id,
		"name":       in.Name,
		"address":    in.Address,
		"starts_at":  in.StartsAt,
		"ends_at":    in.EndsAt,
		"notes":      in.Notes,
		"point":      pointOrNil(in.Latitude, in.Longitude),
		"created_at": time.Now().UTC(),
		"updated_at": time.Now().UTC(),
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("events").Create(row).Error; err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{
			ActorUserID: audit.StrPtr(actorID),
			Action:      "event.created",
			EntityType:  "event",
			EntityID:    &id,
			Metadata:    map[string]any{"name": in.Name},
		})
	})
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, id)
}

func (r *gormRepository) Update(ctx context.Context, id string, in UpdateInput, actorID string) (*Event, error) {
	updates := map[string]any{}
	if in.Name != nil {
		updates["name"] = *in.Name
	}
	if in.Address != nil {
		updates["address"] = nullable(*in.Address)
	}
	if in.StartsAt != nil {
		updates["starts_at"] = in.StartsAt.UTC()
	}
	if in.EndsAt != nil {
		updates["ends_at"] = in.EndsAt.UTC()
	}
	if in.Notes != nil {
		updates["notes"] = nullable(*in.Notes)
	}
	if in.Latitude != nil && in.Longitude != nil {
		updates["point"] = pointOrNil(in.Latitude, in.Longitude)
	}

	if len(updates) == 0 {
		return r.Get(ctx, id)
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Table("events").Where("id = ?", id).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		return audit.Record(ctx, tx, audit.Entry{
			ActorUserID: audit.StrPtr(actorID),
			Action:      "event.updated",
			EntityType:  "event",
			EntityID:    &id,
		})
	})
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, id)
}

func (r *gormRepository) Delete(ctx context.Context, id, actorID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Exec("DELETE FROM events WHERE id = ?", id)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		return audit.Record(ctx, tx, audit.Entry{
			ActorUserID: audit.StrPtr(actorID),
			Action:      "event.deleted",
			EntityType:  "event",
			EntityID:    &id,
		})
	})
}

func (r *gormRepository) AddParticipant(ctx context.Context, eventID, staffID, actorID string) (bool, error) {
	inserted := false
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Exec(`
			INSERT INTO event_staff (id, event_id, staff_id, created_at)
			VALUES (?, ?, ?, now())
			ON CONFLICT (event_id, staff_id) DO NOTHING`,
			uuid.NewString(), eventID, staffID,
		)
		if result.Error != nil {
			return result.Error
		}
		inserted = result.RowsAffected > 0
		if !inserted {
			return nil
		}
		return audit.Record(ctx, tx, audit.Entry{
			ActorUserID: audit.StrPtr(actorID),
			Action:      "event.participant_added",
			EntityType:  "event",
			EntityID:    &eventID,
			Metadata:    map[string]any{"staff_id": staffID},
		})
	})
	return inserted, err
}

func (r *gormRepository) RemoveParticipant(ctx context.Context, eventID, staffID, actorID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Exec("DELETE FROM event_staff WHERE event_id = ? AND staff_id = ?", eventID, staffID)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return nil
		}
		return audit.Record(ctx, tx, audit.Entry{
			ActorUserID: audit.StrPtr(actorID),
			Action:      "event.participant_removed",
			EntityType:  "event",
			EntityID:    &eventID,
			Metadata:    map[string]any{"staff_id": staffID},
		})
	})
}

// StaffReader reads staff profiles for participant validation.
type StaffReader interface {
	Get(ctx context.Context, id string) (*staff.Staff, error)
}

// Service holds event business behavior.
type Service struct {
	repo  Repository
	staff StaffReader
}

// NewService returns an event service.
func NewService(repo Repository, staffSvc StaffReader) *Service {
	return &Service{repo: repo, staff: staffSvc}
}

// List returns a page of events.
func (s *Service) List(ctx context.Context, offset, limit int) ([]Event, int64, error) {
	return s.repo.List(ctx, offset, limit)
}

// Get loads a single event with participants and associated trips.
func (s *Service) Get(ctx context.Context, id string) (*Event, error) {
	return s.repo.Get(ctx, id)
}

// Create validates and stores an event.
func (s *Service) Create(ctx context.Context, in CreateInput, actorID string) (*Event, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return nil, ErrNameRequired
	}
	if err := validateTimes(in.StartsAt, in.EndsAt); err != nil {
		return nil, err
	}
	in.Address = cleanOptional(in.Address)
	in.Notes = cleanOptional(in.Notes)
	return s.repo.Create(ctx, in, actorID)
}

// Update applies a partial update to an event.
func (s *Service) Update(ctx context.Context, id string, in UpdateInput, actorID string) (*Event, error) {
	if in.Name != nil {
		trimmed := strings.TrimSpace(*in.Name)
		if trimmed == "" {
			return nil, ErrNameRequired
		}
		in.Name = &trimmed
	}
	if err := validateTimes(in.StartsAt, in.EndsAt); err != nil {
		return nil, err
	}
	in.Address = cleanOptional(in.Address)
	in.Notes = cleanOptional(in.Notes)
	return s.repo.Update(ctx, id, in, actorID)
}

// Delete removes an event. Associated trips are preserved and unlinked.
func (s *Service) Delete(ctx context.Context, id, actorID string) error {
	return s.repo.Delete(ctx, id, actorID)
}

// AddParticipant adds staff to an event. Duplicate additions are a no-op.
func (s *Service) AddParticipant(ctx context.Context, eventID, staffID, actorID string) error {
	if _, err := s.repo.Get(ctx, eventID); err != nil {
		return err
	}
	member, err := s.staff.Get(ctx, staffID)
	if errors.Is(err, staff.ErrNotFound) {
		return ErrStaffNotFound
	}
	if err != nil {
		return err
	}
	if !member.Active {
		return ErrStaffNotFound
	}
	_, err = s.repo.AddParticipant(ctx, eventID, staffID, actorID)
	return err
}

// RemoveParticipant removes staff from an event.
func (s *Service) RemoveParticipant(ctx context.Context, eventID, staffID, actorID string) error {
	if _, err := s.repo.Get(ctx, eventID); err != nil {
		return err
	}
	return s.repo.RemoveParticipant(ctx, eventID, staffID, actorID)
}

func validateTimes(startsAt, endsAt *time.Time) error {
	if startsAt != nil && endsAt != nil && endsAt.Before(*startsAt) {
		return ErrInvalidTimeRange
	}
	return nil
}

func pointOrNil(latitude, longitude *float64) any {
	if latitude == nil || longitude == nil {
		return nil
	}
	return gorm.Expr("ST_SetSRID(ST_MakePoint(?, ?), 4326)", *longitude, *latitude)
}

func cleanOptional(value *string) *string {
	if value == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return nil
	}
	return &trimmed
}

func nullable(value string) any {
	if strings.TrimSpace(value) == "" {
		return nil
	}
	return strings.TrimSpace(value)
}
