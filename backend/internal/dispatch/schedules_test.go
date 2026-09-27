package dispatch

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"

	"staff-transport/internal/auth"
	"staff-transport/internal/trips"
)

type fakeRepo struct {
	schedules   map[string]*Schedule
	occurrences map[string]map[string]struct{}
	removed     []string
}

func newFakeRepo() *fakeRepo {
	return &fakeRepo{schedules: map[string]*Schedule{}, occurrences: map[string]map[string]struct{}{}}
}

func (f *fakeRepo) Create(_ context.Context, in CreateInput, _ string) (*Schedule, error) {
	s := &Schedule{
		ID:         "sch1",
		Name:       in.Name,
		TripType:   in.TripType,
		Timezone:   in.Timezone,
		Active:     true,
		StartDate:  in.StartDate,
		EndDate:    in.EndDate,
		Recurrence: in.Recurrence,
		Template:   in.Template,
	}
	f.schedules[s.ID] = s
	return s, nil
}

func (f *fakeRepo) List(_ context.Context, _, _ int) ([]Schedule, int64, error) {
	out := make([]Schedule, 0, len(f.schedules))
	for _, s := range f.schedules {
		out = append(out, *s)
	}
	return out, int64(len(out)), nil
}

func (f *fakeRepo) Get(_ context.Context, id string) (*Schedule, error) {
	if s, ok := f.schedules[id]; ok {
		return s, nil
	}
	return nil, ErrNotFound
}

func (f *fakeRepo) Update(_ context.Context, id string, in UpdateInput, _ string) (*Schedule, error) {
	s, ok := f.schedules[id]
	if !ok {
		return nil, ErrNotFound
	}
	if in.Name != nil {
		s.Name = *in.Name
	}
	if in.Active != nil {
		s.Active = *in.Active
	}
	return s, nil
}

func (f *fakeRepo) SetActive(_ context.Context, id string, active bool, _ string) error {
	s, ok := f.schedules[id]
	if !ok {
		return ErrNotFound
	}
	s.Active = active
	return nil
}

func (f *fakeRepo) ExistingOccurrences(_ context.Context, scheduleID, _, _ string) (map[string]struct{}, error) {
	out := map[string]struct{}{}
	for date := range f.occurrences[scheduleID] {
		out[date] = struct{}{}
	}
	return out, nil
}

func (f *fakeRepo) AddOccurrence(_ context.Context, scheduleID, _, date string) (bool, error) {
	if f.occurrences[scheduleID] == nil {
		f.occurrences[scheduleID] = map[string]struct{}{}
	}
	if _, ok := f.occurrences[scheduleID][date]; ok {
		return false, nil
	}
	f.occurrences[scheduleID][date] = struct{}{}
	return true, nil
}

func (f *fakeRepo) RemoveTrip(_ context.Context, tripID string) error {
	f.removed = append(f.removed, tripID)
	return nil
}

type fakeTrips struct {
	calls int
}

func (f *fakeTrips) Create(_ context.Context, in trips.CreateInput, _ string) (*trips.Trip, error) {
	f.calls++
	return &trips.Trip{ID: "trip-" + in.ScheduledStartAt.Format(dateLayout), Type: in.Type}, nil
}

func allWeekdays() []int { return []int{0, 1, 2, 3, 4, 5, 6} }

func validTemplate() Template {
	return Template{
		ScheduledTime: "08:00",
		Stops:         []TemplateStop{{Type: "pickup"}},
	}
}

