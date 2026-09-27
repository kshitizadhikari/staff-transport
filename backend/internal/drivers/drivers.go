package drivers

import (
	"context"
	"errors"
	"net/mail"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"staff-transport/internal/db"
	"staff-transport/internal/users"
)

// Driver operational statuses.
const (
	StatusAvailable = "available"
	StatusOnTrip    = "on_trip"
	StatusOffline   = "offline"
	StatusLeave     = "leave"
	StatusInactive  = "inactive"
)

var (
	ErrNotFound      = errors.New("driver not found")
	ErrNameRequired  = errors.New("name is required")
	ErrInvalidEmail  = errors.New("invalid email address")
	ErrEmailTaken    = errors.New("email already in use")
	ErrInvalidStatus = errors.New("invalid driver status")
)

// Driver is a driver's profile joined with their user account.
type Driver struct {
	ID            string
	UserID        string
	Name          string
	Email         *string
	Phone         *string
	LicenseNumber *string
	LicenseExpiry *time.Time
	Status        string
	CreatedAt     time.Time
	UpdatedAt     time.Time
}

// CreateInput carries validated create values to the repository.
type CreateInput struct {
	Name          string
	Email         *string
	Phone         *string
	LicenseNumber *string
	LicenseExpiry *time.Time
	Status        string
}

// UpdateInput carries partial update values. A nil field is left unchanged.
type UpdateInput struct {
	Name          *string
	Email         *string
	Phone         *string
	LicenseNumber *string
	LicenseExpiry *time.Time
	Status        *string
}

// ListFilter controls driver listing.
type ListFilter struct {
	Search string
	Status string
	Offset int
	Limit  int
}

// Repository persists the driver aggregate (a users row plus a drivers row).
type Repository interface {
	List(ctx context.Context, f ListFilter) ([]Driver, int64, error)
	Get(ctx context.Context, id string) (*Driver, error)
	Create(ctx context.Context, in CreateInput, passwordHash *string) (*Driver, error)
	Update(ctx context.Context, id string, in UpdateInput) (*Driver, error)
}

type driverRow struct {
	ID            string     `gorm:"column:id"`
	UserID        string     `gorm:"column:user_id"`
	Name          string     `gorm:"column:name"`
	Email         *string    `gorm:"column:email"`
	Phone         *string    `gorm:"column:phone"`
	LicenseNumber *string    `gorm:"column:license_number"`
	LicenseExpiry *time.Time `gorm:"column:license_expiry"`
	Status        string     `gorm:"column:status"`
	CreatedAt     time.Time  `gorm:"column:created_at"`
	UpdatedAt     time.Time  `gorm:"column:updated_at"`
}

const driverSelect = `d.id, d.user_id, u.name, u.email, u.phone, d.license_number, ` +
	`d.license_expiry, d.status, d.created_at, d.updated_at`

func (r *driverRow) toDriver() *Driver {
	return &Driver{
		ID:            r.ID,
		UserID:        r.UserID,
		Name:          r.Name,
		Email:         r.Email,
		Phone:         r.Phone,
		LicenseNumber: r.LicenseNumber,
		LicenseExpiry: r.LicenseExpiry,
		Status:        r.Status,
		CreatedAt:     r.CreatedAt,
		UpdatedAt:     r.UpdatedAt,
	}
}

type gormRepository struct {
	db *gorm.DB
}

// NewRepository returns a GORM-backed driver repository.
func NewRepository(gdb *gorm.DB) Repository {
	return &gormRepository{db: gdb}
}

func (r *gormRepository) List(ctx context.Context, f ListFilter) ([]Driver, int64, error) {
	query := func() *gorm.DB {
		q := r.db.WithContext(ctx).
			Table("drivers AS d").
			Joins("JOIN users u ON u.id = d.user_id")
		if f.Search != "" {
			like := "%" + f.Search + "%"
			q = q.Where("u.name ILIKE ? OR u.email ILIKE ? OR u.phone ILIKE ?", like, like, like)
		}
		if f.Status != "" {
			q = q.Where("d.status = ?", f.Status)
		}
		return q
	}

	var total int64
	if err := query().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []driverRow
	if err := query().
		Select(driverSelect).
		Order("u.name ASC").
		Limit(f.Limit).
		Offset(f.Offset).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	out := make([]Driver, 0, len(rows))
	for i := range rows {
		out = append(out, *rows[i].toDriver())
	}
	return out, total, nil
}

func (r *gormRepository) Get(ctx context.Context, id string) (*Driver, error) {
	var row driverRow
	err := r.db.WithContext(ctx).
		Table("drivers AS d").
		Select(driverSelect).
		Joins("JOIN users u ON u.id = d.user_id").
		Where("d.id = ?", id).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toDriver(), nil
}

func (r *gormRepository) Create(ctx context.Context, in CreateInput, passwordHash *string) (*Driver, error) {
	userID := uuid.NewString()
	driverID := uuid.NewString()
	now := time.Now().UTC()

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user := map[string]any{
			"id":            userID,
			"name":          in.Name,
			"email":         in.Email,
			"phone":         in.Phone,
			"role":          string(users.RoleDriver),
			"status":        string(users.StatusActive),
			"password_hash": passwordHash,
			"created_at":    now,
			"updated_at":    now,
		}
		if err := tx.Table("users").Create(user).Error; err != nil {
			return err
		}

		row := map[string]any{
			"id":             driverID,
			"user_id":        userID,
			"license_number": in.LicenseNumber,
			"license_expiry": dateOrNil(in.LicenseExpiry),
			"status":         in.Status,
			"created_at":     now,
			"updated_at":     now,
		}
		return tx.Table("drivers").Create(row).Error
	})
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, driverID)
}

