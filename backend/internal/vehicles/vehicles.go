package vehicles

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"staff-transport/internal/db"
)

// Vehicle statuses.
const (
	StatusAvailable   = "available"
	StatusAssigned    = "assigned"
	StatusMaintenance = "maintenance"
	StatusInactive    = "inactive"
)

var (
	ErrNotFound             = errors.New("vehicle not found")
	ErrRegistrationRequired = errors.New("registration number is required")
	ErrRegistrationTaken    = errors.New("registration number already in use")
	ErrInvalidStatus        = errors.New("invalid vehicle status")
	ErrCapacityInvalid      = errors.New("capacity must not be negative")
)

// Vehicle is a fleet vehicle record.
type Vehicle struct {
	ID                 string
	RegistrationNumber string
	Model              *string
	Capacity           int
	Status             string
	CreatedAt          time.Time
	UpdatedAt          time.Time
}

// CreateInput carries validated create values to the repository.
type CreateInput struct {
	RegistrationNumber string
	Model              *string
	Capacity           int
	Status             string
}

// UpdateInput carries partial update values. A nil field is left unchanged.
type UpdateInput struct {
	RegistrationNumber *string
	Model              *string
	Capacity           *int
	Status             *string
}

// ListFilter controls vehicle listing.
type ListFilter struct {
	Search string
	Status string
	Offset int
	Limit  int
}

// Repository persists vehicle records.
type Repository interface {
	List(ctx context.Context, f ListFilter) ([]Vehicle, int64, error)
	Get(ctx context.Context, id string) (*Vehicle, error)
	Create(ctx context.Context, in CreateInput) (*Vehicle, error)
	Update(ctx context.Context, id string, in UpdateInput) (*Vehicle, error)
}

type vehicleRow struct {
	ID                 string    `gorm:"column:id"`
	RegistrationNumber string    `gorm:"column:registration_number"`
	Model              *string   `gorm:"column:model"`
	Capacity           int       `gorm:"column:capacity"`
	Status             string    `gorm:"column:status"`
	CreatedAt          time.Time `gorm:"column:created_at"`
	UpdatedAt          time.Time `gorm:"column:updated_at"`
}

func (r *vehicleRow) toVehicle() *Vehicle {
	return &Vehicle{
		ID:                 r.ID,
		RegistrationNumber: r.RegistrationNumber,
		Model:              r.Model,
		Capacity:           r.Capacity,
		Status:             r.Status,
		CreatedAt:          r.CreatedAt,
		UpdatedAt:          r.UpdatedAt,
	}
}

type gormRepository struct {
	db *gorm.DB
}

// NewRepository returns a GORM-backed vehicle repository.
func NewRepository(gdb *gorm.DB) Repository {
	return &gormRepository{db: gdb}
}

func (r *gormRepository) List(ctx context.Context, f ListFilter) ([]Vehicle, int64, error) {
	query := func() *gorm.DB {
		q := r.db.WithContext(ctx).Table("vehicles")
		if f.Search != "" {
			like := "%" + f.Search + "%"
			q = q.Where("registration_number ILIKE ? OR model ILIKE ?", like, like)
		}
		if f.Status != "" {
			q = q.Where("status = ?", f.Status)
		}
		return q
	}

	var total int64
	if err := query().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []vehicleRow
	if err := query().
		Order("registration_number ASC").
		Limit(f.Limit).
		Offset(f.Offset).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	out := make([]Vehicle, 0, len(rows))
	for i := range rows {
		out = append(out, *rows[i].toVehicle())
	}
	return out, total, nil
}

func (r *gormRepository) Get(ctx context.Context, id string) (*Vehicle, error) {
	var row vehicleRow
	err := r.db.WithContext(ctx).
		Table("vehicles").
		Where("id = ?", id).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toVehicle(), nil
}

