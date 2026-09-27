package trips

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"staff-transport/internal/auth"
	"staff-transport/internal/httpx"
	"staff-transport/internal/users"
)

// Handler serves manager-facing trip endpoints.
type Handler struct {
	svc  *Service
	auth auth.Service
}

// NewHandler returns a trip HTTP handler.
func NewHandler(svc *Service, authSvc auth.Service) *Handler {
	return &Handler{svc: svc, auth: authSvc}
}

// RegisterRoutes registers manager-only trip routes on rg.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/trips")
	g.Use(auth.RequireAuth(h.auth), auth.RequireRole(string(users.RoleManager)))

	g.GET("", h.list)
	g.POST("", h.create)
	g.GET("/:id", h.get)
	g.PATCH("/:id", h.update)
	g.POST("/:id/cancel", h.cancel)

	h.registerDriverExecution(rg)
}

type stopRequest struct {
	Type        string  `json:"type" binding:"required,max=20"`
	Address     *string `json:"address" binding:"omitempty,max=500"`
	ScheduledAt *string `json:"scheduled_at"`
}

type passengerRequest struct {
	StaffID          string `json:"staff_id" binding:"required"`
	PickupStopIndex  *int   `json:"pickup_stop_index"`
	DropoffStopIndex *int   `json:"dropoff_stop_index"`
}

type createRequest struct {
	Type             string             `json:"type" binding:"required,max=50"`
	ScheduledStartAt string             `json:"scheduled_start_at" binding:"required"`
	DriverID         *string            `json:"driver_id" binding:"omitempty,max=64"`
	VehicleID        *string            `json:"vehicle_id" binding:"omitempty,max=64"`
	EventID          *string            `json:"event_id" binding:"omitempty,max=64"`
	Notes            *string            `json:"notes" binding:"omitempty,max=2000"`
	Stops            []stopRequest      `json:"stops" binding:"required,min=1,dive"`
	Passengers       []passengerRequest `json:"passengers" binding:"omitempty,dive"`
}

type updateRequest struct {
	Type             *string             `json:"type" binding:"omitempty,max=50"`
	ScheduledStartAt *string             `json:"scheduled_start_at"`
	DriverID         *string             `json:"driver_id" binding:"omitempty,max=64"`
	VehicleID        *string             `json:"vehicle_id" binding:"omitempty,max=64"`
	EventID          *string             `json:"event_id" binding:"omitempty,max=64"`
	Notes            *string             `json:"notes" binding:"omitempty,max=2000"`
	Passengers       *[]passengerRequest `json:"passengers" binding:"omitempty,dive"`
}

type stopResponse struct {
	ID          string     `json:"id"`
	Sequence    int        `json:"sequence"`
	Type        string     `json:"type"`
	Address     *string    `json:"address,omitempty"`
	ScheduledAt *time.Time `json:"scheduled_at,omitempty"`
	Status      string     `json:"status"`
}

type passengerResponse struct {
	ID            string  `json:"id"`
	StaffID       string  `json:"staff_id"`
	Name          string  `json:"name"`
	PickupStopID  *string `json:"pickup_stop_id,omitempty"`
	DropoffStopID *string `json:"dropoff_stop_id,omitempty"`
	Status        string  `json:"status"`
}

type response struct {
	ID                  string              `json:"id"`
	Type                string              `json:"type"`
	ScheduledStartAt    time.Time           `json:"scheduled_start_at"`
	DriverID            *string             `json:"driver_id,omitempty"`
	DriverName          *string             `json:"driver_name,omitempty"`
	VehicleID           *string             `json:"vehicle_id,omitempty"`
	VehicleRegistration *string             `json:"vehicle_registration,omitempty"`
	EventID             *string             `json:"event_id,omitempty"`
	Status              string              `json:"status"`
	Notes               *string             `json:"notes,omitempty"`
	StartedAt           *time.Time          `json:"started_at,omitempty"`
	CompletedAt         *time.Time          `json:"completed_at,omitempty"`
	CancelledAt         *time.Time          `json:"cancelled_at,omitempty"`
	Stops               []stopResponse      `json:"stops,omitempty"`
	Passengers          []passengerResponse `json:"passengers,omitempty"`
	CreatedAt           time.Time           `json:"created_at"`
	UpdatedAt           time.Time           `json:"updated_at"`
}

