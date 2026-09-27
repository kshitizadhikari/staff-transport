package staff

import (
	"errors"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"staff-transport/internal/auth"
	"staff-transport/internal/httpx"
	"staff-transport/internal/users"
)

// Handler serves manager-facing staff endpoints.
type Handler struct {
	svc  *Service
	auth auth.Service
}

// NewHandler returns a staff HTTP handler.
func NewHandler(svc *Service, authSvc auth.Service) *Handler {
	return &Handler{svc: svc, auth: authSvc}
}

// RegisterRoutes registers manager-only staff routes on rg.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/staff")
	g.Use(auth.RequireAuth(h.auth), auth.RequireRole(string(users.RoleManager)))

	g.GET("", h.list)
	g.POST("", h.create)
	g.GET("/:id", h.get)
	g.PATCH("/:id", h.update)
	g.DELETE("/:id", h.deactivate)
}

type createRequest struct {
	Name        string  `json:"name" binding:"required,max=200"`
	Email       *string `json:"email" binding:"omitempty,max=200"`
	Phone       *string `json:"phone" binding:"omitempty,max=50"`
	Department  *string `json:"department" binding:"omitempty,max=200"`
	HomeAddress *string `json:"home_address" binding:"omitempty,max=500"`
	Password    string  `json:"password" binding:"omitempty,min=8,max=200"`
	Active      *bool   `json:"active"`
}

type updateRequest struct {
	Name        *string `json:"name" binding:"omitempty,max=200"`
	Email       *string `json:"email" binding:"omitempty,max=200"`
	Phone       *string `json:"phone" binding:"omitempty,max=50"`
	Department  *string `json:"department" binding:"omitempty,max=200"`
	HomeAddress *string `json:"home_address" binding:"omitempty,max=500"`
	Active      *bool   `json:"active"`
}

type response struct {
	ID          string    `json:"id"`
	UserID      string    `json:"user_id"`
	Name        string    `json:"name"`
	Email       *string   `json:"email,omitempty"`
	Phone       *string   `json:"phone,omitempty"`
	Department  *string   `json:"department,omitempty"`
	HomeAddress *string   `json:"home_address,omitempty"`
	Active      bool      `json:"active"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

func toResponse(s *Staff) response {
	return response{
		ID:          s.ID,
		UserID:      s.UserID,
		Name:        s.Name,
		Email:       s.Email,
		Phone:       s.Phone,
		Department:  s.Department,
		HomeAddress: s.HomeAddress,
		Active:      s.Active,
		CreatedAt:   s.CreatedAt,
		UpdatedAt:   s.UpdatedAt,
	}
}

func (h *Handler) list(c *gin.Context) {
	page, pageSize, offset := httpx.PageParams(c)

	filter := ListFilter{
		Search:     c.Query("search"),
		Department: c.Query("department"),
		Offset:     offset,
		Limit:      pageSize,
	}
	if raw := c.Query("active"); raw != "" {
		active, err := strconv.ParseBool(raw)
		if err != nil {
			httpx.BadRequest(c, "VALIDATION_ERROR", "active must be true or false.")
			return
		}
		filter.Active = &active
	}

	items, total, err := h.svc.List(c.Request.Context(), filter)
	if err != nil {
		httpx.Internal(c, err)
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
		httpx.BadRequest(c, "VALIDATION_ERROR", "Provide a valid name and account details.")
		return
	}

	active := true
	if req.Active != nil {
		active = *req.Active
	}

	created, err := h.svc.Create(c.Request.Context(), CreateInput{
		Name:        req.Name,
		Email:       req.Email,
		Phone:       req.Phone,
		Department:  req.Department,
		HomeAddress: req.HomeAddress,
		Active:      active,
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
		httpx.BadRequest(c, "VALIDATION_ERROR", "Provide valid staff fields.")
		return
	}

	updated, err := h.svc.Update(c.Request.Context(), c.Param("id"), UpdateInput{
		Name:        req.Name,
		Email:       req.Email,
		Phone:       req.Phone,
		Department:  req.Department,
		HomeAddress: req.HomeAddress,
		Active:      req.Active,
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
		httpx.NotFound(c, "STAFF_NOT_FOUND", "Staff member not found.")
	case errors.Is(err, ErrNameRequired):
		httpx.BadRequest(c, "VALIDATION_ERROR", "Name is required.")
	case errors.Is(err, ErrInvalidEmail):
		httpx.BadRequest(c, "VALIDATION_ERROR", "Enter a valid email address.")
	case errors.Is(err, ErrEmailTaken):
		httpx.Conflict(c, "EMAIL_IN_USE", "That email is already in use.")
	default:
		httpx.Internal(c, err)
	}
}
