package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"staff-transport/internal/audit"
	"staff-transport/internal/trips"
)

const dateLayout = "2006-01-02"

var (
	ErrNotFound          = errors.New("recurring schedule not found")
	ErrNameRequired      = errors.New("name is required")
	ErrInvalidTripType   = errors.New("invalid trip type")
	ErrInvalidTimezone   = errors.New("invalid timezone")
	ErrInvalidRecurrence = errors.New("invalid recurrence rule")
	ErrInvalidTemplate   = errors.New("invalid schedule template")
	ErrInvalidDate       = errors.New("invalid date")
	ErrInvalidRange      = errors.New("invalid generation range")
	ErrInactive          = errors.New("schedule is inactive")
)

// Recurrence describes when a schedule generates trips. Only weekly patterns
// are supported for now.
type Recurrence struct {
	Frequency string `json:"frequency"`
	Weekdays  []int  `json:"weekdays"`
}

// TemplateStop is a stop in a schedule template.
type TemplateStop struct {
	Type    string  `json:"type"`
	Address *string `json:"address,omitempty"`
}

// TemplatePassenger is a passenger assignment in a schedule template.
type TemplatePassenger struct {
	StaffID          string `json:"staff_id"`
	PickupStopIndex  *int   `json:"pickup_stop_index,omitempty"`
	DropoffStopIndex *int   `json:"dropoff_stop_index,omitempty"`
}

// Template is the shape used to generate concrete trips.
type Template struct {
	ScheduledTime string              `json:"scheduled_time"`
	DriverID      *string             `json:"driver_id,omitempty"`
	VehicleID     *string             `json:"vehicle_id,omitempty"`
	Notes         *string             `json:"notes,omitempty"`
	Stops         []TemplateStop      `json:"stops"`
	Passengers    []TemplatePassenger `json:"passengers"`
}