func (r *gormRepository) Create(ctx context.Context, in CreateInput) (*Vehicle, error) {
	id := uuid.NewString()
	now := time.Now().UTC()

	row := map[string]any{
		"id":                  id,
		"registration_number": in.RegistrationNumber,
		"model":               in.Model,
		"capacity":            in.Capacity,
		"status":              in.Status,
		"created_at":          now,
		"updated_at":          now,
	}
	if err := r.db.WithContext(ctx).Table("vehicles").Create(row).Error; err != nil {
		return nil, err
	}
	return r.Get(ctx, id)
}

func (r *gormRepository) Update(ctx context.Context, id string, in UpdateInput) (*Vehicle, error) {
	updates := map[string]any{}
	if in.RegistrationNumber != nil {
		updates["registration_number"] = *in.RegistrationNumber
	}
	if in.Model != nil {
		updates["model"] = nullable(*in.Model)
	}
	if in.Capacity != nil {
		updates["capacity"] = *in.Capacity
	}
	if in.Status != nil {
		updates["status"] = *in.Status
	}

	if len(updates) == 0 {
		return r.Get(ctx, id)
	}

	res := r.db.WithContext(ctx).Table("vehicles").Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return nil, ErrNotFound
	}
	return r.Get(ctx, id)
}

// Service holds vehicle business behavior.
type Service struct {
	repo Repository
}

// NewService returns a vehicle service backed by repo.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// List returns a page of vehicles.
func (s *Service) List(ctx context.Context, f ListFilter) ([]Vehicle, int64, error) {
	f.Search = strings.TrimSpace(f.Search)
	if f.Status != "" && !ValidStatus(f.Status) {
		return nil, 0, ErrInvalidStatus
	}
	return s.repo.List(ctx, f)
}

// Get loads a single vehicle.
func (s *Service) Get(ctx context.Context, id string) (*Vehicle, error) {
	return s.repo.Get(ctx, id)
}

// Create adds a vehicle to the fleet.
func (s *Service) Create(ctx context.Context, in CreateInput) (*Vehicle, error) {
	in.RegistrationNumber = strings.ToUpper(strings.TrimSpace(in.RegistrationNumber))
	if in.RegistrationNumber == "" {
		return nil, ErrRegistrationRequired
	}
	in.Model = cleanOptional(in.Model)
	if in.Capacity < 0 {
		return nil, ErrCapacityInvalid
	}
	if in.Status == "" {
		in.Status = StatusAvailable
	}
	if !ValidStatus(in.Status) {
		return nil, ErrInvalidStatus
	}

	created, err := s.repo.Create(ctx, in)
	if err != nil {
		if db.IsUniqueViolation(err) {
			return nil, ErrRegistrationTaken
		}
		return nil, err
	}
	return created, nil
}

// Update applies a partial update to a vehicle.
func (s *Service) Update(ctx context.Context, id string, in UpdateInput) (*Vehicle, error) {
	if in.RegistrationNumber != nil {
		trimmed := strings.ToUpper(strings.TrimSpace(*in.RegistrationNumber))
		if trimmed == "" {
			return nil, ErrRegistrationRequired
		}
		in.RegistrationNumber = &trimmed
	}
	in.Model = cleanOptional(in.Model)
	if in.Capacity != nil && *in.Capacity < 0 {
		return nil, ErrCapacityInvalid
	}
	if in.Status != nil && !ValidStatus(*in.Status) {
		return nil, ErrInvalidStatus
	}

	updated, err := s.repo.Update(ctx, id, in)
	if err != nil {
		if db.IsUniqueViolation(err) {
			return nil, ErrRegistrationTaken
		}
		return nil, err
	}
	return updated, nil
}

// Deactivate marks a vehicle inactive.
func (s *Service) Deactivate(ctx context.Context, id string) error {
	inactive := StatusInactive
	_, err := s.repo.Update(ctx, id, UpdateInput{Status: &inactive})
	return err
}

// ValidStatus reports whether status is a recognized vehicle status.
func ValidStatus(status string) bool {
	switch status {
	case StatusAvailable, StatusAssigned, StatusMaintenance, StatusInactive:
		return true
	default:
		return false
	}
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