func TestCreateValidates(t *testing.T) {
	repo := newFakeRepo()
	svc := NewService(repo, &fakeTrips{})
	ctx := context.Background()
	start := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)

	cases := map[string]CreateInput{
		"blank name":     {Name: " ", TripType: trips.TypeCustom, StartDate: start, Recurrence: Recurrence{Frequency: "weekly", Weekdays: []int{1}}, Template: validTemplate()},
		"bad trip type":  {Name: "S", TripType: "teleport", StartDate: start, Recurrence: Recurrence{Frequency: "weekly", Weekdays: []int{1}}, Template: validTemplate()},
		"bad recurrence": {Name: "S", TripType: trips.TypeCustom, StartDate: start, Recurrence: Recurrence{Frequency: "daily", Weekdays: []int{1}}, Template: validTemplate()},
		"empty weekdays": {Name: "S", TripType: trips.TypeCustom, StartDate: start, Recurrence: Recurrence{Frequency: "weekly"}, Template: validTemplate()},
		"no stops":       {Name: "S", TripType: trips.TypeCustom, StartDate: start, Recurrence: Recurrence{Frequency: "weekly", Weekdays: []int{1}}, Template: Template{ScheduledTime: "08:00"}},
		"bad time":       {Name: "S", TripType: trips.TypeCustom, StartDate: start, Recurrence: Recurrence{Frequency: "weekly", Weekdays: []int{1}}, Template: Template{ScheduledTime: "8am", Stops: []TemplateStop{{Type: "pickup"}}}},
		"no start date":  {Name: "S", TripType: trips.TypeCustom, Recurrence: Recurrence{Frequency: "weekly", Weekdays: []int{1}}, Template: validTemplate()},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			if _, err := svc.Create(ctx, in, "m1"); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestGenerateAllWeekdaysAndIdempotent(t *testing.T) {
	repo := newFakeRepo()
	repo.schedules["sch1"] = &Schedule{
		ID:         "sch1",
		TripType:   trips.TypeCustom,
		Timezone:   "UTC",
		Active:     true,
		StartDate:  time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC),
		Recurrence: Recurrence{Frequency: "weekly", Weekdays: allWeekdays()},
		Template:   validTemplate(),
	}
	creator := &fakeTrips{}
	svc := NewService(repo, creator)
	ctx := context.Background()

	from := time.Date(2030, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2030, 1, 7, 0, 0, 0, 0, time.UTC)

	generated, err := svc.Generate(ctx, "sch1", "m1", from, to)
	if err != nil {
		t.Fatalf("generate: %v", err)
	}
	if generated != 7 || creator.calls != 7 {
		t.Fatalf("expected 7 generated, got %d (calls %d)", generated, creator.calls)
	}

	again, err := svc.Generate(ctx, "sch1", "m1", from, to)
	if err != nil {
		t.Fatalf("regenerate: %v", err)
	}
	if again != 0 {
		t.Fatalf("expected idempotent regeneration to generate 0, got %d", again)
	}
}

func TestGenerateRejectsInactive(t *testing.T) {
	repo := newFakeRepo()
	repo.schedules["sch1"] = &Schedule{ID: "sch1", Active: false, Timezone: "UTC"}
	svc := NewService(repo, &fakeTrips{})

	if _, err := svc.Generate(context.Background(), "sch1", "m1", time.Now(), time.Now()); !errors.Is(err, ErrInactive) {
		t.Fatalf("expected ErrInactive, got %v", err)
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

func TestScheduleRoutesRequireManager(t *testing.T) {
	gin.SetMode(gin.TestMode)
	svc := NewService(newFakeRepo(), &fakeTrips{})
	claims := map[string]*auth.Claims{
		"manager-token": {UserID: "m1", Role: "manager"},
		"staff-token":   {UserID: "s1", Role: "staff"},
	}
	r := gin.New()
	NewHandler(svc, fakeAuth{claims: claims}).RegisterRoutes(r.Group("/api/v1"))

	body := `{"name":"Weekday pickup","trip_type":"staff_pickup","timezone":"UTC","start_date":"2030-01-01","recurrence":{"frequency":"weekly","weekdays":[1,2,3,4,5]},"template":{"scheduled_time":"08:00","stops":[{"type":"pickup"}]}}`

	request := httptest.NewRequest(http.MethodPost, "/api/v1/recurring-schedules", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, request)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/recurring-schedules", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer staff-token")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, request)
	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for staff, got %d", w.Code)
	}

	request = httptest.NewRequest(http.MethodPost, "/api/v1/recurring-schedules", strings.NewReader(body))
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Authorization", "Bearer manager-token")
	w = httptest.NewRecorder()
	r.ServeHTTP(w, request)
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201 for manager, got %d: %s", w.Code, w.Body.String())
	}

	var response scheduleResponse
	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if response.ID == "" || response.StartDate != "2030-01-01" {
		t.Fatalf("unexpected response: %+v", response)
	}
}