// Schedule is a recurring trip definition.
type Schedule struct {
	ID         string
	Name       string
	TripType   string
	Timezone   string
	Active     bool
	StartDate  time.Time
	EndDate    *time.Time
	Recurrence Recurrence
	Template   Template
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

// CreateInput carries validated create values.
type CreateInput struct {
	Name       string
	TripType   string
	Timezone   string
	StartDate  time.Time
	EndDate    *time.Time
	Recurrence Recurrence
	Template   Template
}

// UpdateInput carries partial update values. Nil fields are unchanged.
type UpdateInput struct {
	Name       *string
	Active     *bool
	EndDate    *time.Time
	Recurrence *Recurrence
	Template   *Template
}

// Repository persists recurring schedules and generated occurrences.
type Repository interface {
	Create(ctx context.Context, in CreateInput, actorID string) (*Schedule, error)
	List(ctx context.Context, offset, limit int) ([]Schedule, int64, error)
	Get(ctx context.Context, id string) (*Schedule, error)
	Update(ctx context.Context, id string, in UpdateInput, actorID string) (*Schedule, error)
	SetActive(ctx context.Context, id string, active bool, actorID string) error
	ExistingOccurrences(ctx context.Context, scheduleID string, from, to string) (map[string]struct{}, error)
	AddOccurrence(ctx context.Context, scheduleID, tripID, occurrenceDate string) (bool, error)
	RemoveTrip(ctx context.Context, tripID string) error
}

type scheduleRow struct {
	ID             string     `gorm:"column:id"`
	Name           string     `gorm:"column:name"`
	TripType       string     `gorm:"column:trip_type"`
	Timezone       string     `gorm:"column:timezone"`
	Active         bool       `gorm:"column:active"`
	StartDate      time.Time  `gorm:"column:start_date"`
	EndDate        *time.Time `gorm:"column:end_date"`
	RecurrenceRule string     `gorm:"column:recurrence_rule"`
	Template       []byte     `gorm:"column:template"`
	CreatedAt      time.Time  `gorm:"column:created_at"`
	UpdatedAt      time.Time  `gorm:"column:updated_at"`
}

func (r *scheduleRow) toSchedule() (*Schedule, error) {
	var recurrence Recurrence
	if r.RecurrenceRule != "" {
		if err := json.Unmarshal([]byte(r.RecurrenceRule), &recurrence); err != nil {
			return nil, err
		}
	}
	var template Template
	if len(r.Template) > 0 {
		if err := json.Unmarshal(r.Template, &template); err != nil {
			return nil, err
		}
	}
	return &Schedule{
		ID:         r.ID,
		Name:       r.Name,
		TripType:   r.TripType,
		Timezone:   r.Timezone,
		Active:     r.Active,
		StartDate:  r.StartDate,
		EndDate:    r.EndDate,
		Recurrence: recurrence,
		Template:   template,
		CreatedAt:  r.CreatedAt,
		UpdatedAt:  r.UpdatedAt,
	}, nil
}

const scheduleSelect = `id, name, trip_type, timezone, active, start_date, end_date,
	recurrence_rule, template, created_at, updated_at`

type gormRepository struct {
	db *gorm.DB
}

// NewRepository returns a GORM-backed schedule repository.
func NewRepository(gdb *gorm.DB) Repository {
	return &gormRepository{db: gdb}
}

func (r *gormRepository) Create(ctx context.Context, in CreateInput, actorID string) (*Schedule, error) {
	id := uuid.NewString()
	recurrence, err := json.Marshal(in.Recurrence)
	if err != nil {
		return nil, err
	}
	template, err := json.Marshal(in.Template)
	if err != nil {
		return nil, err
	}

	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if err := tx.Table("recurring_schedules").Create(map[string]any{
			"id":              id,
			"name":            in.Name,
			"trip_type":       in.TripType,
			"timezone":        in.Timezone,
			"active":          true,
			"start_date":      in.StartDate.Format(dateLayout),
			"end_date":        dateOrNil(in.EndDate),
			"recurrence_rule": string(recurrence),
			"template":        gorm.Expr("?::jsonb", string(template)),
		}).Error; err != nil {
			return err
		}
		return audit.Record(ctx, tx, audit.Entry{
			ActorUserID: audit.StrPtr(actorID),
			Action:      "recurring_schedule.created",
			EntityType:  "recurring_schedule",
			EntityID:    &id,
			Metadata:    map[string]any{"trip_type": in.TripType, "timezone": in.Timezone},
		})
	})
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, id)
}

func (r *gormRepository) List(ctx context.Context, offset, limit int) ([]Schedule, int64, error) {
	var total int64
	if err := r.db.WithContext(ctx).Table("recurring_schedules").Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []scheduleRow
	if err := r.db.WithContext(ctx).
		Table("recurring_schedules").
		Select(scheduleSelect).
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	out := make([]Schedule, 0, len(rows))
	for i := range rows {
		item, err := rows[i].toSchedule()
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *item)
	}
	return out, total, nil
}

func (r *gormRepository) Get(ctx context.Context, id string) (*Schedule, error) {
	var row scheduleRow
	err := r.db.WithContext(ctx).
		Table("recurring_schedules").
		Select(scheduleSelect).
		Where("id = ?", id).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toSchedule()
}

func (r *gormRepository) Update(ctx context.Context, id string, in UpdateInput, actorID string) (*Schedule, error) {
	updates := map[string]any{}
	if in.Name != nil {
		updates["name"] = *in.Name
	}
	if in.Active != nil {
		updates["active"] = *in.Active
	}
	if in.EndDate != nil {
		updates["end_date"] = in.EndDate.Format(dateLayout)
	}
	if in.Recurrence != nil {
		recurrence, err := json.Marshal(*in.Recurrence)
		if err != nil {
			return nil, err
		}
		updates["recurrence_rule"] = string(recurrence)
	}
	if in.Template != nil {
		template, err := json.Marshal(*in.Template)
		if err != nil {
			return nil, err
		}
		updates["template"] = gorm.Expr("?::jsonb", string(template))
	}

	if len(updates) == 0 {
		return r.Get(ctx, id)
	}

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Table("recurring_schedules").Where("id = ?", id).Updates(updates)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		return audit.Record(ctx, tx, audit.Entry{
			ActorUserID: audit.StrPtr(actorID),
			Action:      "recurring_schedule.updated",
			EntityType:  "recurring_schedule",
			EntityID:    &id,
		})
	})
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, id)
}

