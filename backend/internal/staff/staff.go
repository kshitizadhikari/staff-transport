package staff

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

var (
	ErrNotFound     = errors.New("staff not found")
	ErrNameRequired = errors.New("name is required")
	ErrInvalidEmail = errors.New("invalid email address")
	ErrEmailTaken   = errors.New("email already in use")
)

// Staff is a staff member's profile joined with their user account.
//
// Home coordinates are intentionally not exposed yet; PostGIS storage and
// geocoding arrive with the maps module.
type Staff struct {
	ID          string
	UserID      string
	Name        string
	Email       *string
	Phone       *string
	Department  *string
	HomeAddress *string
	Active      bool
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// CreateInput carries validated create values to the repository.
type CreateInput struct {
	Name        string
	Email       *string
	Phone       *string
	Department  *string
	HomeAddress *string
	Active      bool
}

// UpdateInput carries partial update values. A nil field is left unchanged; a
// non-nil empty string clears a nullable field.
type UpdateInput struct {
	Name        *string
	Email       *string
	Phone       *string
	Department  *string
	HomeAddress *string
	Active      *bool
}

// ListFilter controls staff listing.
type ListFilter struct {
	Search     string
	Department string
	Active     *bool
	Offset     int
	Limit      int
}

// Repository persists the staff aggregate (a users row plus a staff row).
type Repository interface {
	List(ctx context.Context, f ListFilter) ([]Staff, int64, error)
	Get(ctx context.Context, id string) (*Staff, error)
	Create(ctx context.Context, in CreateInput, passwordHash *string) (*Staff, error)
	Update(ctx context.Context, id string, in UpdateInput) (*Staff, error)
}

type staffRow struct {
	ID          string    `gorm:"column:id"`
	UserID      string    `gorm:"column:user_id"`
	Name        string    `gorm:"column:name"`
	Email       *string   `gorm:"column:email"`
	Phone       *string   `gorm:"column:phone"`
	Department  *string   `gorm:"column:department"`
	HomeAddress *string   `gorm:"column:home_address"`
	Active      bool      `gorm:"column:active"`
	CreatedAt   time.Time `gorm:"column:created_at"`
	UpdatedAt   time.Time `gorm:"column:updated_at"`
}

const staffSelect = `s.id, s.user_id, u.name, u.email, u.phone, s.department, ` +
	`s.home_address, s.active, s.created_at, s.updated_at`

func (r *staffRow) toStaff() *Staff {
	return &Staff{
		ID:          r.ID,
		UserID:      r.UserID,
		Name:        r.Name,
		Email:       r.Email,
		Phone:       r.Phone,
		Department:  r.Department,
		HomeAddress: r.HomeAddress,
		Active:      r.Active,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
}

type gormRepository struct {
	db *gorm.DB
}

// NewRepository returns a GORM-backed staff repository.
func NewRepository(gdb *gorm.DB) Repository {
	return &gormRepository{db: gdb}
}

func (r *gormRepository) List(ctx context.Context, f ListFilter) ([]Staff, int64, error) {
	query := func() *gorm.DB {
		q := r.db.WithContext(ctx).
			Table("staff AS s").
			Joins("JOIN users u ON u.id = s.user_id")
		if f.Search != "" {
			like := "%" + f.Search + "%"
			q = q.Where("u.name ILIKE ? OR u.email ILIKE ? OR u.phone ILIKE ?", like, like, like)
		}
		if f.Department != "" {
			q = q.Where("s.department = ?", f.Department)
		}
		if f.Active != nil {
			q = q.Where("s.active = ?", *f.Active)
		}
		return q
	}

	var total int64
	if err := query().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []staffRow
	err := query().
		Select(staffSelect).
		Order("u.name ASC").
		Limit(f.Limit).
		Offset(f.Offset).
		Scan(&rows).Error
	if err != nil {
		return nil, 0, err
	}

	out := make([]Staff, 0, len(rows))
	for i := range rows {
		out = append(out, *rows[i].toStaff())
	}
	return out, total, nil
}

func (r *gormRepository) Get(ctx context.Context, id string) (*Staff, error) {
	var row staffRow
	err := r.db.WithContext(ctx).
		Table("staff AS s").
		Select(staffSelect).
		Joins("JOIN users u ON u.id = s.user_id").
		Where("s.id = ?", id).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toStaff(), nil
}

func (r *gormRepository) Create(ctx context.Context, in CreateInput, passwordHash *string) (*Staff, error) {
	userID := uuid.NewString()
	staffID := uuid.NewString()
	now := time.Now().UTC()

	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		user := map[string]any{
			"id":            userID,
			"name":          in.Name,
			"email":         in.Email,
			"phone":         in.Phone,
			"role":          string(users.RoleStaff),
			"status":        string(users.StatusActive),
			"password_hash": passwordHash,
			"created_at":    now,
			"updated_at":    now,
		}
		if err := tx.Table("users").Create(user).Error; err != nil {
			return err
		}

		row := map[string]any{
			"id":           staffID,
			"user_id":      userID,
			"department":   in.Department,
			"home_address": in.HomeAddress,
			"active":       in.Active,
			"created_at":   now,
			"updated_at":   now,
		}
		return tx.Table("staff").Create(row).Error
	})
	if err != nil {
		return nil, err
	}
	return r.Get(ctx, staffID)
}

