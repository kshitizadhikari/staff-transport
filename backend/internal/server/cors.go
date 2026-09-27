package server

import (
	"time"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

// corsMiddleware allows the configured web origins to call the API.
//
// Credentials are allowed so refresh tokens can move to httpOnly cookies later;
// origins must therefore be explicit (the library rejects wildcard origins when
// credentials are enabled).
func corsMiddleware(allowedOrigins []string) gin.HandlerFunc {
	return cors.New(cors.Config{
		AllowOrigins: allowedOrigins,
		AllowMethods: []string{"GET", "POST", "PATCH", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders: []string{
			"Authorization",
			"Content-Type",
			"Accept",
			"Idempotency-Key",
			"X-Request-ID",
		},
		ExposeHeaders:    []string{"X-Request-ID"},
		AllowCredentials: true,
		MaxAge:           12 * time.Hour,
	})
}