func (r *gormRepository) SetActive(ctx context.Context, id string, active bool, actorID string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		result := tx.Table("recurring_schedules").Where("id = ?", id).Update("active", active)
		if result.Error != nil {
			return result.Error
		}
		if result.RowsAffected == 0 {
			return ErrNotFound
		}
		action := "recurring_schedule.deactivated"
		if active {
			action = "recurring_schedule.activated"
		}
		return audit.Record(ctx, tx, audit.Entry{
			ActorUserID: audit.StrPtr(actorID),
			Action:      action,
			EntityType:  "recurring_schedule",
			EntityID:    &id,
		})
	})
}

func (r *gormRepository) ExistingOccurrences(ctx context.Context, scheduleID, from, to string) (map[string]struct{}, error) {
	type row struct {
		OccurrenceDate time.Time `gorm:"column:occurrence_date"`
	}
	var rows []row
	if err := r.db.WithContext(ctx).
		Table("recurring_schedule_trips").
		Select("occurrence_date").
		Where("recurring_schedule_id = ? AND occurrence_date >= ? AND occurrence_date <= ?", scheduleID, from, to).
		Scan(&rows).Error; err != nil {
		return nil, err
	}
	out := make(map[string]struct{}, len(rows))
	for _, item := range rows {
		out[item.OccurrenceDate.Format(dateLayout)] = struct{}{}
	}
	return out, nil
}

