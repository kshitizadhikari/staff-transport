package trips

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
	"staff-transport/internal/drivers"
	"staff-transport/internal/staff"
	"staff-transport/internal/vehicles"
)

type fakeRepo struct {
	trips            map[string]*Trip
	eventExists      bool
	lastCreate       *CreateInput
	lastCreateStatus string
	lastUpdate       *UpdateInput
	lastUpdateStatus string
	cancelled        string
	lastAction       string
}

func newFakeRepo(trips ...*Trip) *fakeRepo {
	r := &fakeRepo{trips: map[string]*Trip{}}
	for _, t := range trips {
		r.trips[t.ID] = t
	}
	return r
}

func (f *fakeRepo) List(_ context.Context, _ ListFilter) ([]Trip, int64, error) {
	out := make([]Trip, 0, len(f.trips))
	for _, t := range f.trips {
		out = append(out, *t)
	}
	return out, int64(len(out)), nil
}

func (f *fakeRepo) Get(_ context.Context, id string) (*Trip, error) {
	if t, ok := f.trips[id]; ok {
		return t, nil
	}
	return nil, ErrNotFound
}

func (f *fakeRepo) Create(_ context.Context, in CreateInput, status, _ string) (*Trip, error) {
	f.lastCreate = &in
	f.lastCreateStatus = status
	return &Trip{ID: "t1", Type: in.Type, Status: status, Stops: make([]Stop, len(in.Stops))}, nil
}

func (f *fakeRepo) Update(_ context.Context, id string, in UpdateInput, status, _ string) (*Trip, error) {
	f.lastUpdate = &in
	f.lastUpdateStatus = status
	return &Trip{ID: id, Status: status}, nil
}

func (f *fakeRepo) Cancel(_ context.Context, id, _ string) error {
	f.cancelled = id
	return nil
}

func (f *fakeRepo) ListForStaff(_ context.Context, _ string, filter ListFilter) ([]Trip, int64, error) {
	return f.List(context.Background(), filter)
}

func (f *fakeRepo) StartTrip(ctx context.Context, tripID, _ string) (*Trip, error) {
	t, err := f.Get(ctx, tripID)
	if err != nil {
		return nil, err
	}
	if t.Status == StatusInProgress {
		return t, nil
	}
	if t.Status != StatusAssigned {
		return nil, ErrInvalidTransition
	}
	f.lastAction = "start"
	t.Status = StatusInProgress
	return t, nil
}

func (f *fakeRepo) CompleteTrip(ctx context.Context, tripID, _ string) (*Trip, error) {
	t, err := f.Get(ctx, tripID)
	if err != nil {
		return nil, err
	}
	if t.Status == StatusCompleted {
		return t, nil
	}
	if t.Status != StatusInProgress {
		return nil, ErrInvalidTransition
	}
	f.lastAction = "complete"
	t.Status = StatusCompleted
	return t, nil
}

func (f *fakeRepo) ArriveStop(ctx context.Context, tripID, stopID, _ string) (*Trip, error) {
	return f.execStop(ctx, tripID, stopID, StopArrived)
}

func (f *fakeRepo) DepartStop(ctx context.Context, tripID, stopID, _ string) (*Trip, error) {
	return f.execStop(ctx, tripID, stopID, StopDeparted)
}

func (f *fakeRepo) execStop(ctx context.Context, tripID, stopID, status string) (*Trip, error) {
	t, err := f.Get(ctx, tripID)
	if err != nil {
		return nil, err
	}
	if t.Status != StatusInProgress {
		return nil, ErrInvalidTransition
	}
	for i := range t.Stops {
		if t.Stops[i].ID == stopID {
			if t.Stops[i].Status == status {
				return t, nil
			}
			if status == StopDeparted && t.Stops[i].Status != StopArrived {
				return nil, ErrInvalidTransition
			}
			t.Stops[i].Status = status
			f.lastAction = "stop:" + status
			return t, nil
		}
	}
	return nil, ErrStopNotFound
}

func (f *fakeRepo) PickupPassenger(ctx context.Context, tripID, passengerID, _ string) (*Trip, error) {
	return f.execPassenger(ctx, tripID, passengerID, PassengerPickedUp)
}

func (f *fakeRepo) NoShowPassenger(ctx context.Context, tripID, passengerID, _ string) (*Trip, error) {
	return f.execPassenger(ctx, tripID, passengerID, PassengerNoShow)
}

