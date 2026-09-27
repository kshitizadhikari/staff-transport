package staff

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"

	"staff-transport/internal/auth"
	"staff-transport/internal/users"
)

type fakeRepo struct {
	items      []Staff
	createErr  error
	updateErr  error
	lastCreate *CreateInput
	lastHash   *string
	lastUpdate *UpdateInput
	lastFilter *ListFilter
}

func (f *fakeRepo) List(_ context.Context, filter ListFilter) ([]Staff, int64, error) {
	f.lastFilter = &filter
	return f.items, int64(len(f.items)), nil
}

func (f *fakeRepo) Get(_ context.Context, id string) (*Staff, error) {
	for i := range f.items {
		if f.items[i].ID == id {
			return &f.items[i], nil
		}
	}
	return nil, ErrNotFound
}

func (f *fakeRepo) FindByUserID(_ context.Context, userID string) (*Staff, error) {
	for i := range f.items {
		if f.items[i].UserID == userID {
			return &f.items[i], nil
		}
	}
	return nil, ErrNotFound
}

func (f *fakeRepo) Create(_ context.Context, in CreateInput, passwordHash *string) (*Staff, error) {
	if f.createErr != nil {
		return nil, f.createErr
	}
	f.lastCreate = &in
	f.lastHash = passwordHash
	email := in.Email
	return &Staff{ID: "s1", UserID: "u1", Name: in.Name, Email: email, Active: in.Active}, nil
}

func (f *fakeRepo) Update(_ context.Context, id string, in UpdateInput) (*Staff, error) {
	if f.updateErr != nil {
		return nil, f.updateErr
	}
	f.lastUpdate = &in
	return &Staff{ID: id, Name: "Updated", Active: true}, nil
}

func TestCreateTrimsNameAndHashesPassword(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	_, err := svc.Create(context.Background(), CreateInput{Name: "  Ada  ", Active: true}, "secret-password")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if repo.lastCreate == nil || repo.lastCreate.Name != "Ada" {
		t.Fatalf("expected trimmed name, got %+v", repo.lastCreate)
	}
	if repo.lastHash == nil || !users.CheckPassword(*repo.lastHash, "secret-password") {
		t.Fatal("expected password to be hashed")
	}
}

func TestCreateWithoutPasswordLeavesNoHash(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	if _, err := svc.Create(context.Background(), CreateInput{Name: "Ada", Active: true}, ""); err != nil {
		t.Fatalf("create: %v", err)
	}
	if repo.lastHash != nil {
		t.Fatalf("expected nil hash, got %v", *repo.lastHash)
	}
}

func TestCreateValidatesInput(t *testing.T) {
	svc := NewService(&fakeRepo{})
	bad := "not-an-email"

	if _, err := svc.Create(context.Background(), CreateInput{Name: "  ", Active: true}, ""); !errors.Is(err, ErrNameRequired) {
		t.Fatalf("expected ErrNameRequired, got %v", err)
	}
	if _, err := svc.Create(context.Background(), CreateInput{Name: "Ada", Email: &bad, Active: true}, ""); !errors.Is(err, ErrInvalidEmail) {
		t.Fatalf("expected ErrInvalidEmail, got %v", err)
	}
}

func TestCreateMapsUniqueViolation(t *testing.T) {
	repo := &fakeRepo{createErr: &pgconn.PgError{Code: "23505"}}
	svc := NewService(repo)

	if _, err := svc.Create(context.Background(), CreateInput{Name: "Ada", Active: true}, ""); !errors.Is(err, ErrEmailTaken) {
		t.Fatalf("expected ErrEmailTaken, got %v", err)
	}
}

func TestDeactivateSetsInactive(t *testing.T) {
	repo := &fakeRepo{}
	svc := NewService(repo)

	if err := svc.Deactivate(context.Background(), "s1"); err != nil {
		t.Fatalf("deactivate: %v", err)
	}
	if repo.lastUpdate == nil || repo.lastUpdate.Active == nil || *repo.lastUpdate.Active {
		t.Fatalf("expected active=false, got %+v", repo.lastUpdate)
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

func newStaffRouter(repo *fakeRepo, claims map[string]*auth.Claims) *gin.Engine {
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

func TestStaffRoutesRequireManager(t *testing.T) {
	claims := map[string]*auth.Claims{
		"manager-token": {UserID: "m1", Role: "manager"},
		"staff-token":   {UserID: "s1", Role: "staff"},
	}
	r := newStaffRouter(&fakeRepo{}, claims)

	if w := perform(r, http.MethodGet, "/api/v1/staff", "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401 without token, got %d", w.Code)
	}
	if w := perform(r, http.MethodGet, "/api/v1/staff", "", "staff-token"); w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for staff role, got %d", w.Code)
	}
	w := perform(r, http.MethodGet, "/api/v1/staff", "", "manager-token")
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 for manager, got %d: %s", w.Code, w.Body.String())
	}
	var body struct {
		Data       []response `json:"data"`
		Pagination struct {
			Total int64 `json:"total"`
		} `json:"pagination"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if body.Data == nil {
		t.Fatal("expected data array, got nil")
	}
}

func TestCreateStaffHandler(t *testing.T) {
	claims := map[string]*auth.Claims{"manager-token": {UserID: "m1", Role: "manager"}}
	r := newStaffRouter(&fakeRepo{}, claims)

	if w := perform(r, http.MethodPost, "/api/v1/staff", `{"name":""}`, "manager-token"); w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for invalid body, got %d", w.Code)
	}

	w := perform(r, http.MethodPost, "/api/v1/staff", `{"name":"Ada","email":"ada@example.com","password":"secret-password"}`, "manager-token")
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestGetStaffNotFound(t *testing.T) {
	claims := map[string]*auth.Claims{"manager-token": {UserID: "m1", Role: "manager"}}
	r := newStaffRouter(&fakeRepo{}, claims)

	w := perform(r, http.MethodGet, "/api/v1/staff/missing", "", "manager-token")
	if w.Code != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", w.Code)
	}
}