func (r *gormRepository) AddOccurrence(ctx context.Context, scheduleID, tripID, occurrenceDate string) (bool, error) {
	result := r.db.WithContext(ctx).Exec(`
		INSERT INTO recurring_schedule_trips (id, recurring_schedule_id, trip_id, occurrence_date, generated_at, created_at)
		VALUES (?, ?, ?, ?::date, now(), now())
		ON CONFLICT (recurring_schedule_id, occurrence_date) DO NOTHING`,
		uuid.NewString(), scheduleID, tripID, occurrenceDate,
	)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (r *gormRepository) RemoveTrip(ctx context.Context, tripID string) error {
	return r.db.WithContext(ctx).Exec("DELETE FROM trips WHERE id = ?", tripID).Error
}

// TripCreator creates concrete trips. Implemented by the trips module.
type TripCreator interface {
	Create(ctx context.Context, in trips.CreateInput, actorID string) (*trips.Trip, error)
}

// Service expands schedules into concrete trips.
type Service struct {
	repo  Repository
	trips TripCreator
}

// NewService returns a dispatch service.
func NewService(repo Repository, tripCreator TripCreator) *Service {
	return &Service{repo: repo, trips: tripCreator}
}

// Create validates and stores a recurring schedule.
func (s *Service) Create(ctx context.Context, in CreateInput, actorID string) (*Schedule, error) {
	normalized, err := s.normalize(in.Name, in.TripType, in.Timezone, in.Recurrence, in.Template)
	if err != nil {
		return nil, err
	}
	in.Name = normalized.name
	in.TripType = normalized.tripType
	in.Timezone = normalized.timezone
	in.Recurrence = normalized.recurrence
	in.Template = normalized.template

	if in.StartDate.IsZero() {
		return nil, ErrInvalidDate
	}
	if in.EndDate != nil && in.EndDate.Before(in.StartDate) {
		return nil, ErrInvalidDate
	}
	return s.repo.Create(ctx, in, actorID)
}

// List returns a page of schedules.
func (s *Service) List(ctx context.Context, offset, limit int) ([]Schedule, int64, error) {
	return s.repo.List(ctx, offset, limit)
}

// Get loads a single schedule.
func (s *Service) Get(ctx context.Context, id string) (*Schedule, error) {
	return s.repo.Get(ctx, id)
}

// Update applies a partial update to a schedule.
func (s *Service) Update(ctx context.Context, id string, in UpdateInput, actorID string) (*Schedule, error) {
	if in.Name != nil {
		trimmed := strings.TrimSpace(*in.Name)
		if trimmed == "" {
			return nil, ErrNameRequired
		}
		in.Name = &trimmed
	}
	if in.Recurrence != nil {
		if err := validateRecurrence(*in.Recurrence); err != nil {
			return nil, err
		}
	}
	if in.Template != nil {
		if err := validateTemplate(*in.Template); err != nil {
			return nil, err
		}
	}
	return s.repo.Update(ctx, id, in, actorID)
}

// Deactivate stops a schedule from generating further trips.
func (s *Service) Deactivate(ctx context.Context, id, actorID string) error {
	return s.repo.SetActive(ctx, id, false, actorID)
}

// Generate expands a schedule into concrete trips for the inclusive date range.
// It is idempotent: occurrences already generated are skipped, and a unique
// constraint prevents duplicate generation on retry.
func (s *Service) Generate(ctx context.Context, scheduleID, actorID string, from, to time.Time) (int, error) {
	schedule, err := s.repo.Get(ctx, scheduleID)
	if err != nil {
		return 0, err
	}
	if !schedule.Active {
		return 0, ErrInactive
	}
	location, err := time.LoadLocation(schedule.Timezone)
	if err != nil {
		return 0, ErrInvalidTimezone
	}
	if to.Before(from) {
		return 0, ErrInvalidRange
	}

	fromDate := from.Format(dateLayout)
	toDate := to.Format(dateLayout)
	existing, err := s.repo.ExistingOccurrences(ctx, scheduleID, fromDate, toDate)
	if err != nil {
		return 0, err
	}

	generated := 0
	for cursor := from; !cursor.After(to); cursor = cursor.AddDate(0, 0, 1) {
		date := cursor.Format(dateLayout)
		if !withinSchedule(cursor, schedule) || !weekdayMatches(schedule.Recurrence, cursor.Weekday()) {
			continue
		}
		if _, ok := existing[date]; ok {
			continue
		}

		start, err := scheduledStart(cursor, schedule.Template.ScheduledTime, location)
		if err != nil {
			return generated, err
		}

		trip, err := s.trips.Create(ctx, tripInputFromSchedule(schedule, start), actorID)
		if err != nil {
			return generated, err
		}

		inserted, err := s.repo.AddOccurrence(ctx, scheduleID, trip.ID, date)
		if err != nil {
			return generated, err
		}
		if !inserted {
			// A concurrent run generated this occurrence first; discard ours.
			_ = s.repo.RemoveTrip(ctx, trip.ID)
			continue
		}
		existing[date] = struct{}{}
		generated++
	}
	return generated, nil
}

type normalized struct {
	name       string
	tripType   string
	timezone   string
	recurrence Recurrence
	template   Template
}

func (s *Service) normalize(name, tripType, tz string, recurrence Recurrence, template Template) (normalized, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return normalized{}, ErrNameRequired
	}
	if !trips.ValidType(tripType) {
		return normalized{}, ErrInvalidTripType
	}
	if tz == "" {
		tz = "UTC"
	}
	if _, err := time.LoadLocation(tz); err != nil {
		return normalized{}, ErrInvalidTimezone
	}
	if err := validateRecurrence(recurrence); err != nil {
		return normalized{}, err
	}
	if err := validateTemplate(template); err != nil {
		return normalized{}, err
	}
	return normalized{name: name, tripType: tripType, timezone: tz, recurrence: recurrence, template: template}, nil
}