func (f *fakeRepo) execPassenger(ctx context.Context, tripID, passengerID, status string) (*Trip, error) {
	t, err := f.Get(ctx, tripID)
	if err != nil {
		return nil, err
	}
	if t.Status != StatusInProgress {
		return nil, ErrInvalidTransition
	}
	for i := range t.Passengers {
		if t.Passengers[i].ID == passengerID {
			if t.Passengers[i].Status == status {
				return t, nil
			}
			if t.Passengers[i].Status != PassengerAssigned {
				return nil, ErrInvalidTransition
			}
			t.Passengers[i].Status = status
			f.lastAction = "passenger:" + status
			return t, nil
		}
	}
	return nil, ErrPassengerNotFound
}

func (f *fakeRepo) EventExists(_ context.Context, _ string) (bool, error) {
	return f.eventExists, nil
}

type fakeDrivers struct {
	status string
	err    error
	id     string
	userID string
}

func (f fakeDrivers) Get(_ context.Context, id string) (*drivers.Driver, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &drivers.Driver{ID: id, Status: f.status}, nil
}

func (f fakeDrivers) FindByUserID(_ context.Context, userID string) (*drivers.Driver, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.userID == "" || f.userID != userID {
		return nil, drivers.ErrNotFound
	}
	return &drivers.Driver{ID: f.id, Status: f.status}, nil
}

type fakeVehicles struct {
	capacity int
	status   string
	err      error
}

func (f fakeVehicles) Get(_ context.Context, id string) (*vehicles.Vehicle, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &vehicles.Vehicle{ID: id, Capacity: f.capacity, Status: f.status}, nil
}

type fakeStaff struct {
	active bool
	err    error
	id     string
	userID string
}

func (f fakeStaff) Get(_ context.Context, id string) (*staff.Staff, error) {
	if f.err != nil {
		return nil, f.err
	}
	return &staff.Staff{ID: id, Active: f.active}, nil
}

func (f fakeStaff) FindByUserID(_ context.Context, userID string) (*staff.Staff, error) {
	if f.err != nil {
		return nil, f.err
	}
	if f.userID == "" || f.userID != userID {
		return nil, staff.ErrNotFound
	}
	return &staff.Staff{ID: f.id, Active: f.active}, nil
}

func ptr(s string) *string { return &s }

func newService(repo Repository, d fakeDrivers, v fakeVehicles, s fakeStaff) *Service {
	return NewService(repo, d, v, s, nil, "UTC")
}

func validCreate() CreateInput {
	return CreateInput{
		Type:             TypeStaffPickup,
		ScheduledStartAt: time.Date(2030, 1, 2, 8, 0, 0, 0, time.UTC),
		Stops:            []StopInput{{Type: StopPickup}, {Type: StopDropoff}},
		DriverID:         ptr("d1"),
		VehicleID:        ptr("v1"),
	}
}

func TestCreateSetsAssignedStatus(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo, fakeDrivers{status: drivers.StatusAvailable}, fakeVehicles{capacity: 4, status: vehicles.StatusAvailable}, fakeStaff{active: true})

	created, err := svc.Create(context.Background(), validCreate(), "m1")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if created.Status != StatusAssigned || repo.lastCreateStatus != StatusAssigned {
		t.Fatalf("expected assigned status, got %q / %q", created.Status, repo.lastCreateStatus)
	}
}

func TestCreateWithoutAssignmentIsScheduled(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo, fakeDrivers{}, fakeVehicles{}, fakeStaff{})

	in := validCreate()
	in.DriverID = nil
	in.VehicleID = nil

	if _, err := svc.Create(context.Background(), in, "m1"); err != nil {
		t.Fatalf("create: %v", err)
	}
	if repo.lastCreateStatus != StatusScheduled {
		t.Fatalf("expected scheduled status, got %q", repo.lastCreateStatus)
	}
}

func TestCreateValidates(t *testing.T) {
	repo := newFakeRepo()
	svc := newService(repo, fakeDrivers{status: drivers.StatusAvailable}, fakeVehicles{capacity: 1, status: vehicles.StatusAvailable}, fakeStaff{active: true})

	noStops := validCreate()
	noStops.Stops = nil
	if _, err := svc.Create(context.Background(), noStops, "m1"); !errors.Is(err, ErrStopsRequired) {
		t.Fatalf("expected ErrStopsRequired, got %v", err)
	}

	badType := validCreate()
	badType.Type = "teleport"
	if _, err := svc.Create(context.Background(), badType, "m1"); !errors.Is(err, ErrInvalidType) {
		t.Fatalf("expected ErrInvalidType, got %v", err)
	}

	badStop := validCreate()
	badStop.Stops[0].Type = "portal"
	if _, err := svc.Create(context.Background(), badStop, "m1"); !errors.Is(err, ErrInvalidStopType) {
		t.Fatalf("expected ErrInvalidStopType, got %v", err)
	}

	overCapacity := validCreate()
	overCapacity.Passengers = []PassengerInput{{StaffID: "s1"}, {StaffID: "s2"}}
	if _, err := svc.Create(context.Background(), overCapacity, "m1"); !errors.Is(err, ErrCapacityExceeded) {
		t.Fatalf("expected ErrCapacityExceeded, got %v", err)
	}

	dup := validCreate()
	dup.Passengers = []PassengerInput{{StaffID: "s1"}, {StaffID: "s1"}}
	svcWide := newService(newFakeRepo(), fakeDrivers{}, fakeVehicles{capacity: 4}, fakeStaff{active: true})
	if _, err := svcWide.Create(context.Background(), dup, "m1"); !errors.Is(err, ErrDuplicatePassenger) {
		t.Fatalf("expected ErrDuplicatePassenger, got %v", err)
	}
}

