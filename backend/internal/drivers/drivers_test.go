package drivers

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
	items      []Driver
	createErr  error
	lastCreate *CreateInput
	lastUpdate *UpdateInput
}

func (f *fakeRepo) List(_ context.Context, _ ListFilter) ([]Driver, int64, error) {
	return f.items, int64(len(f.items)), nil
}

func (f *fakeRepo) Get(_ context.Context, id string) (*Driver, error) {
	for i := range f.items {
		if f.items[i].ID == id {
			return &f.items[i], nil
		}
	}
	return nil, ErrNotFound
}

func (f *fakeRepo) FindByUserID(_ context.Context, userID string) (*Driver, error) {
	for i := range f.items {
		if f.items[i].UserID == userID {
			return &f.items[i], nil
		}
	}
	return nil, ErrNotFound
}

func (f *fakeRepo) Create(_ context.Context, in CreateInput, _ *string) (*Driver, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.lastCreate = &in
	return &Driver{ID: "d1", Name: in.Name, Status: in.Status}, nil
}

func (f *fakeRepo) Update(_ context.Context, id string, in UpdateInput) (*Driver, error) {
	f.lastUpdate = &in
	return &Driver{ID: id, Status: StatusOffline}, nil
}

func TestCreateDefaultsToOffline(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	created, err := svc.Create(context.Background(), CreateInput{Name: "Sam"}, "")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Status != StatusOffline {
		t.Fatalf("expected offline status, got %q", created.Status)
	}
}

func TestCreateRejectsInvalidStatus(t *testing.T) {
	svc := NewService(&fakeRepo{})

	if _, err := svc.Create(context.Background(), CreateInput{Name: "Sam", Status: "sleeping"}, ""); !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestUpdateRejectsInvalidStatus(t *testing.T) {
	svc := NewService(&fakeRepo{})
	status := "sleeping"

	if _, err := svc.Update(context.Background(), "d1", UpdateInput{Status: &status}); !errors.Is(err, ErrInvalidStatus) {
		t.Fatalf("expected ErrInvalidStatus, got %v", err)
	}
}

func TestCreateMapsUniqueViolation(t *testing.T) {
	repo := &fakeRepo{createErr: &pgconn.PgError{Code: "23505"}}
	svc := NewService(repo)

	if _, err := svc.Create(context.Background(), CreateInput{Name: "Sam"}, ""); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("expected ErrEmailTaken, got %v", err)
	}
}

func TestDeactivateSetsInactive(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	if err := svc.Deactivate(context.Background(), "d1"); err != nil {
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

func newDriverRouter(repo *fakeRepo, claims map[string]*auth.Claims) *gin.Engine {
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

func TestDriverRoutesRequireManager(t *testing.T) {
	claims := map[string]*auth.Claims{
		"manager-token": {UserID: "m1", Role: "manager"},
		"driver-token":  {UserID: "d1", Role: "driver"},
	}
	r := newDriverRouter(&fakeRepo{}, claims)

	if w := perform(r, http.MethodGet, "/api/v1/drivers", "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if w := perform(r, http.MethodGet, "/api/v1/drivers", "", "driver-token"); w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	if w := perform(r, http.MethodGet, "/api/v1/drivers", "", "manager-token"); w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCreateDriverHandler(t *testing.T) {
	claims := map[string]*auth.Claims{"manager-token": {UserID: "m1", Role: "manager"}}
	r := newDriverRouter(&fakeRepo{}, claims)

	if w := perform(r, http.MethodPost, "/api/v1/drivers", `{"name":"Sam","license_expiry":"nope"}`, "manager-token"); w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid date, got %d", w.Code)
	}

	w := perform(r, http.MethodPost, "/api/v1/drivers", `{"name":"Sam","license_expiry":"2030-01-02"}`, "manager-token")
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}
