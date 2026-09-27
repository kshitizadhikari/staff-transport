package auth

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"staff-transport/internal/config"
	"staff-transport/internal/users"
)

type fakeTokens struct {
	pair        TokenPair
	issueErr    error
	issuedUser  string
	issuedRole  string
	consumeID   string
	consumeErr  error
	revoked     string
	revokeErr   error
	parseClaims *Claims
	parseErr    error
}

func (f *fakeTokens) Issue(_ context.Context, userID, role string) (TokenPair, error) {
	f.issuedUser, f.issuedRole = userID, role
	return f.pair, f.issueErr
}

func (f *fakeTokens) ConsumeRefresh(_ context.Context, _ string) (string, error) {
	return f.consumeID, f.consumeErr
}

func (f *fakeTokens) Revoke(_ context.Context, token string) error {
	f.revoked = token
	return f.revokeErr
}

func (f *fakeTokens) ParseAccessToken(_ string) (*Claims, error) {
	return f.parseClaims, f.parseErr
}

type fakeUsers struct {
	byID     map[string]*users.User
	authUser *users.User
	authErr  error
}

func (f *fakeUsers) Authenticate(_ context.Context, _, _ string) (*users.User, error) {
	return f.authUser, f.authErr
}

func (f *fakeUsers) FindByID(_ context.Context, id string) (*users.User, error) {
	u, ok := f.byID[id]
	if !ok {
		return nil, users.ErrNotFound
	}
	return u, nil
}

func newTestRouter(tokens Service, userSvc UserService) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	NewHandler(tokens, userSvc).RegisterRoutes(r.Group("/api/v1"))
	return r
}

func perform(r http.Handler, method, path, body, bearer string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if body != "" {
		req.Header.Set("Content-Type", "application/json")
	}
	if bearer != "" {
		req.Header.Set("Authorization", "Bearer "+bearer)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func errorCode(t *testing.T, w *httptest.ResponseRecorder) string {
	t.Helper()
	var body struct {
		Error struct {
			Code string `json:"code"`
		} `json:"error"`
	}
	if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode error body %q: %v", w.Body.String(), err)
	}
	return body.Error.Code
}

func TestLoginSuccess(t *testing.T) {
	email := "manager@example.com"
	tokens := &fakeTokens{pair: TokenPair{AccessToken: "access", RefreshToken: "refresh", ExpiresIn: 900}}
	userSvc := &fakeUsers{authUser: &users.User{ID: "u1", Name: "Manager", Email: &email, Role: users.RoleManager, Status: users.StatusActive}}
	r := newTestRouter(tokens, userSvc)

	w := perform(r, http.MethodPost, "/api/v1/auth/login", `{"email":"manager@example.com","password":"pw"}`, "")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if tokens.issuedUser != "u1" || tokens.issuedRole != "manager" {
		t.Fatalf("expected tokens issued for u1/manager, got %q/%q", tokens.issuedUser, tokens.issuedRole)
	}
	var pair TokenPair
	if err := json.Unmarshal(w.Body.Bytes(), &pair); err != nil {
		t.Fatalf("decode pair: %v", err)
	}
	if pair.AccessToken != "access" || pair.RefreshToken != "refresh" {
		t.Fatalf("unexpected pair: %+v", pair)
	}
}

func TestLoginRejectsInvalidCredentials(t *testing.T) {
	tokens := &fakeTokens{}
	userSvc := &fakeUsers{authErr: users.ErrInvalidCredentials}
	r := newTestRouter(tokens, userSvc)

	w := perform(r, http.MethodPost, "/api/v1/auth/login", `{"email":"manager@example.com","password":"wrong"}`, "")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if code := errorCode(t, w); code != "INVALID_CREDENTIALS" {
		t.Fatalf("expected INVALID_CREDENTIALS, got %q", code)
	}
}

func TestLoginRejectsInactiveAccount(t *testing.T) {
	tokens := &fakeTokens{}
	userSvc := &fakeUsers{authErr: users.ErrInactive}
	r := newTestRouter(tokens, userSvc)

	w := perform(r, http.MethodPost, "/api/v1/auth/login", `{"email":"manager@example.com","password":"pw"}`, "")

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
}

func TestLoginValidatesBody(t *testing.T) {
	r := newTestRouter(&fakeTokens{}, &fakeUsers{})

	w := perform(r, http.MethodPost, "/api/v1/auth/login", `{"email":"not-an-email"}`, "")

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", w.Code)
	}
	if code := errorCode(t, w); code != "VALIDATION_ERROR" {
		t.Fatalf("expected VALIDATION_ERROR, got %q", code)
	}
}