func TestUpdateRejectsFinalStates(t *testing.T) {
	for _, status := range []string{StatusCompleted, StatusCancelled} {
		repo := newFakeRepo(&Trip{ID: "t1", Status: status, Stops: []Stop{{ID: "s1"}}})
		svc := newService(repo, fakeDrivers{}, fakeVehicles{}, fakeStaff{})
		if _, err := svc.Update(context.Background(), "t1", UpdateInput{}, "m1"); !errors.Is(err, ErrNotEditable) {
			t.Fatalf("status %s: expected ErrNotEditable, got %v", status, err)
		}
	}
}

func TestUpdateUnassignsDriver(t *testing.T) {
	repo := newFakeRepo(&Trip{
		ID:        "t1",
		Status:    StatusAssigned,
		DriverID:  ptr("d1"),
		VehicleID: ptr("v1"),
		Stops:     []Stop{{ID: "s1"}},
	})
	svc := newService(repo, fakeDrivers{}, fakeVehicles{}, fakeStaff{})

	empty := ""
	if _, err := svc.Update(context.Background(), "t1", UpdateInput{DriverID: &empty}, "m1"); err != nil {
		t.Fatalf("update: %v", err)
	}
	if repo.lastUpdateStatus != StatusScheduled {
		t.Fatalf("expected scheduled after unassigning driver, got %q", repo.lastUpdateStatus)
	}
	if repo.lastUpdate == nil || repo.lastUpdate.DriverID == nil || *repo.lastUpdate.DriverID != "" {
		t.Fatalf("expected driver_id provided as empty, got %+v", repo.lastUpdate)
	}
}

func TestCancel(t *testing.T) {
	repo := newFakeRepo(&Trip{ID: "t1", Status: StatusScheduled})
	svc := newService(repo, fakeDrivers{}, fakeVehicles{}, fakeStaff{})

	if err := svc.Cancel(context.Background(), "t1", "m1"); err != nil {
		t.Fatalf("cancel: %v", err)
	}
	if repo.cancelled != "t1" {
		t.Fatalf("expected repo cancel call, got %q", repo.cancelled)
	}
}

func TestCancelFinalStates(t *testing.T) {
	completed := newFakeRepo(&Trip{ID: "t1", Status: StatusCompleted})
	svc := newService(completed, fakeDrivers{}, fakeVehicles{}, fakeStaff{})
	if err := svc.Cancel(context.Background(), "t1", "m1"); !errors.Is(err, ErrNotCancellable) {
		t.Fatalf("expected ErrNotCancellable, got %v", err)
	}
	if completed.cancelled != "" {
		t.Fatal("expected no repo cancel for completed trip")
	}

	already := newFakeRepo(&Trip{ID: "t2", Status: StatusCancelled})
	svc = newService(already, fakeDrivers{}, fakeVehicles{}, fakeStaff{})
	if err := svc.Cancel(context.Background(), "t2", "m1"); err != nil {
		t.Fatalf("expected idempotent cancel, got %v", err)
	}
	if already.cancelled != "" {
		t.Fatal("expected no repo cancel for already-cancelled trip")
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

func newTripRouter(repo *fakeRepo, claims map[string]*auth.Claims) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	svc := newService(repo, fakeDrivers{status: drivers.StatusAvailable}, fakeVehicles{capacity: 4, status: vehicles.StatusAvailable}, fakeStaff{active: true})
	NewHandler(svc, fakeAuth{claims: claims}).RegisterRoutes(r.Group("/api/v1"))
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

func TestTripRoutesRequireManager(t *testing.T) {
	claims := map[string]*auth.Claims{
		"manager-token": {UserID: "m1", Role: "manager"},
		"staff-token":   {UserID: "s1", Role: "staff"},
	}
	r := newTripRouter(newFakeRepo(), claims)

	if w := perform(r, http.MethodGet, "/api/v1/trips", "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if w := perform(r, http.MethodGet, "/api/v1/trips", "", "staff-token"); w.Code != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", w.Code)
	}
	if w := perform(r, http.MethodGet, "/api/v1/trips", "", "manager-token"); w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestCreateTripHandler(t *testing.T) {
	claims := map[string]*auth.Claims{"manager-token": {UserID: "m1", Role: "manager"}}
	r := newTripRouter(newFakeRepo(), claims)

	if w := perform(r, http.MethodPost, "/api/v1/trips", `{"type":"staff_pickup","scheduled_start_at":"2030-01-02T08:00:00Z"}`, "manager-token"); w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 without stops, got %d", w.Code)
	}

	body := `{"type":"staff_pickup","scheduled_start_at":"2030-01-02T08:00:00Z","driver_id":"d1","vehicle_id":"v1","stops":[{"type":"pickup","address":"A"},{"type":"dropoff","address":"B"}]}`
	w := perform(r, http.MethodPost, "/api/v1/trips", body, "manager-token")
	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d: %s", w.Code, w.Body.String())
	}
}

