package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	"staff-transport/internal/db"
	"staff-transport/internal/redis"
)

func (s *Server) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

func (s *Server) handleReady(c *gin.Context) {
	ctx, cancel := context.WithTimeout(c.Request.Context(), 2*time.Second)
	defer cancel()

	checks := gin.H{"database": "ok", "redis": "ok"}
	ready := true

	if err := db.Ping(ctx, s.db); err != nil {
		checks["database"] = "unavailable"
		ready = false
		slog.Error("readiness: database check failed", "error", err)
	}
	if err := redis.Ping(ctx, s.rdb); err != nil {
		checks["redis"] = "unavailable"
		ready = false
		slog.Error("readiness: redis check failed", "error", err)
	}

	status := http.StatusOK
	if !ready {
		status = http.StatusServiceUnavailable
	}
	c.JSON(status, gin.H{"status": map[bool]string{true: "ready", false: "not_ready"}[ready], "checks": checks})
}

func requestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		requestID := c.GetHeader("X-Request-ID")
		if requestID == "" {
			requestID = uuid.NewString()
		}
		c.Header("X-Request-ID", requestID)

		c.Next()

		slog.Info("http_request",
			"request_id", requestID,
			"method", c.Request.Method,
			"path", c.Request.URL.Path,
			"status", c.Writer.Status(),
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}
}