func TestRefreshSuccess(t *testing.T) {
	email := "staff@example.com"
	tokens := &fakeTokens{pair: TokenPair{AccessToken: "new-access", RefreshToken: "new-refresh"}, consumeID: "u1"}
	userSvc := &fakeUsers{byID: map[string]*users.User{
		"u1": {ID: "u1", Name: "Staff", Email: &email, Role: users.RoleStaff, Status: users.StatusActive},
	}}
	r := newTestRouter(tokens, userSvc)

	w := perform(r, http.MethodPost, "/api/v1/auth/refresh", `{"refresh_token":"old"}`, "")

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	if tokens.issuedRole != "staff" {
		t.Fatalf("expected role re-resolved to staff, got %q", tokens.issuedRole)
	}
}

func TestRefreshRejectsInvalidToken(t *testing.T) {
	tokens := &fakeTokens{consumeErr: ErrInvalidToken}
	r := newTestRouter(tokens, &fakeUsers{})

	w := perform(r, http.MethodPost, "/api/v1/auth/refresh", `{"refresh_token":"bad"}`, "")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if code := errorCode(t, w); code != "INVALID_REFRESH_TOKEN" {
		t.Fatalf("expected INVALID_REFRESH_TOKEN, got %q", code)
	}
}

func TestLogoutRevokesToken(t *testing.T) {
	tokens := &fakeTokens{}
	r := newTestRouter(tokens, &fakeUsers{})

	w := perform(r, http.MethodPost, "/api/v1/auth/logout", `{"refresh_token":"token-to-revoke"}`, "")

	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
	if tokens.revoked != "token-to-revoke" {
		t.Fatalf("expected token to be revoked, got %q", tokens.revoked)
	}
}

func TestMeRequiresAuthentication(t *testing.T) {
	r := newTestRouter(&fakeTokens{}, &fakeUsers{})

	w := perform(r, http.MethodGet, "/api/v1/me", "", "")

	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if code := errorCode(t, w); code != "UNAUTHENTICATED" {
		t.Fatalf("expected UNAUTHENTICATED, got %q", code)
	}
}

func TestMeReturnsCurrentUser(t *testing.T) {
	cfg := &config.Config{JWTAccessSecret: "test-secret", AccessTokenTTL: time.Minute}
	realTokens := NewService(cfg, nil)
	token, err := realTokens.(*service).signAccessToken("u1", "staff")
	if err != nil {
		t.Fatalf("sign token: %v", err)
	}
	email := "staff@example.com"
	userSvc := &fakeUsers{byID: map[string]*users.User{
		"u1": {ID: "u1", Name: "Staff", Email: &email, Role: users.RoleStaff, Status: users.StatusActive},
	}}
	r := newTestRouter(realTokens, userSvc)

	w := perform(r, http.MethodGet, "/api/v1/me", "", token)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d: %s", w.Code, w.Body.String())
	}
	var me meResponse
	if err := json.Unmarshal(w.Body.Bytes(), &me); err != nil {
		t.Fatalf("decode me: %v", err)
	}
	if me.ID != "u1" || me.Role != "staff" {
		t.Fatalf("unexpected me response: %+v", me)
	}
}