func TestCancelTripHandler(t *testing.T) {
	claims := map[string]*auth.Claims{"manager-token": {UserID: "m1", Role: "manager"}}
	r := newTripRouter(newFakeRepo(&Trip{ID: "t1", Status: StatusScheduled}), claims)

	w := perform(r, http.MethodPost, "/api/v1/trips/t1/cancel", "", "manager-token")
	if w.Code != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", w.Code)
	}
}

func executionService(repo *fakeRepo) *Service {
	return NewService(repo,
		fakeDrivers{id: "d1", userID: "driver-user", status: drivers.StatusAvailable},
		fakeVehicles{capacity: 4, status: vehicles.StatusAvailable},
		fakeStaff{id: "s1", userID: "staff-user", active: true},
		nil,
		"UTC")
}

func TestAuthorizeDriverRejectsOtherDriver(t *testing.T) {
	repo := newFakeRepo(&Trip{ID: "t1", Status: StatusAssigned, DriverID: ptr("d1")})
	svc := executionService(repo)

	if _, err := svc.StartTrip(context.Background(), "t1", "someone-else"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestStartTrip(t *testing.T) {
	repo := newFakeRepo(&Trip{ID: "t1", Status: StatusAssigned, DriverID: ptr("d1")})
	svc := executionService(repo)

	trip, err := svc.StartTrip(context.Background(), "t1", "driver-user")
	if err != nil {
		t.Fatalf("start: %v", err)
	}
	if trip.Status != StatusInProgress || repo.lastAction != "start" {
		t.Fatalf("expected in_progress/start, got %q/%q", trip.Status, repo.lastAction)
	}
}

func TestStartTripRequiresAssignedState(t *testing.T) {
	repo := newFakeRepo(&Trip{ID: "t1", Status: StatusScheduled, DriverID: ptr("d1")})
	svc := executionService(repo)

	if _, err := svc.StartTrip(context.Background(), "t1", "driver-user"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition, got %v", err)
	}
}

func TestCompleteTripIdempotent(t *testing.T) {
	repo := newFakeRepo(&Trip{ID: "t1", Status: StatusCompleted, DriverID: ptr("d1")})
	svc := executionService(repo)

	trip, err := svc.CompleteTrip(context.Background(), "t1", "driver-user")
	if err != nil {
		t.Fatalf("complete: %v", err)
	}
	if trip.Status != StatusCompleted || repo.lastAction != "" {
		t.Fatalf("expected idempotent complete, got %q/%q", trip.Status, repo.lastAction)
	}
}

func TestStopExecution(t *testing.T) {
	repo := newFakeRepo(&Trip{
		ID:         "t1",
		Status:     StatusInProgress,
		DriverID:   ptr("d1"),
		Stops:      []Stop{{ID: "s1", Status: StopPending}},
		Passengers: []Passenger{{ID: "p1", StaffID: "s1", Status: PassengerAssigned}},
	})
	svc := executionService(repo)
	ctx := context.Background()

	if _, err := svc.DepartStop(ctx, "t1", "s1", "driver-user"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition departing pending stop, got %v", err)
	}
	arrived, err := svc.ArriveStop(ctx, "t1", "s1", "driver-user")
	if err != nil || arrived.Stops[0].Status != StopArrived {
		t.Fatalf("arrive: %v status=%q", err, arrived.Stops[0].Status)
	}
	departed, err := svc.DepartStop(ctx, "t1", "s1", "driver-user")
	if err != nil || departed.Stops[0].Status != StopDeparted {
		t.Fatalf("depart: %v status=%q", err, departed.Stops[0].Status)
	}
	if _, err := svc.ArriveStop(ctx, "t1", "missing", "driver-user"); !errors.Is(err, ErrStopNotFound) {
		t.Fatalf("expected ErrStopNotFound, got %v", err)
	}
}

func TestPassengerExecution(t *testing.T) {
	repo := newFakeRepo(&Trip{
		ID:         "t1",
		Status:     StatusInProgress,
		DriverID:   ptr("d1"),
		Passengers: []Passenger{{ID: "p1", StaffID: "s1", Status: PassengerAssigned}},
	})
	svc := executionService(repo)
	ctx := context.Background()

	picked, err := svc.PickupPassenger(ctx, "t1", "p1", "driver-user")
	if err != nil || picked.Passengers[0].Status != PassengerPickedUp {
		t.Fatalf("pickup: %v status=%q", err, picked.Passengers[0].Status)
	}
	if _, err := svc.NoShowPassenger(ctx, "t1", "p1", "driver-user"); !errors.Is(err, ErrInvalidTransition) {
		t.Fatalf("expected ErrInvalidTransition for picked-up passenger no-show, got %v", err)
	}
	if _, err := svc.PickupPassenger(ctx, "t1", "missing", "driver-user"); !errors.Is(err, ErrPassengerNotFound) {
		t.Fatalf("expected ErrPassengerNotFound, got %v", err)
	}
}

func TestMyTrips(t *testing.T) {
	repo := newFakeRepo(&Trip{ID: "t1", DriverID: ptr("d1")})
	svc := executionService(repo)
	ctx := context.Background()

	if _, _, err := svc.MyTrips(ctx, "driver-user", "driver", ListFilter{}, ""); err != nil {
		t.Fatalf("driver my trips: %v", err)
	}
	if _, _, err := svc.MyTrips(ctx, "staff-user", "staff", ListFilter{}, ""); err != nil {
		t.Fatalf("staff my trips: %v", err)
	}
	if _, _, err := svc.MyTrips(ctx, "m1", "manager", ListFilter{}, ""); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for manager, got %v", err)
	}
}

func newExecutionRouter(repo *fakeRepo, claims map[string]*auth.Claims) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	NewHandler(executionService(repo), fakeAuth{claims: claims}).RegisterRoutes(r.Group("/api/v1"))
	return r
}

