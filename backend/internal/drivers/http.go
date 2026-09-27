package drivers

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"staff-transport/internal/auth"
	"staff-transport/internal/httpx"
	"staff-transport/internal/users"
)

const dateLayout = "2006-01-02"

// Handler serves manager-facing driver endpoints.
type Handler struct {
	svc  *Service
	auth auth.Service
}

// NewHandler returns a driver HTTP handler.
func NewHandler(svc *Service, authSvc auth.Service) *Handler {
	return &Handler{svc: svc, auth: authSvc}
}

// RegisterRoutes registers manager-only driver routes on rg.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/drivers")
	g.Use(auth.RequireAuth(h.auth), auth.RequireRole(string(users.RoleManager)))

	g.GET("", h.list)
	g.POST("", h.create)
	g.GET("/:id", h.get)
	g.PATCH("/:id", h.update)
	g.DELETE("/:id", h.deactivate)
}

type createRequest struct {
	Name          string  `json:"name" binding:"required,max=200"`
	Email         *string `json:"email" binding:"omitempty,max=200"`
	Phone         *string `json:"phone" binding:"omitempty,max=50"`
	LicenseNumber *string `json:"license_number" binding:"omitempty,max=100"`
	LicenseExpiry *string `json:"license_expiry" binding:"omitempty,datetime=2006-01-02"`
	Status        string  `json:"status" binding:"omitempty,max=20"`
	Password      string  `json:"password" binding:"omitempty,min=8,max=200"`
}

type updateRequest struct {
	Name          *string `json:"name" binding:"omitempty,max=200"`
	Email         *string `json:"email" binding:"omitempty,max=200"`
	Phone         *string `json:"phone" binding:"omitempty,max=50"`
	LicenseNumber *string `json:"license_number" binding:"omitempty,max=100"`
	LicenseExpiry *string `json:"license_expiry" binding:"omitempty,datetime=2006-01-02"`
	Status        *string `json:"status" binding:"omitempty,max=20"`
}

type response struct {
	ID            string    `json:"id"`
	UserID        string    `json:"user_id"`
	Name          string    `json:"name"`
	Email         *string   `json:"email,omitempty"`
	Phone         *string   `json:"phone,omitempty"`
	LicenseNumber *string   `json:"license_number,omitempty"`
	LicenseExpiry *string   `json:"license_expiry,omitempty"`
	Status        string    `json:"status"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
}

func toResponse(d *Driver) response {
	var expiry *string
	if d.LicenseExpiry != nil {
		formatted := d.LicenseExpiry.Format(dateLayout)
		expiry = &formatted
	}
	return response{
		ID:            d.ID,
		UserID:        d.UserID,
		Name:          d.Name,
		Email:         d.Email,
		Phone:         d.Phone,
		LicenseNumber: d.LicenseNumber,
		LicenseExpiry: expiry,
		Status:        d.Status,
		CreatedAt:     d.CreatedAt,
		UpdatedAt:     d.UpdatedAt,
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
		httpx.BadRequest(c, "VALIDATION_ERROR", "Provide valid driver details.")
		return
	}

	expiry, err := parseDate(req.LicenseExpiry)
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_ERROR", "license_expiry must be a valid date (YYYY-MM-DD).")
		return
	}

	created, err := h.svc.Create(c.Request.Context(), CreateInput{
		Name:          req.Name,
		Email:         req.Email,
		Phone:         req.Phone,
		LicenseNumber: req.LicenseNumber,
		LicenseExpiry: expiry,
		Status:        req.Status,
	}, req.Password)
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
		httpx.BadRequest(c, "VALIDATION_ERROR", "Provide valid driver fields.")
		return
	}

	expiry, err := parseDate(req.LicenseExpiry)
	if err != nil {
		httpx.BadRequest(c, "VALIDATION_ERROR", "license_expiry must be a valid date (YYYY-MM-DD).")
		return
	}

	updated, err := h.svc.Update(c.Request.Context(), c.Param("id"), UpdateInput{
		Name:          req.Name,
		Email:         req.Email,
		Phone:         req.Phone,
		LicenseNumber: req.LicenseNumber,
		LicenseExpiry: expiry,
		Status:        req.Status,
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
		httpx.NotFound(c, "DRIVER_NOT_FOUND", "Driver not found.")
	case errors.Is(err, ErrNameRequired):
		httpx.BadRequest(c, "VALIDATION_ERROR", "Name is required.")
	case errors.Is(err, ErrInvalidEmail):
		httpx.BadRequest(c, "VALIDATION_ERROR", "Enter a valid email address.")
	case errors.Is(err, ErrInvalidStatus):
		httpx.BadRequest(c, "VALIDATION_ERROR", "Invalid driver status.")
	case errors.Is(err, ErrEmailTaken):
		httpx.Conflict(c, "EMAIL_IN_USE", "That email is already in use.")
	default:
		httpx.Internal(c, err)
	}
}

func parseDate(raw *string) (*time.Time, error) {
	if raw == nil || *raw == "" {
		return nil, nil
	}
	t, err := time.Parse(dateLayout, *raw)
	if err != nil {
		return nil, err
	}
	return &t, nil
}