func toResponse(t *Trip) response {
	stops := make([]stopResponse, 0, len(t.Stops))
	for _, s := range t.Stops {
		stops = append(stops, stopResponse{
			ID:          s.ID,
			Sequence:    s.Sequence,
			Type:        s.Type,
			Address:     s.Address,
			ScheduledAt: s.ScheduledAt,
			Status:      s.Status,
		})
	}
	passengers := make([]passengerResponse, 0, len(t.Passengers))
	for _, p := range t.Passengers {
		passengers = append(passengers, passengerResponse{
			ID:            p.ID,
			StaffID:       p.StaffID,
			Name:          p.Name,
			PickupStopID:  p.PickupStopID,
			DropoffStopID: p.DropoffStopID,
			Status:        p.Status,
		})
	}
	return response{
		ID:                  t.ID,
		Type:                t.Type,
		ScheduledStartAt:    t.ScheduledStartAt,
		DriverID:            t.DriverID,
		DriverName:          t.DriverName,
		VehicleID:           t.VehicleID,
		VehicleRegistration: t.VehicleReg,
		EventID:             t.EventID,
		Status:              t.Status,
		Notes:               t.Notes,
		StartedAt:           t.StartedAt,
		CompletedAt:         t.CompletedAt,
		CancelledAt:         t.CancelledAt,
		Stops:               stops,
		Passengers:          passengers,
		CreatedAt:           t.CreatedAt,
		UpdatedAt:           t.UpdatedAt,
	}
}

func (h *Handler) list(c *gin.Context) {
	page, pageSize, offset := httpx.PageParams(c)

	items, total, err := h.svc.List(c.Request.Context(), ListFilter{
		Status:    c.Query("status"),
		DriverID:  c.Query("driver_id"),
		VehicleID: c.Query("vehicle_id"),
		Offset:    offset,
		Limit:     pageSize,
	}, c.Query("date"))
	if err != nil {
		h.writeError(c, err)
		return
	}

	data := make([]response, 0, len(items))
	for i := range items {
		data = append(data, toResponse(&items[i]))
	}
	c.JSON(http.StatusOK, httpx.ListResponse[response]{
		Data:       data,
		Pagination: httpx.Pagination{Page: page, PageSize: pageSize, Total: total},
	})
}

func (h *Handler) create(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "VALIDATION_ERROR", "Provide valid trip details.")
		return
	}

	start, err := parseTime(req.ScheduledStartAt)
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_ERROR", "scheduled_start_at must be an RFC3339 timestamp.")
		return
	}
	stops, err := parseStops(req.Stops)
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_ERROR", "A stop has an invalid scheduled_at timestamp.")
		return
	}

	actorID := actorFrom(c)
	created, err := h.svc.Create(c.Request.Context(), CreateInput{
		Type:             req.Type,
		ScheduledStartAt: start,
		DriverID:         req.DriverID,
		VehicleID:        req.VehicleID,
		EventID:          req.EventID,
		Notes:            req.Notes,
		Stops:            stops,
		Passengers:       passengersFrom(req.Passengers),
	}, actorID)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toResponse(created))
}

func (h *Handler) get(c *gin.Context) {
	trip, err := h.svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, toResponse(trip))
}

func (h *Handler) update(c *gin.Context) {
	var req updateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "VALIDATION_ERROR", "Provide valid trip fields.")
		return
	}

	in := UpdateInput{
		Type:      req.Type,
		DriverID:  req.DriverID,
		VehicleID: req.VehicleID,
		EventID:   req.EventID,
		Notes:     req.Notes,
	}
	if req.ScheduledStartAt != nil {
		start, err := parseTime(*req.ScheduledStartAt)
		if err != nil {
			httpx.BadRequest(c, "VALIDATION_ERROR", "scheduled_start_at must be an RFC3339 timestamp.")
			return
		}
		in.ScheduledStartAt = &start
	}
	if req.Passengers != nil {
		passengers := passengersFrom(*req.Passengers)
		in.Passengers = &passengers
	}

	updated, err := h.svc.Update(c.Request.Context(), c.Param("id"), in, actorFrom(c))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, toResponse(updated))
}

