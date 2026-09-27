package events

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"staff-transport/internal/auth"
	"staff-transport/internal/staff"
)

type fakeRepo struct {
	events            map[string]*Event
	participants      map[string]map[string]struct{}
	lastUpdate        *UpdateInput
	addParticipant    int
	removeParticipant int
	deleted           string
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{events: map[string]*Event{}, participants: map[string]map[string]struct{}{}}
}

func (f *fakeRepo) List(_ context.Context, _, _ int) ([]Event, int64, error) {
	out := make([]Event, 0, len(f.events))
	for _, event := range f.events {
		out = append(out, *event)
	}
	return out, int64(len(out)), nil
}

func (f *fakeRepo) Get(_ context.Context, id string) (*Event, error) {
	if event, ok := f.events[id]; ok {
		return event, nil
	}
	return nil, ErrNotFound
}

func (f *fakeRepo) Create(_ context.Context, in CreateInput, _ string) (*Event, error) {
	event := &Event{ID: "e1", Name: in.Name, Address: in.Address, StartsAt: in.StartsAt, EndsAt: in.EndsAt, Notes: in.Notes}
	f.events[event.ID] = event
	return event, nil
}

func (f *fakeRepo) Update(_ context.Context, id string, in UpdateInput, _ string) (*Event, error) {
	event, ok := f.events[id]
	if !ok {
		return nil, ErrNotFound
	}
	f.lastUpdate = &in
	if in.Name != nil {
		event.Name = *in.Name
	}
	return event, nil
}

func (f *fakeRepo) Delete(_ context.Context, id, _ string) error {
	if _, ok := f.events[id]; !ok {
		return ErrNotFound
	}
	f.deleted = id
	return nil
}

func (f *fakeRepo) AddParticipant(_ context.Context, eventID, staffID, _ string) (bool, error) {
	if f.participants[eventID] == nil {
		f.participants[eventID] = map[string]struct{}{}
	}
	if _, ok := f.participants[eventID][staffID]; ok {
		return false, nil
	}
	f.participants[eventID][staffID] = struct{}{}
	f.addParticipant++
	return true, nil
}

func (f *fakeRepo) RemoveParticipant(_ context.Context, eventID, staffID, _ string) error {
	delete(f.participants[eventID], staffID)
	f.removeParticipant++
	return nil
}

type fakeStaff struct {
	member *staff.Staff
	err    error
}

func (f fakeStaff) Get(_ context.Context, id string) (*staff.Staff, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.member, nil
}

func TestCreateValidates(t *testing.T) {
	svc := NewService(newFakeRepo(), fakeStaff{})
	ctx := context.Background()
	start := time.Date(2030, 1, 2, 10, 0, 0, 0, time.UTC)
	end := time.Date(2030, 1, 2, 9, 0, 0, 0, time.UTC)

	if _, err := svc.Create(ctx, CreateInput{Name: "  "}, "m1"); !errors.Is(err, ErrNameRequired) {
		t.Fatalf("expected ErrNameRequired, got %v", err)
	}
	if _, err := svc.Create(ctx, CreateInput{Name: "Gala", StartsAt: &start, EndsAt: &end}, "m1"); !errors.Is(err, ErrInvalidTimeRange) {
		t.Fatalf("expected ErrInvalidTimeRange, got %v", err)
	}
}

func TestAddParticipantValidatesStaff(t *testing.T) {
	repo := newFakeRepo()
	repo.events["e1"] = &Event{ID: "e1", Name: "Gala"}
	ctx := context.Background()

	missing := NewService(repo, fakeStaff{err: staff.ErrNotFound})
	if err := missing.AddParticipant(ctx, "e1", "s1", "m1"); !errors.Is(err, ErrStaffNotFound) {
		t.Fatalf("expected ErrStaffNotFound, got %v", err)
	}

	inactive := NewService(repo, fakeStaff{member: &staff.Staff{ID: "s1", Active: false}})
	if err := inactive.AddParticipant(ctx, "e1", "s1", "m1"); !errors.Is(err, ErrStaffNotFound) {
		t.Fatalf("expected ErrStaffNotFound for inactive, got %v", err)
	}

	if err := inactive.AddParticipant(ctx, "missing", "s1", "m1"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound for missing event, got %v", err)
	}
}

func TestAddParticipantIsIdempotent(t *testing.T) {
	repo := newFakeRepo()
	repo.events["e1"] = &Event{ID: "e1"}
	svc := NewService(repo, fakeStaff{member: &staff.Staff{ID: "s1", Active: true}})
	ctx := context.Background()

	if err := svc.AddParticipant(ctx, "e1", "s1", "m1"); err != nil {
		t.Fatalf("add: %v", err)
	}
	if err := svc.AddParticipant(ctx, "e1", "s1", "m1"); err != nil {
		t.Fatalf("re-add: %v", err)
	}
	if repo.addParticipant != 1 {
		t.Fatalf("expected one insert, got %d", repo.addParticipant)
	}

	if err := svc.RemoveParticipant(ctx, "e1", "s1", "m1"); err != nil {
		t.Fatalf("remove: %v", err)
	}
	if repo.removeParticipant != 1 {
		t.Fatalf("expected one remove, got %d", repo.removeParticipant)
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
	if claims, ok := f.claims[token]; ok {
		return claims, nil
	}
	return nil, auth.ErrInvalidToken
}

func TestEventRoutesRequireManager(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService(newFakeRepo(), fakeStaff{member: &staff.Staff{ID: "s1", Active: true}})
	claims := map[string]*auth.Claims{
		"manager-token": {UserID: "m1", Role: "manager"},
		"staff-token":   {UserID: "s1", Role: "staff"},
	}
	r := gin.New()
	NewHandler(svc, fakeAuth{claims: claims}).RegisterRoutes(r.Group("/api/v1"))

	body := `{"name":"Company gala","starts_at":"2030-01-02T10:00:00Z","ends_at":"2030-01-02T18:00:00Z"}`

	request := httptest.NewRequest(http.MethodPost, "/api/v1/events", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, request)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/events", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer staff-token")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, request)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for staff, got %d", w.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/events", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer manager-token")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, request)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}
