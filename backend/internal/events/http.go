package events

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"staff-transport/internal/auth"
	"staff-transport/internal/httpx"
	"staff-transport/internal/users"
)

// Handler serves manager-facing event endpoints.
type Handler struct {
	svc  *Service
	auth auth.Service
}

// NewHandler returns an event HTTP handler.
func NewHandler(svc *Service, authSvc auth.Service) *Handler {
	return &Handler{svc: svc, auth: authSvc}
}

// RegisterRoutes registers manager-only event routes on rg.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/events")
	g.Use(auth.RequireAuth(h.auth), auth.RequireRole(string(users.RoleManager)))

	g.GET("", h.list)
	g.POST("", h.create)
	g.GET("/:id", h.get)
	g.PATCH("/:id", h.update)
	g.DELETE("/:id", h.delete)
	g.POST("/:id/participants", h.addParticipant)
	g.DELETE("/:id/participants/:staffId", h.removeParticipant)
}

type createRequest struct {
	Name      string   `json:"name" binding:"required,max=200"`
	Address   *string  `json:"address" binding:"omitempty,max=500"`
	StartsAt  *string  `json:"starts_at"`
	EndsAt    *string  `json:"ends_at"`
	Notes     *string  `json:"notes" binding:"omitempty,max=2000"`
	Latitude  *float64 `json:"latitude" binding:"omitempty,gte=-90,lte=90"`
	Longitude *float64 `json:"longitude" binding:"omitempty,gte=-180,lte=180"`
}

type updateRequest struct {
	Name      *string  `json:"name" binding:"omitempty,max=200"`
	Address   *string  `json:"address" binding:"omitempty,max=500"`
	StartsAt  *string  `json:"starts_at"`
	EndsAt    *string  `json:"ends_at"`
	Notes     *string  `json:"notes" binding:"omitempty,max=2000"`
	Latitude  *float64 `json:"latitude" binding:"omitempty,gte=-90,lte=90"`
	Longitude *float64 `json:"longitude" binding:"omitempty,gte=-180,lte=180"`
}

type participantRequest struct {
	StaffID string `json:"staff_id" binding:"required"`
}

type participantResponse struct {
	StaffID string `json:"staff_id"`
	Name    string `json:"name"`
}

type tripResponse struct {
	ID               string    `json:"id"`
	Type             string    `json:"type"`
	Status           string    `json:"status"`
	ScheduledStartAt time.Time `json:"scheduled_start_at"`
	DriverName       *string   `json:"driver_name,omitempty"`
	VehicleReg       *string   `json:"vehicle_registration,omitempty"`
}

type eventResponse struct {
	ID           string                `json:"id"`
	Name         string                `json:"name"`
	Address      *string               `json:"address,omitempty"`
	StartsAt     *time.Time            `json:"starts_at,omitempty"`
	EndsAt       *time.Time            `json:"ends_at,omitempty"`
	Notes        *string               `json:"notes,omitempty"`
	Participants []participantResponse `json:"participants"`
	Trips        []tripResponse        `json:"trips"`
	CreatedAt    time.Time             `json:"created_at"`
	UpdatedAt    time.Time             `json:"updated_at"`
}

func toResponse(event *Event) eventResponse {
	participants := make([]participantResponse, 0, len(event.Participants))
	for _, participant := range event.Participants {
		participants = append(participants, participantResponse{StaffID: participant.StaffID, Name: participant.Name})
	}
	trips := make([]tripResponse, 0, len(event.Trips))
	for _, trip := range event.Trips {
		trips = append(trips, tripResponse{
			ID:               trip.ID,
			Type:             trip.Type,
			Status:           trip.Status,
			ScheduledStartAt: trip.ScheduledStartAt,
			DriverName:       trip.DriverName,
			VehicleReg:       trip.VehicleReg,
		})
	}
	return eventResponse{
		ID:           event.ID,
		Name:         event.Name,
		Address:      event.Address,
		StartsAt:     event.StartsAt,
		EndsAt:       event.EndsAt,
		Notes:        event.Notes,
		Participants: participants,
		Trips:        trips,
		CreatedAt:    event.CreatedAt,
		UpdatedAt:    event.UpdatedAt,
	}
}

