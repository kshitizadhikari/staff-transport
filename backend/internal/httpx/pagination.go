package httpx

import (
	"strconv"

	"github.com/gin-gonic/gin"
)

const (
	defaultPageSize = 25
	maxPageSize     = 100
)

// Pagination is the pagination metadata returned with collection responses.
type Pagination struct {
	Page     int   `json:"page"`
	PageSize int   `json:"page_size"`
	Total    int64 `json:"total"`
}

// ListResponse is the standard collection envelope (docs/API_CONVENTIONS.md).
type ListResponse[T any] struct {
	Data       []T        `json:"data"`
	Pagination Pagination `json:"pagination"`
}

// PageParams reads page and page_size query parameters, applying defaults and
// a maximum page size, and returns the SQL offset for the requested page.
func PageParams(c *gin.Context) (page, pageSize, offset int) {
	page = positiveInt(c.Query("page"), 1)
	pageSize = positiveInt(c.Query("page_size"), defaultPageSize)
	if pageSize > maxPageSize {
		pageSize = maxPageSize
	}
	offset = (page - 1) * pageSize
	return
}

func positiveInt(raw string, fallback int) int {
	n, err := strconv.Atoi(raw)
	if err != nil || n < 1 {
		return fallback
	}
	return n
}
