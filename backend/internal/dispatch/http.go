package dispatch

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"staff-transport/internal/auth"
	"staff-transport/internal/httpx"
	"staff-transport/internal/users"
)

// Handler serves manager-facing recurring-schedule endpoints.
type Handler struct {
	svc  *Service
	auth auth.Service
}

// NewHandler returns a dispatch HTTP handler.
func NewHandler(svc *Service, authSvc auth.Service) *Handler {
	return &Handler{svc: svc, auth: authSvc}
}

// RegisterRoutes registers manager-only recurring-schedule routes on rg.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/recurring-schedules")
	g.Use(auth.RequireAuth(h.auth), auth.RequireRole(string(users.RoleManager)))

	g.GET("", h.list)
	g.POST("", h.create)
	g.GET("/:id", h.get)
	g.PATCH("/:id", h.update)
	g.DELETE("/:id", h.deactivate)
	g.POST("/:id/generate", h.generate)
}

type recurrenceRequest struct {
	Frequency string `json:"frequency" binding:"required"`
	Weekdays  []int  `json:"weekdays" binding:"required,min=1,max=7,dive,gte=0,lte=6"`
}

type templateStopRequest struct {
	Type    string  `json:"type" binding:"required"`
	Address *string `json:"address"`
}

type templatePassengerRequest struct {
	StaffID          string `json:"staff_id" binding:"required"`
	PickupStopIndex  *int   `json:"pickup_stop_index"`
	DropoffStopIndex *int   `json:"dropoff_stop_index"`
}

type templateRequest struct {
	ScheduledTime string                     `json:"scheduled_time" binding:"required"`
	DriverID      *string                    `json:"driver_id"`
	VehicleID     *string                    `json:"vehicle_id"`
	Notes         *string                    `json:"notes"`
	Stops         []templateStopRequest      `json:"stops" binding:"required,min=1,dive"`
	Passengers    []templatePassengerRequest `json:"passengers" binding:"omitempty,dive"`
}

type createRequest struct {
	Name       string            `json:"name" binding:"required,max=200"`
	TripType   string            `json:"trip_type" binding:"required"`
	Timezone   string            `json:"timezone"`
	StartDate  string            `json:"start_date" binding:"required"`
	EndDate    *string           `json:"end_date"`
	Recurrence recurrenceRequest `json:"recurrence" binding:"required"`
	Template   templateRequest   `json:"template" binding:"required"`
}

type updateRequest struct {
	Name       *string            `json:"name" binding:"omitempty,max=200"`
	Active     *bool              `json:"active"`
	EndDate    *string            `json:"end_date"`
	Recurrence *recurrenceRequest `json:"recurrence"`
	Template   *templateRequest   `json:"template"`
}

type generateRequest struct {
	From string `json:"from" binding:"required"`
	To   string `json:"to" binding:"required"`
}

type templateResponse struct {
	ScheduledTime string              `json:"scheduled_time"`
	DriverID      *string             `json:"driver_id,omitempty"`
	VehicleID     *string             `json:"vehicle_id,omitempty"`
	Notes         *string             `json:"notes,omitempty"`
	Stops         []TemplateStop      `json:"stops"`
	Passengers    []TemplatePassenger `json:"passengers"`
}

type scheduleResponse struct {
	ID         string           `json:"id"`
	Name       string           `json:"name"`
	TripType   string           `json:"trip_type"`
	Timezone   string           `json:"timezone"`
	Active     bool             `json:"active"`
	StartDate  string           `json:"start_date"`
	EndDate    *string          `json:"end_date,omitempty"`
	Recurrence Recurrence       `json:"recurrence"`
	Template   templateResponse `json:"template"`
	CreatedAt  time.Time        `json:"created_at"`
	UpdatedAt  time.Time        `json:"updated_at"`
}

func toResponse(schedule *Schedule) scheduleResponse {
	var endDate *string
	if schedule.EndDate != nil {
		value := schedule.EndDate.Format(dateLayout)
		endDate = &value
	}
	return scheduleResponse{
		ID:         schedule.ID,
		Name:       schedule.Name,
		TripType:   schedule.TripType,
		Timezone:   schedule.Timezone,
		Active:     schedule.Active,
		StartDate:  schedule.StartDate.Format(dateLayout),
		EndDate:    endDate,
		Recurrence: schedule.Recurrence,
		Template: templateResponse{
			ScheduledTime: schedule.Template.ScheduledTime,
			DriverID:      schedule.Template.DriverID,
			VehicleID:     schedule.Template.VehicleID,
			Notes:         schedule.Template.Notes,
			Stops:         schedule.Template.Stops,
			Passengers:    schedule.Template.Passengers,
		},
		CreatedAt: schedule.CreatedAt,
		UpdatedAt: schedule.UpdatedAt,
	}
}

