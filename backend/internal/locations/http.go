package locations

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"staff-transport/internal/auth"
	"staff-transport/internal/httpx"
	"staff-transport/internal/users"
)

// Handler serves driver location ingestion.
type Handler struct {
	svc  *Service
	auth auth.Service
}

// NewHandler returns a location HTTP handler.
func NewHandler(svc *Service, authSvc auth.Service) *Handler {
	return &Handler{svc: svc, auth: authSvc}
}

// RegisterRoutes registers the driver-only location ingestion route on rg.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/trips")
	g.Use(auth.RequireAuth(h.auth), auth.RequireRole(string(users.RoleDriver)))
	g.POST("/:id/locations", h.ingest)
}

type sampleRequest struct {
	Latitude   *float64 `json:"latitude" binding:"required"`
	Longitude  *float64 `json:"longitude" binding:"required"`
	RecordedAt string   `json:"recorded_at" binding:"required"`
	AccuracyM  *float64 `json:"accuracy_m"`
}

type ingestRequest struct {
	Locations []sampleRequest `json:"locations" binding:"required,min=1,max=100,dive"`
}

func (h *Handler) ingest(c *gin.Context) {
	claims, ok := auth.ClaimsFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication is required.")
		return
	}

	var req ingestRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "VALIDATION_ERROR", "Provide a batch of 1-100 location samples.")
		return
	}

	samples := make([]Sample, 0, len(req.Locations))
	for _, item := range req.Locations {
		recordedAt, err := time.Parse(time.RFC3339, item.RecordedAt)
		if err != nil {
			httpx.BadRequest(c, "VALIDATION_ERROR", "Each recorded_at must be an RFC3339 timestamp.")
			return
		}
		samples = append(samples, Sample{
			Latitude:   *item.Latitude,
			Longitude:  *item.Longitude,
			RecordedAt: recordedAt,
			AccuracyM:  item.AccuracyM,
		})
	}

	accepted, err := h.svc.Ingest(c.Request.Context(), c.Param("id"), claims.UserID, samples)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusAccepted, gin.H{"accepted": accepted})
}

func (h *Handler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrForbidden):
		httpx.Forbidden(c, "FORBIDDEN", "You are not assigned to this trip.")
	case errors.Is(err, ErrNotFound):
		httpx.NotFound(c, "TRIP_NOT_FOUND", "Trip not found.")
	case errors.Is(err, ErrTripNotActive):
		httpx.Conflict(c, "TRIP_NOT_ACTIVE", "Location updates are only accepted for an active trip.")
	case errors.Is(err, ErrNoSamples),
		errors.Is(err, ErrBatchTooLarge),
		errors.Is(err, ErrInvalidSample):
		httpx.BadRequest(c, "VALIDATION_ERROR", "One or more location samples are invalid.")
	default:
		httpx.Internal(c, err)
	}
}
