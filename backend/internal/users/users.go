package users

import (
	"context"
	"errors"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// Role is a user's authorization role.
type Role string

const (
	RoleManager Role = "manager"
	RoleDriver  Role = "driver"
	RoleStaff   Role = "staff"
)

// Status is a user's account status. Accounts are deactivated rather than
// deleted so historical relationships remain intact.
type Status string

const (
	StatusActive   Status = "active"
	StatusInactive Status = "inactive"
)

var (
	ErrNotFound           = errors.New("user not found")
	ErrInvalidCredentials = errors.New("invalid credentials")
	ErrInactive           = errors.New("user is inactive")
)

// User is an authentication identity shared by the manager, driver, and staff
// roles. Profile-specific data lives in the staff and drivers modules.
type User struct {
	ID           string
	Name         string
	Email        *string
	Phone        *string
	PasswordHash *string
	Role         Role
	Status       Status
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

// HasPassword reports whether the account can authenticate with a password.
func (u *User) HasPassword() bool {
	return u.PasswordHash != nil && *u.PasswordHash != ""
}

// userRow maps the users table. Schema is owned by migrations, not AutoMigrate.
type userRow struct {
	ID           string    `gorm:"column:id;primaryKey"`
	Name         string    `gorm:"column:name"`
	Email        *string   `gorm:"column:email"`
	Phone        *string   `gorm:"column:phone"`
	PasswordHash *string   `gorm:"column:password_hash"`
	Role         string    `gorm:"column:role"`
	Status       string    `gorm:"column:status"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

func (userRow) TableName() string { return "users" }

func (r *userRow) toUser() *User {
	return &User{
		ID:           r.ID,
		Name:         r.Name,
		Email:        r.Email,
		Phone:        r.Phone,
		PasswordHash: r.PasswordHash,
		Role:         Role(r.Role),
		Status:       Status(r.Status),
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
}

// Repository reads and writes user records.
type Repository interface {
	FindByEmail(ctx context.Context, email string) (*User, error)
	FindByID(ctx context.Context, id string) (*User, error)
	Create(ctx context.Context, u *User, passwordHash string) (*User, error)
	Upsert(ctx context.Context, u *User, passwordHash string) (*User, error)
}

type gormRepository struct {
	db *gorm.DB
}

// NewRepository returns a GORM-backed user repository.
func NewRepository(db *gorm.DB) Repository {
	return &gormRepository{db: db}
}

func (r *gormRepository) FindByEmail(ctx context.Context, email string) (*User, error) {
	var row userRow
	err := r.db.WithContext(ctx).
		Where("lower(email) = lower(?)", strings.TrimSpace(email)).
		First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toUser(), nil
}

func (r *gormRepository) FindByID(ctx context.Context, id string) (*User, error) {
	var row userRow
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toUser(), nil
}

func (r *gormRepository) Create(ctx context.Context, u *User, passwordHash string) (*User, error) {
	row := userRow{
		Name:         u.Name,
		Email:        u.Email,
		Phone:        u.Phone,
		PasswordHash: &passwordHash,
		Role:         string(u.Role),
		Status:       string(StatusActive),
	}
	if err := r.db.WithContext(ctx).Create(&row).Error; err != nil {
		return nil, err
	}
	return row.toUser(), nil
}

func (r *gormRepository) Upsert(ctx context.Context, u *User, passwordHash string) (*User, error) {
	existing, err := r.FindByEmail(ctx, derefString(u.Email))
	switch {
	case err == nil:
		updates := map[string]any{
			"name":          u.Name,
			"role":          string(u.Role),
			"status":        string(StatusActive),
			"password_hash": passwordHash,
		}
		if err := r.db.WithContext(ctx).
			Model(&userRow{}).
			Where("id = ?", existing.ID).
			Updates(updates).Error; err != nil {
			return nil, err
		}
		return r.FindByID(ctx, existing.ID)
	case errors.Is(err, ErrNotFound):
		return r.Create(ctx, u, passwordHash)
	default:
		return nil, err
	}
}

// Service holds user business behavior consumed by API modules.
type Service struct {
	repo Repository
}

// NewService returns a user service backed by repo.
func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

// FindByID loads a user by ID.
func (s *Service) FindByID(ctx context.Context, id string) (*User, error) {
	if strings.TrimSpace(id) == "" {
		return nil, ErrNotFound
	}
	return s.repo.FindByID(ctx, id)
}

// Authenticate verifies an email/password pair and returns the active user.
// It returns ErrInvalidCredentials for both unknown emails and wrong passwords
// so callers cannot enumerate accounts.
func (s *Service) Authenticate(ctx context.Context, email, password string) (*User, error) {
	if strings.TrimSpace(email) == "" || password == "" {
		return nil, ErrInvalidCredentials
	}
	u, err := s.repo.FindByEmail(ctx, email)
	if err != nil {
		if errors.Is(err, ErrNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, err
	}
	if !u.HasPassword() || !CheckPassword(*u.PasswordHash, password) {
		return nil, ErrInvalidCredentials
	}
	if u.Status != StatusActive {
		return nil, ErrInactive
	}
	return u, nil
}

// HashPassword returns a bcrypt hash suitable for storage.
func HashPassword(plain string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(plain), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// CheckPassword reports whether plain matches the stored bcrypt hash.
func CheckPassword(hash, plain string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(plain)) == nil
}

// Provision creates or updates a password-based user account. It is intended
// for provisioning tools such as the seed command, not for public API use.
func (s *Service) Provision(ctx context.Context, name, email, password string, role Role) (*User, error) {
	if strings.TrimSpace(name) == "" {
		return nil, errors.New("name is required")
	}
	email = strings.TrimSpace(email)
	if email == "" {
		return nil, errors.New("email is required")
	}
	if password == "" {
		return nil, errors.New("password is required")
	}
	if !ValidRole(role) {
		return nil, errors.New("invalid role")
	}
	hash, err := HashPassword(password)
	if err != nil {
		return nil, err
	}
	return s.repo.Upsert(ctx, &User{Name: name, Email: &email, Role: role}, hash)
}

// ValidRole reports whether role is a recognized user role.
func ValidRole(role Role) bool {
	switch role {
	case RoleManager, RoleDriver, RoleStaff:
		return true
	default:
		return false
	}
}

func derefString(v *string) string {
	if v == nil {
		return ""
	}
	return *v
}
