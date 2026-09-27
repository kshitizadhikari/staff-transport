package vehicles

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"

	"staff-transport/internal/auth"
)

type fakeRepo struct {
	items      []Vehicle
	createErr  error
	lastCreate *CreateInput
	lastUpdate *UpdateInput
}

func (f *fakeRepo) List(_ context.Context, _ ListFilter) ([]Vehicle, int64, error) {
	return f.items, int64(len(f.items)), nil
}

func (f *fakeRepo) Get(_ context.Context, id string) (*Vehicle, error) {
	for i := range f.items {
		if f.items[i].ID == id {
			return &f.items[i], nil
		}
	}
	return nil, ErrNotFound
}

func (f *fakeRepo) Create(_ context.Context, in CreateInput) (*Vehicle, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.lastCreate = &in
	return &Vehicle{ID: "v1", RegistrationNumber: in.RegistrationNumber, Capacity: in.Capacity, Status: in.Status}, nil
}

func (f *fakeRepo) Update(_ context.Context, id string, in UpdateInput) (*Vehicle, error) {
	f.lastUpdate = &in
	return &Vehicle{ID: id, Status: StatusAvailable}, nil
}

func TestCreateNormalizesRegistration(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	created, err := svc.Create(context.Background(), CreateInput{RegistrationNumber: "  ba 1 pa 1234 ", Capacity: 4})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.RegistrationNumber != "BA 1 PA 1234" {
		t.Fatalf("expected normalized registration, got %q", created.RegistrationNumber)
	}
	if created.Status != StatusAvailable {
		t.Fatalf("expected available status, got %q", created.Status)
	}
}

func TestCreateRejectsBlankRegistration(t *testing.T) {
	svc := NewService(&fakeRepo{})

	if _, err := svc.Create(context.Background(), CreateInput{RegistrationNumber: "   "}); !errors.Is(err, ErrRegistrationRequired) {
		t.Fatalf("expected ErrRegistrationRequired, got %v", err)
	}
}

func TestCreateRejectsNegativeCapacity(t *testing.T) {
	svc := NewService(&fakeRepo{})

	if _, err := svc.Create(context.Background(), CreateInput{RegistrationNumber: "BA 1 PA 1234", Capacity: -1}); !errors.Is(err, ErrCapacityInvalid) {
		t.Fatalf("expected ErrCapacityInvalid, got %v", err)
	}
}

func TestCreateRejectsInvalidStatus(t *testing.T) {
	svc := NewService(&fakeRepo{})

	if _, err := svc.Create(context.Background(), CreateInput{RegistrationNumber: "BA 1 PA 1234", Status: "flying"}); !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestCreateMapsRegistrationTaken(t *testing.T) {
	repo := &fakeRepo{createErr: &pgconn.PgError{Code: "23505"}}
	svc := NewService(repo)

	if _, err := svc.Create(context.Background(), CreateInput{RegistrationNumber: "BA 1 PA 1234"}); !errors.Is(err, ErrRegistrationTaken) {
		t.Fatalf("expected ErrRegistrationTaken, got %v", err)
	}
}

func TestDeactivateSetsInactive(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	if err := svc.Deactivate(context.Background(), "v1"); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if repo.lastUpdate == nil || repo.lastUpdate.Status == nil || *repo.lastUpdate.Status != StatusInactive {
		t.Fatalf("expected inactive status, got %+v", repo.lastUpdate)
	}
}

type fakeAuth struct {
	claims map[string]*auth.Claims
}

func (f fakeAuth) Issue(context.Context, string, string) (auth.TokenPair, error) {
	return auth.TokenPair{}, nil
}
func (f fakeAuth) ConsumeRefresh(context.Context, string) (string, error) { return "", nil }
func (f fakeAuth) Revoke(context.Context, string) error                   { return nil }
func (f fakeAuth) ParseAccessToken(token string) (*auth.Claims, error) {
	if c, ok := f.claims[token]; ok {
		return c, nil
	}
	return nil, auth.ErrInvalidToken
}

func newVehicleRouter(repo *fakeRepo, claims map[string]*auth.Claims) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	NewHandler(NewService(repo), fakeAuth{claims: claims}).RegisterRoutes(r.Group("/api/v1"))
	return r
}

func perform(r http.Handler, method, path, body, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestVehicleRoutesRequireManager(t *testing.T) {
	claims := map[string]*auth.Claims{
		"manager-token": {UserID: "m1", Role: "manager"},
		"staff-token":   {UserID: "s1", Role: "staff"},
	}
	r := newVehicleRouter(&fakeRepo{}, claims)

	if w := perform(r, http.MethodGet, "/api/v1/vehicles", "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if w := perform(r, http.MethodGet, "/api/v1/vehicles", "", "staff-token"); w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	if w := perform(r, http.MethodGet, "/api/v1/vehicles", "", "manager-token"); w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCreateVehicleHandler(t *testing.T) {
	claims := map[string]*auth.Claims{"manager-token": {UserID: "m1", Role: "manager"}}
	r := newVehicleRouter(&fakeRepo{}, claims)

	if w := perform(r, http.MethodPost, "/api/v1/vehicles", `{"registration_number":""}`, "manager-token"); w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for blank registration, got %d", w.Code)
	}

	w := perform(r, http.MethodPost, "/api/v1/vehicles", `{"registration_number":"BA 1 PA 1234","capacity":4}`, "manager-token")
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}
