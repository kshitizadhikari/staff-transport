// Package httpx contains small HTTP response helpers shared by API modules.
//
// The error envelope is defined by docs/API_CONVENTIONS.md:
//
//	{ "error": { "code": "...", "message": "...", "details": {} } }
//
// Handlers must never include stack traces or database errors in responses.
package httpx

import (
	"log/slog"
	"net/http"

	"github.com/gin-gonic/gin"
)

// ErrorDetail is the inner error object returned to clients.
type ErrorDetail struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Details any    `json:"details,omitempty"`
}

// ErrorBody is the top-level error response envelope.
type ErrorBody struct {
	Error ErrorDetail `json:"error"`
}

// Error writes a structured error response with the given status, code, and
// user-facing message. Details are optional and must not contain internal data.
func Error(c *gin.Context, status int, code, message string, details ...any) {
	body := ErrorBody{Error: ErrorDetail{Code: code, Message: message}}
	if len(details) > 0 {
		body.Error.Details = details[0]
	}
	c.AbortWithStatusJSON(status, body)
}

// BadRequest writes a 400 response for malformed or invalid input.
func BadRequest(c *gin.Context, code, message string) {
	Error(c, http.StatusBadRequest, code, message)
}

// Unauthorized writes a 401 response.
func Unauthorized(c *gin.Context, code, message string) {
	Error(c, http.StatusUnauthorized, code, message)
}

// Forbidden writes a 403 response.
func Forbidden(c *gin.Context, code, message string) {
	Error(c, http.StatusForbidden, code, message)
}

// NotFound writes a 404 response.
func NotFound(c *gin.Context, code, message string) {
	Error(c, http.StatusNotFound, code, message)
}

// Conflict writes a 409 response for business-state conflicts.
func Conflict(c *gin.Context, code, message string) {
	Error(c, http.StatusConflict, code, message)
}

// Internal logs the underlying error and writes a generic 500 response so
// internal details are not exposed to clients.
func Internal(c *gin.Context, err error) {
	slog.Error("internal_server_error",
		"error", err,
		"method", c.Request.Method,
		"path", c.Request.URL.Path,
	)
	Error(c, http.StatusInternalServerError, "INTERNAL_ERROR", "An unexpected error occurred.")
}
