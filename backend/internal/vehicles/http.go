package vehicles

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"staff-transport/internal/auth"
	"staff-transport/internal/httpx"
	"staff-transport/internal/users"
)

// Handler serves manager-facing vehicle endpoints.
type Handler struct {
	svc  *Service
	auth auth.Service
}

// NewHandler returns a vehicle HTTP handler.
func NewHandler(svc *Service, authSvc auth.Service) *Handler {
	return &Handler{svc: svc, auth: authSvc}
}

// RegisterRoutes registers manager-only vehicle routes on rg.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/vehicles")
	g.Use(auth.RequireAuth(h.auth), auth.RequireRole(string(users.RoleManager)))

	g.GET("", h.list)
	g.POST("", h.create)
	g.GET("/:id", h.get)
	g.PATCH("/:id", h.update)
	g.DELETE("/:id", h.deactivate)
}

type createRequest struct {
	RegistrationNumber string  `json:"registration_number" binding:"required,max=50"`
	Model              *string `json:"model" binding:"omitempty,max=200"`
	Capacity           int     `json:"capacity" binding:"gte=0"`
	Status             string  `json:"status" binding:"omitempty,max=20"`
}

type updateRequest struct {
	RegistrationNumber *string `json:"registration_number" binding:"omitempty,max=50"`
	Model              *string `json:"model" binding:"omitempty,max=200"`
	Capacity           *int    `json:"capacity" binding:"omitempty,gte=0"`
	Status             *string `json:"status" binding:"omitempty,max=20"`
}

type response struct {
	ID                 string    `json:"id"`
	RegistrationNumber string    `json:"registration_number"`
	Model              *string   `json:"model,omitempty"`
	Capacity           int       `json:"capacity"`
	Status             string    `json:"status"`
	CreatedAt          time.Time `json:"created_at"`
	UpdatedAt          time.Time `json:"updated_at"`
}

func toResponse(v *Vehicle) response {
	return response{
		ID:                 v.ID,
		RegistrationNumber: v.RegistrationNumber,
		Model:              v.Model,
		Capacity:           v.Capacity,
		Status:             v.Status,
		CreatedAt:          v.CreatedAt,
		UpdatedAt:          v.UpdatedAt,
	}
}

func (h *Handler) list(c *gin.Context) {
	page, pageSize, offset := httpx.PageParams(c)

	items, total, err := h.svc.List(c.Request.Context(), ListFilter{
		Search: c.Query("search"),
		Status: c.Query("status"),
		Offset: offset,
		Limit:  pageSize,
	})
	if err != nil {
		h.writeServiceError(c, err)
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
		httpx.BadRequest(c, "VALIDATION_ERROR", "Provide valid vehicle details.")
		return
	}

	created, err := h.svc.Create(c.Request.Context(), CreateInput{
		RegistrationNumber: req.RegistrationNumber,
		Model:              req.Model,
		Capacity:           req.Capacity,
		Status:             req.Status,
	})
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusCreated, toResponse(created))
}

func (h *Handler) get(c *gin.Context) {
	item, err := h.svc.Get(c.Request.Context(), c.Param("id"))
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, toResponse(item))
}

func (h *Handler) update(c *gin.Context) {
	var req updateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "VALIDATION_ERROR", "Provide valid vehicle fields.")
		return
	}

	updated, err := h.svc.Update(c.Request.Context(), c.Param("id"), UpdateInput{
		RegistrationNumber: req.RegistrationNumber,
		Model:              req.Model,
		Capacity:           req.Capacity,
		Status:             req.Status,
	})
	if err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.JSON(http.StatusOK, toResponse(updated))
}

func (h *Handler) deactivate(c *gin.Context) {
	if err := h.svc.Deactivate(c.Request.Context(), c.Param("id")); err != nil {
		h.writeServiceError(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) writeServiceError(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		httpx.NotFound(c, "VEHICLE_NOT_FOUND", "Vehicle not found.")
	case errors.Is(err, ErrRegistrationRequired):
		httpx.BadRequest(c, "VALIDATION_ERROR", "Registration number is required.")
	case errors.Is(err, ErrInvalidStatus):
		httpx.BadRequest(c, "VALIDATION_ERROR", "Invalid vehicle status.")
	case errors.Is(err, ErrCapacityInvalid):
		httpx.BadRequest(c, "VALIDATION_ERROR", "Capacity must not be negative.")
	case errors.Is(err, ErrRegistrationTaken):
		httpx.Conflict(c, "REGISTRATION_IN_USE", "That registration number is already in use.")
	default:
		httpx.Internal(c, err)
	}
}
