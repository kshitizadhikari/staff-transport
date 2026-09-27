package notifications

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"

	"staff-transport/internal/auth"
	"staff-transport/internal/httpx"
)

// Handler serves current-user notification endpoints.
type Handler struct {
	svc  *Service
	auth auth.Service
}

// NewHandler returns a notification HTTP handler.
func NewHandler(svc *Service, authSvc auth.Service) *Handler {
	return &Handler{svc: svc, auth: authSvc}
}

// RegisterRoutes registers push-token and notification routes on rg.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	me := rg.Group("/me")
	me.Use(auth.RequireAuth(h.auth))
	me.POST("/push-tokens", h.registerToken)
	me.GET("/notifications", h.list)
}

type registerTokenRequest struct {
	Token    string `json:"token" binding:"required,max=255"`
	Platform string `json:"platform" binding:"omitempty,oneof=ios android web unknown"`
}

type notificationResponse struct {
	ID        string         `json:"id"`
	Type      string         `json:"type"`
	Status    string         `json:"status"`
	Payload   map[string]any `json:"payload"`
	SentAt    *time.Time     `json:"sent_at,omitempty"`
	CreatedAt time.Time      `json:"created_at"`
}

func (h *Handler) registerToken(c *gin.Context) {
	claims, ok := auth.ClaimsFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication is required.")
		return
	}

	var req registerTokenRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "VALIDATION_ERROR", "A push token is required.")
		return
	}

	if err := h.svc.RegisterToken(c.Request.Context(), claims.UserID, req.Token, req.Platform); err != nil {
		httpx.Internal(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) list(c *gin.Context) {
	claims, ok := auth.ClaimsFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication is required.")
		return
	}

	page, pageSize, offset := httpx.PageParams(c)
	items, total, err := h.svc.List(c.Request.Context(), claims.UserID, offset, pageSize)
	if err != nil {
		httpx.Internal(c, err)
		return
	}

	data := make([]notificationResponse, 0, len(items))
	for _, item := range items {
		data = append(data, notificationResponse{
			ID:        item.ID,
			Type:      item.Type,
			Status:    item.Status,
			Payload:   item.Payload,
			SentAt:    item.SentAt,
			CreatedAt: item.CreatedAt,
		})
	}

	c.JSON(http.StatusOK, httpx.ListResponse[notificationResponse]{
		Data:       data,
		Pagination: httpx.Pagination{Page: page, PageSize: pageSize, Total: total},
	})
}