func (h *Handler) list(c *gin.Context) {
	page, pageSize, offset := httpx.PageParams(c)
	items, total, err := h.svc.List(c.Request.Context(), offset, pageSize)
	if err != nil {
		h.writeError(c, err)
		return
	}
	data := make([]scheduleResponse, 0, len(items))
	for i := range items {
		data = append(data, toResponse(&items[i]))
	}
	c.JSON(http.StatusOK, httpx.ListResponse[scheduleResponse]{
		Data:       data,
		Pagination: httpx.Pagination{Page: page, PageSize: pageSize, Total: total},
	})
}

func (h *Handler) create(c *gin.Context) {
	var req createRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "VALIDATION_ERROR", "Provide valid schedule details.")
		return
	}

	in, err := createInputFromRequest(req)
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_ERROR", "One or more schedule fields are invalid.")
		return
	}

	created, err := h.svc.Create(c.Request.Context(), in, actorFrom(c))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toResponse(created))
}

func (h *Handler) get(c *gin.Context) {
	schedule, err := h.svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, toResponse(schedule))
}

func (h *Handler) update(c *gin.Context) {
	var req updateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "VALIDATION_ERROR", "Provide valid schedule fields.")
		return
	}

	in := UpdateInput{Name: req.Name, Active: req.Active}
	if req.EndDate != nil {
		parsed, err := time.Parse(dateLayout, *req.EndDate)
		if err != nil {
			httpx.BadRequest(c, "VALIDATION_ERROR", "end_date must be a valid date (YYYY-MM-DD).")
			return
		}
		in.EndDate = &parsed
	}
	if req.Recurrence != nil {
		recurrence := Recurrence(*req.Recurrence)
		in.Recurrence = &recurrence
	}
	if req.Template != nil {
		template := templateFromRequest(*req.Template)
		in.Template = &template
	}

	updated, err := h.svc.Update(c.Request.Context(), c.Param("id"), in, actorFrom(c))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, toResponse(updated))
}

func (h *Handler) deactivate(c *gin.Context) {
	if err := h.svc.Deactivate(c.Request.Context(), c.Param("id"), actorFrom(c)); err != nil {
		h.writeError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) generate(c *gin.Context) {
	var req generateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "VALIDATION_ERROR", "from and to dates are required.")
		return
	}
	from, err := time.Parse(dateLayout, req.From)
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_ERROR", "from must be a valid date (YYYY-MM-DD).")
		return
	}
	to, err := time.Parse(dateLayout, req.To)
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_ERROR", "to must be a valid date (YYYY-MM-DD).")
		return
	}

	generated, err := h.svc.Generate(c.Request.Context(), c.Param("id"), actorFrom(c), from, to)
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, gin.H{"generated": generated})
}

func (h *Handler) writeError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.NotFound(c, "SCHEDULE_NOT_FOUND", "Recurring schedule not found.")
	case errors.Is(err, ErrInactive):
		httpx.Conflict(c, "SCHEDULE_INACTIVE", "This schedule is inactive.")
	case errors.Is(err, ErrNameRequired),
		errors.Is(err, ErrInvalidTripType),
		errors.Is(err, ErrInvalidTimezone),
		errors.Is(err, ErrInvalidRecurrence),
		errors.Is(err, ErrInvalidTemplate),
		errors.Is(err, ErrInvalidDate),
		errors.Is(err, ErrInvalidRange):
		httpx.BadRequest(c, "VALIDATION_ERROR", "One or more schedule fields are invalid.")
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

func createInputFromRequest(req createRequest) (CreateInput, error) {
	startDate, err := time.Parse(dateLayout, req.StartDate)
	if err != nil {
		return CreateInput{}, ErrInvalidDate
	}
	in := CreateInput{
		Name:       req.Name,
		TripType:   req.TripType,
		Timezone:   req.Timezone,
		StartDate:  startDate,
		Recurrence: Recurrence(req.Recurrence),
		Template:   templateFromRequest(req.Template),
	}
	if req.EndDate != nil {
		endDate, err := time.Parse(dateLayout, *req.EndDate)
		if err != nil {
			return CreateInput{}, ErrInvalidDate
		}
		in.EndDate = &endDate
	}
	return in, nil
}

func templateFromRequest(req templateRequest) Template {
	stops := make([]TemplateStop, 0, len(req.Stops))
	for _, stop := range req.Stops {
		stops = append(stops, TemplateStop{Type: stop.Type, Address: stop.Address})
	}
	passengers := make([]TemplatePassenger, 0, len(req.Passengers))
	for _, passenger := range req.Passengers {
		passengers = append(passengers, TemplatePassenger{
			StaffID:          passenger.StaffID,
			PickupStopIndex:  passenger.PickupStopIndex,
			DropoffStopIndex: passenger.DropoffStopIndex,
		})
	}
	return Template{
		ScheduledTime: req.ScheduledTime,
		DriverID:      req.DriverID,
		VehicleID:     req.VehicleID,
		Notes:         req.Notes,
		Stops:         stops,
		Passengers:    passengers,
	}
}
