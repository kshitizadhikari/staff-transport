package auth

import (
	"strings"

	"github.com/gin-gonic/gin"

	"staff-transport/internal/httpx"
)

const claimsContextKey = "auth.claims"

// RequireAuth rejects requests without a valid Bearer access token and stores
// the parsed claims on the request context for downstream handlers.
func RequireAuth(svc Service) gin.HandlerFunc {
	return func(c *gin.Context) {
		token, ok := bearerToken(c)
		if !ok {
			httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication is required.")
			return
		}
		claims, err := svc.ParseAccessToken(token)
		if err != nil {
			httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication is required.")
			return
		}
		c.Set(claimsContextKey, claims)
		c.Next()
	}
}

// ClaimsFrom returns the access-token claims set by RequireAuth.
func ClaimsFrom(c *gin.Context) (*Claims, bool) {
	value, ok := c.Get(claimsContextKey)
	if !ok {
		return nil, false
	}
	claims, ok := value.(*Claims)
	return claims, ok
}

// RequireRole rejects requests whose authenticated user does not have the given
// role. It must be used after RequireAuth.
func RequireRole(role string) gin.HandlerFunc {
	return func(c *gin.Context) {
		claims, ok := ClaimsFrom(c)
		if !ok || claims.Role != role {
			httpx.Forbidden(c, "FORBIDDEN", "You do not have permission to perform this action.")
			return
		}
		c.Next()
	}
}

func bearerToken(c *gin.Context) (string, bool) {
	header := c.GetHeader("Authorization")
	const prefix = "Bearer "
	if len(header) <= len(prefix) || !strings.EqualFold(header[:len(prefix)], prefix) {
		return "", false
	}
	token := strings.TrimSpace(header[len(prefix):])
	return token, token != ""
}