func (r *gormRepository) Update(ctx context.Context, id string, in UpdateInput) (*Driver, error) {
	var userIDs []string
	err := r.db.WithContext(ctx).
		Table("drivers").
		Where("id = ?", id).
		Pluck("user_id", &userIDs).Error
	if err != nil {
		return nil, err
	}
	if len(userIDs) == 0 {
		return nil, ErrNotFound
	}
	userID := userIDs[0]

	userUpdates := map[string]any{}
	driverUpdates := map[string]any{}

	if in.Name != nil {
		userUpdates["name"] = *in.Name
	}
	if in.Email != nil {
		userUpdates["email"] = nullable(*in.Email)
	}
	if in.Phone != nil {
		userUpdates["phone"] = nullable(*in.Phone)
	}
	if in.LicenseNumber != nil {
		driverUpdates["license_number"] = nullable(*in.LicenseNumber)
	}
	if in.LicenseExpiry != nil {
		driverUpdates["license_expiry"] = dateOrNil(in.LicenseExpiry)
	}
	if in.Status != nil {
		driverUpdates["status"] = *in.Status
	}

	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(userUpdates) > 0 {
			if err := tx.Table("users").Where("id = ?", userID).Updates(userUpdates).Error; err != nil {
				return err
			}
		}
		if len(driverUpdates) > 0 {
			if err := tx.Table("drivers").Where("id = ?", id).Updates(driverUpdates).Error; err != nil {
				return err
			}
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, id)
}

// Service holds driver business behavior.
type Service struct {
	repo Repository
}

// NewService returns a driver service backed by repo.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// List returns a page of driver profiles.
func (s *Service) List(ctx context.Context, f ListFilter) ([]Driver, int64, error) {
	f.Search = strings.TrimSpace(f.Search)
	if f.Status != "" && !ValidStatus(f.Status) {
		return nil, 0, ErrInvalidStatus
	}
	return s.repo.List(ctx, f)
}

// Get loads a single driver profile.
func (s *Service) Get(ctx context.Context, id string) (*Driver, error) {
	return s.repo.Get(ctx, id)
}

// Create provisions a driver account and profile. A blank password leaves the
// account without password credentials.
func (s *Service) Create(ctx context.Context, in CreateInput, password string) (*Driver, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return nil, ErrNameRequired
	}
	if err := validateEmail(in.Email); err != nil {
		return nil, err
	}
	in.Email = cleanOptional(in.Email)
	in.Phone = cleanOptional(in.Phone)
	in.LicenseNumber = cleanOptional(in.LicenseNumber)

	if in.Status == "" {
		in.Status = StatusOffline
	}
	if !ValidStatus(in.Status) {
		return nil, ErrInvalidStatus
	}

	var hash *string
	if password != "" {
		h, err := users.HashPassword(password)
		if err != nil {
			return nil, err
		}
		hash = &h
	}

	created, err := s.repo.Create(ctx, in, hash)
	if err != nil {
		if db.IsUniqueViolation(err) {
			return nil, ErrEmailTaken
		}
		return nil, err
	}
	return created, nil
}

// Update applies a partial update to a driver profile.
func (s *Service) Update(ctx context.Context, id string, in UpdateInput) (*Driver, error) {
	if in.Name != nil {
		trimmed := strings.TrimSpace(*in.Name)
		if trimmed == "" {
			return nil, ErrNameRequired
		}
		in.Name = &trimmed
	}
	if err := validateEmail(in.Email); err != nil {
		return nil, err
	}
	in.Email = cleanOptional(in.Email)
	in.Phone = cleanOptional(in.Phone)
	in.LicenseNumber = cleanOptional(in.LicenseNumber)

	if in.Status != nil && !ValidStatus(*in.Status) {
		return nil, ErrInvalidStatus
	}

	updated, err := s.repo.Update(ctx, id, in)
	if err != nil {
		if db.IsUniqueViolation(err) {
			return nil, ErrEmailTaken
		}
		return nil, err
	}
	return updated, nil
}

// Deactivate sets a driver's operational status to inactive.
func (s *Service) Deactivate(ctx context.Context, id string) error {
	inactive := StatusInactive
	_, err := s.repo.Update(ctx, id, UpdateInput{Status: &inactive})
	return err
}

// ValidStatus reports whether status is a recognized driver status.
func ValidStatus(status string) bool {
	switch status {
	case StatusAvailable, StatusOnTrip, StatusOffline, StatusLeave, StatusInactive:
		return true
	default:
		return false
	}
}

func dateOrNil(t *time.Time) any {
	if t == nil {
		return nil
	}
	return gorm.Expr("?::date", *t)
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

func validateEmail(v *string) error {
	if v == nil {
		return nil
	}
	trimmed := strings.TrimSpace(*v)
	if trimmed == "" {
		return nil
	}
	if _, err := mail.ParseAddress(trimmed); err != nil {
		return ErrInvalidEmail
	}
	return nil
}
