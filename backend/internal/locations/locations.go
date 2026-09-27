package locations

import (
	"context"
	"errors"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"

	"staff-transport/internal/trips"
	"staff-transport/internal/users"
)

const maxBatchSize = 100

var (
	ErrForbidden     = errors.New("not permitted to submit locations for this trip")
	ErrNotFound      = errors.New("trip not found")
	ErrTripNotActive = errors.New("location updates are only accepted for an active or completed trip")
	ErrNoSamples     = errors.New("at least one location sample is required")
	ErrBatchTooLarge = errors.New("too many location samples in one request")
	ErrInvalidSample = errors.New("invalid location sample")
)

// Sample is one device location reading. The server stamps received_at on
// insert; recorded_at is when the device observed the position.
type Sample struct {
	Latitude   float64
	Longitude  float64
	RecordedAt time.Time
	AccuracyM  *float64
}

// Repository appends location telemetry.
type Repository interface {
	InsertBatch(ctx context.Context, driverID, tripID string, samples []Sample) (int, error)
}

type gormRepository struct {
	db *gorm.DB
}

// NewRepository returns a GORM-backed location repository.
func NewRepository(gdb *gorm.DB) Repository {
	return &gormRepository{db: gdb}
}

func (r *gormRepository) InsertBatch(ctx context.Context, driverID, tripID string, samples []Sample) (int, error) {
	inserted := 0
	err := r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, sample := range samples {
			// The unique (driver_id, recorded_at) constraint makes retries of
			// the same device reading a no-op.
			result := tx.Exec(`
				INSERT INTO driver_locations
					(id, driver_id, trip_id, point, recorded_at, received_at, accuracy_m)
				VALUES
					(?, ?, ?, ST_SetSRID(ST_MakePoint(?, ?), 4326), ?, now(), ?)
				ON CONFLICT (driver_id, recorded_at) DO NOTHING`,
				uuid.NewString(),
				driverID,
				tripID,
				sample.Longitude,
				sample.Latitude,
				sample.RecordedAt.UTC(),
				sample.AccuracyM,
			)
			if result.Error != nil {
				return result.Error
			}
			inserted += int(result.RowsAffected)
		}
		return nil
	})
	if err != nil {
		return 0, err
	}
	return inserted, nil
}

// TripReader resolves a trip the caller is authorized to see.
type TripReader interface {
	MyTrip(ctx context.Context, userID, role, tripID string) (*trips.Trip, error)
}

// Service ingests driver location telemetry.
type Service struct {
	repo  Repository
	trips TripReader
}

// NewService returns a location service.
func NewService(repo Repository, tripReader TripReader) *Service {
	return &Service{repo: repo, trips: tripReader}
}

// Ingest accepts a batch of driver location samples for a trip. The caller must
// be the trip's assigned driver. Samples are append-only and deduplicated by
// device timestamp, so retries and out-of-order deliveries are safe.
func (s *Service) Ingest(ctx context.Context, tripID, userID string, samples []Sample) (int, error) {
	if len(samples) == 0 {
		return 0, ErrNoSamples
	}
	if len(samples) > maxBatchSize {
		return 0, ErrBatchTooLarge
	}
	for _, sample := range samples {
		if sample.RecordedAt.IsZero() || !validCoordinate(sample.Latitude, sample.Longitude) {
			return 0, ErrInvalidSample
		}
	}

	trip, err := s.trips.MyTrip(ctx, userID, string(users.RoleDriver), tripID)
	if err != nil {
		switch {
		case errors.Is(err, trips.ErrForbidden):
			return 0, ErrForbidden
		case errors.Is(err, trips.ErrNotFound):
			return 0, ErrNotFound
		default:
			return 0, err
		}
	}
	if trip.Status != trips.StatusInProgress && trip.Status != trips.StatusCompleted {
		return 0, ErrTripNotActive
	}
	if trip.DriverID == nil {
		return 0, ErrForbidden
	}

	return s.repo.InsertBatch(ctx, *trip.DriverID, tripID, samples)
}

func validCoordinate(latitude, longitude float64) bool {
	return latitude >= -90 && latitude <= 90 && longitude >= -180 && longitude <= 180
}