func (h *Handler) list(c *gin.Context) {
	page, pageSize, offset := httpx.PageParams(c)
	items, total, err := h.svc.List(c.Request.Context(), offset, pageSize)
	if err != nil {
		h.writeError(c, err)
		return
	}
	data := make([]eventResponse, 0, len(items))
	for i := range items {
		data = append(data, toResponse(&items[i]))
	}
	c.JSON(http.StatusOK, httpx.ListResponse[eventResponse]{
		Data:       data,
		Pagination: httpx.Pagination{Page: page, PageSize: pageSize, Total: total},
	})
}

func (h *Handler) create(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "VALIDATION_ERROR", "Provide valid event details.")
		return
	}
	startsAt, endsAt, err := parseTimes(req.StartsAt, req.EndsAt)
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_ERROR", "starts_at and ends_at must be RFC3339 timestamps.")
		return
	}

	created, err := h.svc.Create(c.Request.Context(), CreateInput{
		Name:      req.Name,
		Address:   req.Address,
		StartsAt:  startsAt,
		EndsAt:    endsAt,
		Notes:     req.Notes,
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
	}, actorFrom(c))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toResponse(created))
}

func (h *Handler) get(c *gin.Context) {
	event, err := h.svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, toResponse(event))
}

func (h *Handler) update(c *gin.Context) {
	var req updateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "VALIDATION_ERROR", "Provide valid event fields.")
		return
	}
	startsAt, endsAt, err := parseTimes(req.StartsAt, req.EndsAt)
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_ERROR", "starts_at and ends_at must be RFC3339 timestamps.")
		return
	}

	updated, err := h.svc.Update(c.Request.Context(), c.Param("id"), UpdateInput{
		Name:      req.Name,
		Address:   req.Address,
		StartsAt:  startsAt,
		EndsAt:    endsAt,
		Notes:     req.Notes,
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
	}, actorFrom(c))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, toResponse(updated))
}

func (h *Handler) delete(c *gin.Context) {
	if err := h.svc.Delete(c.Request.Context(), c.Param("id"), actorFrom(c)); err != nil {
		h.writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) addParticipant(c *gin.Context) {
	var req participantRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "VALIDATION_ERROR", "staff_id is required.")
		return
	}
	if err := h.svc.AddParticipant(c.Request.Context(), c.Param("id"), req.StaffID, actorFrom(c)); err != nil {
		h.writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) removeParticipant(c *gin.Context) {
	if err := h.svc.RemoveParticipant(c.Request.Context(), c.Param("id"), c.Param("staffId"), actorFrom(c)); err != nil {
		h.writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.NotFound(c, "EVENT_NOT_FOUND", "Event not found.")
	case errors.Is(err, ErrStaffNotFound):
		httpx.BadRequest(c, "STAFF_NOT_FOUND", "The selected staff member does not exist or is inactive.")
	case errors.Is(err, ErrNameRequired), errors.Is(err, ErrInvalidTimeRange):
		httpx.BadRequest(c, "VALIDATION_ERROR", "One or more event fields are invalid.")
	default:
		httpx.Internal(c, err)
	}
}

func actorFrom(c *gin.Context) string {
	if claims, ok := auth.ClaimsFrom(c); ok {
		return claims.UserID
	}
	return ""
}

func parseTimes(startsAt, endsAt *string) (*time.Time, *time.Time, error) {
	start, err := parseOptionalTime(startsAt)
	if err != nil {
		return nil, nil, err
	}
	end, err := parseOptionalTime(endsAt)
	if err != nil {
		return nil, nil, err
	}
	return start, end, nil
}

func parseOptionalTime(value *string) (*time.Time, error) {
	if value == nil || *value == "" {
		return nil, nil
	}
	parsed, err := time.Parse(time.RFC3339, *value)
	if err != nil {
		return nil, err
	}
	parsed = parsed.UTC()
	return &parsed, nil
}