func TestExecutionRoutesAuthorization(t *testing.T) {
	repo := newFakeRepo(&Trip{ID: "t1", Status: StatusAssigned, DriverID: ptr("d1")})
	claims := map[string]*auth.Claims{
		"driver-token":  {UserID: "driver-user", Role: "driver"},
		"manager-token": {UserID: "m1", Role: "manager"},
	}
	r := newExecutionRouter(repo, claims)

	if w := perform(r, http.MethodPost, "/api/v1/trips/t1/start", "", ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if w := perform(r, http.MethodPost, "/api/v1/trips/t1/start", "", "manager-token"); w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for manager, got %d", w.Code)
	}
	if w := perform(r, http.MethodPost, "/api/v1/trips/t1/start", "", "driver-token"); w.Code != http.StatusOK {
		t.Fatalf("expected 200 for driver, got %d: %s", w.Code, w.Body.String())
	}
	if w := perform(r, http.MethodGet, "/api/v1/me/trips", "", "driver-token"); w.Code != http.StatusOK {
		t.Fatalf("expected 200 for /me/trips, got %d", w.Code)
	}
}

func TestMyTripAuthorizes(t *testing.T) {
	repo := newFakeRepo(&Trip{
		ID:         "t1",
		DriverID:   ptr("d1"),
		Passengers: []Passenger{{ID: "p1", StaffID: "s1"}},
	})
	svc := executionService(repo)
	ctx := context.Background()

	if _, err := svc.MyTrip(ctx, "driver-user", "driver", "t1"); err != nil {
		t.Fatalf("assigned driver should see trip: %v", err)
	}
	if _, err := svc.MyTrip(ctx, "staff-user", "staff", "t1"); err != nil {
		t.Fatalf("assigned passenger should see trip: %v", err)
	}
	if _, err := svc.MyTrip(ctx, "m1", "manager", "t1"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for manager, got %v", err)
	}

	other := newFakeRepo(&Trip{ID: "t2", DriverID: ptr("d2")})
	if _, err := executionService(other).MyTrip(ctx, "driver-user", "driver", "t2"); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden for other driver, got %v", err)
	}
}