func (h *Handler) cancel(c *gin.Context) {
	if err := h.svc.Cancel(c.Request.Context(), c.Param("id"), actorFrom(c)); err != nil {
		h.writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.NotFound(c, "TRIP_NOT_FOUND", "Trip not found.")
	case errors.Is(err, ErrNotEditable):
		httpx.Conflict(c, "TRIP_NOT_EDITABLE", "A completed or cancelled trip cannot be modified.")
	case errors.Is(err, ErrNotCancellable):
		httpx.Conflict(c, "TRIP_NOT_CANCELLABLE", "A completed trip cannot be cancelled.")
	case errors.Is(err, ErrForbidden):
		httpx.Forbidden(c, "FORBIDDEN", "You are not assigned to this trip.")
	case errors.Is(err, ErrInvalidTransition):
		httpx.Conflict(c, "TRIP_INVALID_STATE", "This action is not allowed in the trip's current state.")
	case errors.Is(err, ErrStopNotFound):
		httpx.NotFound(c, "STOP_NOT_FOUND", "Stop not found on this trip.")
	case errors.Is(err, ErrPassengerNotFound):
		httpx.NotFound(c, "PASSENGER_NOT_FOUND", "Passenger not found on this trip.")
	case errors.Is(err, ErrCapacityExceeded):
		httpx.Conflict(c, "TRIP_CAPACITY_EXCEEDED", "Passenger count exceeds vehicle capacity.")
	case errors.Is(err, ErrDriverNotFound):
		httpx.BadRequest(c, "DRIVER_NOT_FOUND", "The selected driver does not exist.")
	case errors.Is(err, ErrDriverInactive):
		httpx.Conflict(c, "DRIVER_INACTIVE", "The selected driver is not active.")
	case errors.Is(err, ErrVehicleNotFound):
		httpx.BadRequest(c, "VEHICLE_NOT_FOUND", "The selected vehicle does not exist.")
	case errors.Is(err, ErrVehicleUnavailable):
		httpx.Conflict(c, "VEHICLE_UNAVAILABLE", "The selected vehicle is not available.")
	case errors.Is(err, ErrStaffNotFound):
		httpx.BadRequest(c, "STAFF_NOT_FOUND", "A selected staff member does not exist.")
	case errors.Is(err, ErrStaffInactive):
		httpx.Conflict(c, "STAFF_INACTIVE", "A selected staff member is not active.")
	case errors.Is(err, ErrEventNotFound):
		httpx.BadRequest(c, "EVENT_NOT_FOUND", "The selected event does not exist.")
	case errors.Is(err, ErrInvalidPassengerStop):
		httpx.BadRequest(c, "VALIDATION_ERROR", "A passenger references an invalid stop.")
	case errors.Is(err, ErrDuplicatePassenger):
		httpx.BadRequest(c, "VALIDATION_ERROR", "A staff member appears more than once.")
	case errors.Is(err, ErrStopsRequired):
		httpx.BadRequest(c, "VALIDATION_ERROR", "At least one stop is required.")
	case errors.Is(err, ErrInvalidType), errors.Is(err, ErrInvalidStatus),
		errors.Is(err, ErrInvalidStopType), errors.Is(err, ErrInvalidDate),
		errors.Is(err, ErrInvalidScheduledTime):
		httpx.BadRequest(c, "VALIDATION_ERROR", "One or more trip fields are invalid.")
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

func parseTime(raw string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, err
	}
	return t.UTC(), nil
}

func parseOptionalTime(raw *string) (*time.Time, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	t, err := parseTime(*raw)
	if err != nil {
		return nil, err
	}
	return &t, nil
}

func parseStops(stops []stopRequest) ([]StopInput, error) {
	out := make([]StopInput, 0, len(stops))
	for _, s := range stops {
		scheduledAt, err := parseOptionalTime(s.ScheduledAt)
		if err != nil {
			return nil, err
		}
		out = append(out, StopInput{Type: s.Type, Address: s.Address, ScheduledAt: scheduledAt})
	}
	return out, nil
}

func passengersFrom(passengers []passengerRequest) []PassengerInput {
	out := make([]PassengerInput, 0, len(passengers))
	for _, p := range passengers {
		out = append(out, PassengerInput{
			StaffID:          p.StaffID,
			PickupStopIndex:  p.PickupStopIndex,
			DropoffStopIndex: p.DropoffStopIndex,
		})
	}
	return out
}
