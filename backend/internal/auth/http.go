package auth

import (
	"context"
	"errors"
	"net/http"

	"github.com/gin-gonic/gin"

	"staff-transport/internal/httpx"
	"staff-transport/internal/users"
)

// UserService is the subset of the users module needed for authentication.
type UserService interface {
	Authenticate(ctx context.Context, email, password string) (*users.User, error)
	FindByID(ctx context.Context, id string) (*users.User, error)
}

// Handler serves authentication and current-user endpoints.
type Handler struct {
	tokens Service
	users  UserService
}

// NewHandler returns an authentication handler.
func NewHandler(tokens Service, userSvc UserService) *Handler {
	return &Handler{tokens: tokens, users: userSvc}
}

type loginRequest struct {
	Email    string `json:"email" binding:"required,email"`
	Password string `json:"password" binding:"required"`
}

type refreshRequest struct {
	RefreshToken string `json:"refresh_token" binding:"required"`
}

type logoutRequest struct {
	RefreshToken string `json:"refresh_token"`
}

type meResponse struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Email *string `json:"email,omitempty"`
	Phone *string `json:"phone,omitempty"`
	Role  string  `json:"role"`
}

// RegisterRoutes registers /auth/* and the authenticated /me endpoint on rg.
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	g := rg.Group("/auth")
	g.POST("/login", h.login)
	g.POST("/refresh", h.refresh)
	g.POST("/logout", h.logout)

	me := rg.Group("")
	me.Use(RequireAuth(h.tokens))
	me.GET("/me", h.me)
}

func (h *Handler) login(c *gin.Context) {
	var req loginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "VALIDATION_ERROR", "A valid email and password are required.")
		return
	}

	user, err := h.users.Authenticate(c.Request.Context(), req.Email, req.Password)
	if err != nil {
		switch {
		case errors.Is(err, users.ErrInvalidCredentials):
			httpx.Unauthorized(c, "INVALID_CREDENTIALS", "Invalid email or password.")
		case errors.Is(err, users.ErrInactive):
			httpx.Forbidden(c, "ACCOUNT_INACTIVE", "This account is inactive.")
		default:
			httpx.Internal(c, err)
		}
		return
	}

	pair, err := h.tokens.Issue(c.Request.Context(), user.ID, string(user.Role))
	if err != nil {
		httpx.Internal(c, err)
		return
	}
	c.JSON(http.StatusOK, pair)
}

func (h *Handler) refresh(c *gin.Context) {
	var req refreshRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		httpx.BadRequest(c, "VALIDATION_ERROR", "refresh_token is required.")
		return
	}

	userID, err := h.tokens.ConsumeRefresh(c.Request.Context(), req.RefreshToken)
	if err != nil {
		if errors.Is(err, ErrInvalidToken) {
			httpx.Unauthorized(c, "INVALID_REFRESH_TOKEN", "The refresh token is invalid or expired.")
			return
		}
		httpx.Internal(c, err)
		return
	}

	user, err := h.users.FindByID(c.Request.Context(), userID)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			httpx.Unauthorized(c, "INVALID_REFRESH_TOKEN", "The refresh token is invalid or expired.")
			return
		}
		httpx.Internal(c, err)
		return
	}
	if user.Status != users.StatusActive {
		httpx.Forbidden(c, "ACCOUNT_INACTIVE", "This account is inactive.")
		return
	}

	pair, err := h.tokens.Issue(c.Request.Context(), user.ID, string(user.Role))
	if err != nil {
		httpx.Internal(c, err)
		return
	}
	c.JSON(http.StatusOK, pair)
}

func (h *Handler) logout(c *gin.Context) {
	var req logoutRequest
	_ = c.ShouldBindJSON(&req)

	if err := h.tokens.Revoke(c.Request.Context(), req.RefreshToken); err != nil {
		httpx.Internal(c, err)
		return
	}
	c.Status(http.StatusNoContent)
}

func (h *Handler) me(c *gin.Context) {
	claims, ok := ClaimsFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication is required.")
		return
	}

	user, err := h.users.FindByID(c.Request.Context(), claims.UserID)
	if err != nil {
		if errors.Is(err, users.ErrNotFound) {
			httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication is required.")
			return
		}
		httpx.Internal(c, err)
		return
	}

	c.JSON(http.StatusOK, meResponse{
		ID:    user.ID,
		Name:  user.Name,
		Email: user.Email,
		Phone: user.Phone,
		Role:  string(user.Role),
	})
}
