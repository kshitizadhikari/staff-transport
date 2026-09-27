package users

import (
	"context"
	"errors"
	"strings"
	"testing"
)

func TestHashPasswordRoundTrip(t *testing.T) {
	hash, err := HashPassword("s3cret-password")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	if hash == "" || hash == "s3cret-password" {
		t.Fatalf("expected a non-plaintext hash, got %q", hash)
	}
	if !CheckPassword(hash, "s3cret-password") {
		t.Fatal("expected correct password to verify")
	}
	if CheckPassword(hash, "wrong-password") {
		t.Fatal("expected wrong password to fail verification")
	}
}

func TestValidRole(t *testing.T) {
	for _, role := range []Role{RoleManager, RoleDriver, RoleStaff} {
		if !ValidRole(role) {
			t.Fatalf("expected %q to be valid", role)
		}
	}
	if ValidRole("admin") {
		t.Fatal("expected unknown role to be invalid")
	}
}

type fakeRepo struct {
	byEmail map[string]*User
	byID    map[string]*User
}

func newFakeRepo(users ...*User) *fakeRepo {
	r := &fakeRepo{byEmail: map[string]*User{}, byID: map[string]*User{}}
	for _, u := range users {
		r.byEmail[strings.ToLower(derefString(u.Email))] = u
		r.byID[u.ID] = u
	}
	return r
}

func (r *fakeRepo) FindByEmail(_ context.Context, email string) (*User, error) {
	if u, ok := r.byEmail[strings.ToLower(email)]; ok {
		return u, nil
	}
	return nil, ErrNotFound
}

func (r *fakeRepo) FindByID(_ context.Context, id string) (*User, error) {
	if u, ok := r.byID[id]; ok {
		return u, nil
	}
	return nil, ErrNotFound
}

func (r *fakeRepo) Create(_ context.Context, u *User, passwordHash string) (*User, error) {
	u.ID = "generated"
	u.PasswordHash = &passwordHash
	u.Status = StatusActive
	r.byEmail[strings.ToLower(derefString(u.Email))] = u
	r.byID[u.ID] = u
	return u, nil
}

func (r *fakeRepo) Upsert(ctx context.Context, u *User, passwordHash string) (*User, error) {
	if existing, err := r.FindByEmail(ctx, derefString(u.Email)); err == nil {
		existing.Name = u.Name
		existing.Role = u.Role
		existing.PasswordHash = &passwordHash
		return existing, nil
	}
	return r.Create(ctx, u, passwordHash)
}

func TestAuthenticate(t *testing.T) {
	hash, err := HashPassword("correct-password")
	if err != nil {
		t.Fatalf("hash: %v", err)
	}
	email := "manager@example.com"
	active := &User{ID: "u1", Name: "Manager", Email: &email, PasswordHash: &hash, Role: RoleManager, Status: StatusActive}

	repo := newFakeRepo(active)
	svc := NewService(repo)
	ctx := context.Background()

	t.Run("valid credentials", func(t *testing.T) {
		u, err := svc.Authenticate(ctx, "MANAGER@example.com", "correct-password")
		if err != nil {
			t.Fatalf("expected success, got %v", err)
		}
		if u.ID != "u1" {
			t.Fatalf("unexpected user: %+v", u)
		}
	})

	t.Run("wrong password", func(t *testing.T) {
		if _, err := svc.Authenticate(ctx, email, "wrong"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("unknown email", func(t *testing.T) {
		if _, err := svc.Authenticate(ctx, "nobody@example.com", "correct-password"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})

	t.Run("inactive user", func(t *testing.T) {
		inactive := *active
		inactive.ID = "u2"
		inactive.Status = StatusInactive
		inactiveEmail := "inactive@example.com"
		inactive.Email = &inactiveEmail
		svc := NewService(newFakeRepo(&inactive))
		if _, err := svc.Authenticate(ctx, inactiveEmail, "correct-password"); !errors.Is(err, ErrInactive) {
			t.Fatalf("expected ErrInactive, got %v", err)
		}
	})

	t.Run("missing password", func(t *testing.T) {
		noPassword := &User{ID: "u3", Name: "NoPass", Email: &email, Role: RoleStaff, Status: StatusActive}
		svc := NewService(newFakeRepo(noPassword))
		if _, err := svc.Authenticate(ctx, email, "whatever"); !errors.Is(err, ErrInvalidCredentials) {
			t.Fatalf("expected ErrInvalidCredentials, got %v", err)
		}
	})
}

func TestProvisionCreatesThenUpdates(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo)
	ctx := context.Background()

	created, err := svc.Provision(ctx, "Manager", "manager@example.com", "first-password", RoleManager)
	if err != nil {
		t.Fatalf("provision create: %v", err)
	}
	if !CheckPassword(*created.PasswordHash, "first-password") {
		t.Fatal("expected created password to verify")
	}

	updated, err := svc.Provision(ctx, "Renamed", "manager@example.com", "second-password", RoleManager)
	if err != nil {
		t.Fatalf("provision update: %v", err)
	}
	if updated.ID != created.ID {
		t.Fatalf("expected same account, got %s and %s", created.ID, updated.ID)
	}
	if updated.Name != "Renamed" || !CheckPassword(*updated.PasswordHash, "second-password") {
		t.Fatalf("expected updated account, got %+v", updated)
	}
}

func TestProvisionValidatesInput(t *testing.T) {
	svc := NewService(newFakeRepo())
	ctx := context.Background()

	cases := map[string]struct {
		name, email, password string
		role                  Role
	}{
		"missing name":     {name: "", email: "a@example.com", password: "pw", role: RoleManager},
		"missing email":    {name: "A", email: "", password: "pw", role: RoleManager},
		"missing password": {name: "A", email: "a@example.com", password: "", role: RoleManager},
		"invalid role":     {name: "A", email: "a@example.com", password: "pw", role: "admin"},
	}
	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.Provision(ctx, tc.name, tc.email, tc.password, tc.role); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
