package trips

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"staff-transport/internal/auth"
	"staff-transport/internal/httpx"
	"staff-transport/internal/users"
)

// registerDriverExecution registers current-user trip listing and driver
// execution transitions. Every execution route authorizes that the caller is
// the trip's assigned driver.
func (h *Handler) registerDriverExecution(rg *gin.RouterGroup) {
	me := rg.Group("/me")
	me.Use(auth.RequireAuth(h.auth))
	me.GET("/trips", h.myTrips)

	exec := rg.Group("/trips")
	exec.Use(auth.RequireAuth(h.auth), auth.RequireRole(string(users.RoleDriver)))
	exec.POST("/:id/start", h.start)
	exec.POST("/:id/complete", h.complete)
	exec.POST("/:id/stops/:stopId/arrive", h.arriveStop)
	exec.POST("/:id/stops/:stopId/depart", h.departStop)
	exec.POST("/:id/passengers/:passengerId/pickup", h.pickupPassenger)
	exec.POST("/:id/passengers/:passengerId/no-show", h.noShowPassenger)
}

func (h *Handler) myTrips(c *gin.Context) {
	claims, ok := auth.ClaimsFrom(c)
	if !ok {
		httpx.Unauthorized(c, "UNAUTHENTICATED", "Authentication is required.")
		return
	}

	page, pageSize, offset := httpx.PageParams(c)
	items, total, err := h.svc.MyTrips(c.Request.Context(), claims.UserID, claims.Role, ListFilter{
		Status: c.Query("status"),
		Offset: offset,
		Limit:  pageSize,
	}, c.Query("date"))
	if err != nil {
		h.writeError(c, err)
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

func (h *Handler) start(c *gin.Context) {
	trip, err := h.svc.StartTrip(c.Request.Context(), c.Param("id"), actorFrom(c))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, toResponse(trip))
}

func (h *Handler) complete(c *gin.Context) {
	trip, err := h.svc.CompleteTrip(c.Request.Context(), c.Param("id"), actorFrom(c))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, toResponse(trip))
}

func (h *Handler) arriveStop(c *gin.Context) {
	trip, err := h.svc.ArriveStop(c.Request.Context(), c.Param("id"), c.Param("stopId"), actorFrom(c))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, toResponse(trip))
}

func (h *Handler) departStop(c *gin.Context) {
	trip, err := h.svc.DepartStop(c.Request.Context(), c.Param("id"), c.Param("stopId"), actorFrom(c))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, toResponse(trip))
}

func (h *Handler) pickupPassenger(c *gin.Context) {
	trip, err := h.svc.PickupPassenger(c.Request.Context(), c.Param("id"), c.Param("passengerId"), actorFrom(c))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, toResponse(trip))
}

func (h *Handler) noShowPassenger(c *gin.Context) {
	trip, err := h.svc.NoShowPassenger(c.Request.Context(), c.Param("id"), c.Param("passengerId"), actorFrom(c))
	if err != nil {
		h.writeError(c, err)
		return
	}
	c.JSON(http.StatusOK, toResponse(trip))
}