func validateRecurrence(recurrence Recurrence) error {
	if recurrence.Frequency != "weekly" {
		return ErrInvalidRecurrence
	}
	if len(recurrence.Weekdays) == 0 {
		return ErrInvalidRecurrence
	}
	seen := map[int]struct{}{}
	for _, day := range recurrence.Weekdays {
		if day < 0 || day > 6 {
			return ErrInvalidRecurrence
		}
		if _, ok := seen[day]; ok {
			return ErrInvalidRecurrence
		}
		seen[day] = struct{}{}
	}
	return nil
}

func validateTemplate(template Template) error {
	if _, _, err := parseClock(template.ScheduledTime); err != nil {
		return ErrInvalidTemplate
	}
	if len(template.Stops) == 0 {
		return ErrInvalidTemplate
	}
	for _, stop := range template.Stops {
		if !trips.ValidStopType(stop.Type) {
			return ErrInvalidTemplate
		}
	}
	seen := map[string]struct{}{}
	for _, passenger := range template.Passengers {
		if passenger.StaffID == "" {
			return ErrInvalidTemplate
		}
		if _, ok := seen[passenger.StaffID]; ok {
			return ErrInvalidTemplate
		}
		seen[passenger.StaffID] = struct{}{}
		if !validIndex(passenger.PickupStopIndex, len(template.Stops)) || !validIndex(passenger.DropoffStopIndex, len(template.Stops)) {
			return ErrInvalidTemplate
		}
	}
	return nil
}

func tripInputFromSchedule(schedule *Schedule, start time.Time) trips.CreateInput {
	stops := make([]trips.StopInput, 0, len(schedule.Template.Stops))
	for _, stop := range schedule.Template.Stops {
		stops = append(stops, trips.StopInput{Type: stop.Type, Address: stop.Address})
	}
	passengers := make([]trips.PassengerInput, 0, len(schedule.Template.Passengers))
	for _, passenger := range schedule.Template.Passengers {
		passengers = append(passengers, trips.PassengerInput{
			StaffID:          passenger.StaffID,
			PickupStopIndex:  passenger.PickupStopIndex,
			DropoffStopIndex: passenger.DropoffStopIndex,
		})
	}
	return trips.CreateInput{
		Type:             schedule.TripType,
		ScheduledStartAt: start,
		DriverID:         schedule.Template.DriverID,
		VehicleID:        schedule.Template.VehicleID,
		Notes:            schedule.Template.Notes,
		Stops:            stops,
		Passengers:       passengers,
	}
}

func withinSchedule(date time.Time, schedule *Schedule) bool {
	value := date.Format(dateLayout)
	if value < schedule.StartDate.Format(dateLayout) {
		return false
	}
	if schedule.EndDate != nil && value > schedule.EndDate.Format(dateLayout) {
		return false
	}
	return true
}

func weekdayMatches(recurrence Recurrence, weekday time.Weekday) bool {
	for _, day := range recurrence.Weekdays {
		if day == int(weekday) {
			return true
		}
	}
	return false
}

func scheduledStart(date time.Time, clock string, location *time.Location) (time.Time, error) {
	hour, minute, err := parseClock(clock)
	if err != nil {
		return time.Time{}, err
	}
	return time.Date(date.Year(), date.Month(), date.Day(), hour, minute, 0, 0, location).UTC(), nil
}

func parseClock(value string) (int, int, error) {
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return 0, 0, fmt.Errorf("%w: scheduled_time must be HH:MM", ErrInvalidTemplate)
	}
	return parsed.Hour(), parsed.Minute(), nil
}

func validIndex(index *int, count int) bool {
	if index == nil {
		return true
	}
	return *index >= 0 && *index < count
}

func dateOrNil(value *time.Time) any {
	if value == nil {
		return nil
	}
	return value.Format(dateLayout)
}