func (r *gormRepository) Update(ctx context.Context, id string, in UpdateInput) (*Staff, error) {
	var userIDs []string
	err := r.db.WithContext(ctx).
		Table("staff").
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
	staffUpdates := map[string]any{}

	if in.Name != nil {
		userUpdates["name"] = *in.Name
	}
	if in.Email != nil {
		userUpdates["email"] = nullable(*in.Email)
	}
	if in.Phone != nil {
		userUpdates["phone"] = nullable(*in.Phone)
	}
	if in.Department != nil {
		staffUpdates["department"] = nullable(*in.Department)
	}
	if in.HomeAddress != nil {
		staffUpdates["home_address"] = nullable(*in.HomeAddress)
	}
	if in.Active != nil {
		staffUpdates["active"] = *in.Active
		userUpdates["status"] = statusForActive(*in.Active)
	}

	err = r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		if len(userUpdates) > 0 {
			if err := tx.Table("users").Where("id = ?", userID).Updates(userUpdates).Error; err != nil {
				return err
			}
		}
		if len(staffUpdates) > 0 {
			if err := tx.Table("staff").Where("id = ?", id).Updates(staffUpdates).Error; err != nil {
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

// Service holds staff business behavior.
type Service struct {
	repo Repository
}

// NewService returns a staff service backed by repo.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// List returns a page of staff profiles.
func (s *Service) List(ctx context.Context, f ListFilter) ([]Staff, int64, error) {
	f.Search = strings.TrimSpace(f.Search)
	return s.repo.List(ctx, f)
}

// Get loads a single staff profile.
func (s *Service) Get(ctx context.Context, id string) (*Staff, error) {
	return s.repo.Get(ctx, id)
}

// Create provisions a staff user account and profile. A blank password leaves
// the account without password credentials.
func (s *Service) Create(ctx context.Context, in CreateInput, password string) (*Staff, error) {
	in.Name = strings.TrimSpace(in.Name)
	if in.Name == "" {
		return nil, ErrNameRequired
	}
	if err := validateEmail(in.Email); err != nil {
		return nil, err
	}
	in.Email = cleanOptional(in.Email)
	in.Phone = cleanOptional(in.Phone)
	in.Department = cleanOptional(in.Department)
	in.HomeAddress = cleanOptional(in.HomeAddress)

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

// Update applies a partial update to a staff profile.
func (s *Service) Update(ctx context.Context, id string, in UpdateInput) (*Staff, error) {
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
	in.Department = cleanOptional(in.Department)
	in.HomeAddress = cleanOptional(in.HomeAddress)

	updated, err := s.repo.Update(ctx, id, in)
	if err != nil {
		if db.IsUniqueViolation(err) {
			return nil, ErrEmailTaken
		}
		return nil, err
	}
	return updated, nil
}

// Deactivate marks a staff profile and its user account inactive.
func (s *Service) Deactivate(ctx context.Context, id string) error {
	active := false
	_, err := s.repo.Update(ctx, id, UpdateInput{Active: &active})
	return err
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

// nullable maps an empty string to a NULL column value.
func nullable(v string) any {
	if strings.TrimSpace(v) == "" {
		return nil
	}
	return strings.TrimSpace(v)
}

func statusForActive(active bool) string {
	if active {
		return string(users.StatusActive)
	}
	return string(users.StatusInactive)
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
