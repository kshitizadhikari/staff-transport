package locations

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
	"staff-transport/internal/trips"
)

type fakeRepo struct {
	inserted int
	calls    int
	last     []Sample
}

func (f *fakeRepo) InsertBatch(_ context.Context, _, _ string, samples []Sample) (int, error) {
	f.calls++
	f.last = samples
	return f.inserted, nil
}

type fakeTrips struct {
	trip *trips.Trip
	err  error
}

func (f fakeTrips) MyTrip(_ context.Context, _, _, _ string) (*trips.Trip, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.trip, nil
}

func validSample() Sample {
	return Sample{Latitude: 27.7, Longitude: 85.3, RecordedAt: time.Now()}
}

func newService(repo *fakeRepo, trip *trips.Trip, err error) *Service {
	return NewService(repo, fakeTrips{trip: trip, err: err})
}

func TestIngestValidates(t *testing.T) {
	repo := &fakeRepo{}
	svc := newService(repo, nil, nil)
	ctx := context.Background()

	if _, err := svc.Ingest(ctx, "t1", "u1", nil); !errors.Is(err, ErrNoSamples) {
		t.Fatalf("expected ErrNoSamples, got %v", err)
	}

	many := make([]Sample, maxBatchSize+1)
	for i := range many {
		many[i] = validSample()
	}
	if _, err := svc.Ingest(ctx, "t1", "u1", many); !errors.Is(err, ErrBatchTooLarge) {
		t.Fatalf("expected ErrBatchTooLarge, got %v", err)
	}

	bad := validSample()
	bad.Latitude = 120
	if _, err := svc.Ingest(ctx, "t1", "u1", []Sample{bad}); !errors.Is(err, ErrInvalidSample) {
		t.Fatalf("expected ErrInvalidSample for bad latitude, got %v", err)
	}

	zero := Sample{Latitude: 27.7, Longitude: 85.3}
	if _, err := svc.Ingest(ctx, "t1", "u1", []Sample{zero}); !errors.Is(err, ErrInvalidSample) {
		t.Fatalf("expected ErrInvalidSample for zero time, got %v", err)
	}
	if repo.calls != 0 {
		t.Fatalf("expected no repository calls, got %d", repo.calls)
	}
}

func TestIngestMapsAuthorizationErrors(t *testing.T) {
	ctx := context.Background()

	forbidden := newService(&fakeRepo{}, nil, trips.ErrForbidden)
	if _, err := forbidden.Ingest(ctx, "t1", "u1", []Sample{validSample()}); !errors.Is(err, ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}

	missing := newService(&fakeRepo{}, nil, trips.ErrNotFound)
	if _, err := missing.Ingest(ctx, "t1", "u1", []Sample{validSample()}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestIngestRejectsInactiveTrip(t *testing.T) {
	svc := newService(&fakeRepo{}, &trips.Trip{ID: "t1", Status: trips.StatusAssigned, DriverID: strptr("d1")}, nil)
	if _, err := svc.Ingest(context.Background(), "t1", "u1", []Sample{validSample()}); !errors.Is(err, ErrTripNotActive) {
		t.Fatalf("expected ErrTripNotActive, got %v", err)
	}
}

func TestIngestAcceptsActiveAndCompletedTrips(t *testing.T) {
	ctx := context.Background()

	for _, status := range []string{trips.StatusInProgress, trips.StatusCompleted} {
		repo := &fakeRepo{inserted: 1}
		svc := newService(repo, &trips.Trip{ID: "t1", Status: status, DriverID: strptr("d1")}, nil)
		accepted, err := svc.Ingest(ctx, "t1", "u1", []Sample{validSample()})
		if err != nil {
			t.Fatalf("status %s: %v", status, err)
		}
		if accepted != 1 || repo.calls != 1 {
			t.Fatalf("status %s: expected 1 accepted and 1 call, got %d/%d", status, accepted, repo.calls)
		}
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

func newRouter(trip *trips.Trip) *gin.Engine {
	gin.SetMode(gin.TestMode)
	r := gin.New()
	svc := newService(&fakeRepo{inserted: 1}, trip, nil)
	claims := map[string]*auth.Claims{
		"driver-token":  {UserID: "driver-user", Role: "driver"},
		"manager-token": {UserID: "m1", Role: "manager"},
	}
	NewHandler(svc, fakeAuth{claims: claims}).RegisterRoutes(r.Group("/api/v1"))
	return r
}

func perform(r http.Handler, body, token string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodPost, "/api/v1/trips/t1/locations", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func TestLocationRoutesRequireDriver(t *testing.T) {
	r := newRouter(&trips.Trip{ID: "t1", Status: trips.StatusInProgress, DriverID: strptr("d1")})
	body := `{"locations":[{"latitude":27.7,"longitude":85.3,"recorded_at":"2030-01-02T08:00:00Z"}]}`

	if w := perform(r, body, ""); w.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", w.Code)
	}
	if w := perform(r, body, "manager-token"); w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 for manager, got %d", w.Code)
	}
	if w := perform(r, body, "driver-token"); w.Code != http.StatusAccepted {
		t.Fatalf("expected 202 for driver, got %d: %s", w.Code, w.Body.String())
	}
}

func TestLocationIngestHandlerValidates(t *testing.T) {
	r := newRouter(&trips.Trip{ID: "t1", Status: trips.StatusInProgress, DriverID: strptr("d1")})

	w := perform(r, `{"locations":[]}`, "driver-token")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for empty batch, got %d", w.Code)
	}
	w = perform(r, `{"locations":[{"latitude":27.7,"longitude":85.3,"recorded_at":"nope"}]}`, "driver-token")
	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for bad timestamp, got %d", w.Code)
	}
}

func strptr(value string) *string { return &value }
