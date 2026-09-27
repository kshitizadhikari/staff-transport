package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"staff-transport/internal/auth"
	"staff-transport/internal/config"
	"staff-transport/internal/dispatch"
	"staff-transport/internal/drivers"
	"staff-transport/internal/events"
	"staff-transport/internal/locations"
	"staff-transport/internal/notifications"
	"staff-transport/internal/staff"
	"staff-transport/internal/trips"
	"staff-transport/internal/users"
	"staff-transport/internal/vehicles"
)

// Dependencies are the collaborators the HTTP server needs.
type Dependencies struct {
	Config        *config.Config
	DB            *gorm.DB
	Redis         *goredis.Client
	Auth          auth.Service
	Users         *users.Service
	Staff         *staff.Service
	Drivers       *drivers.Service
	Vehicles      *vehicles.Service
	Trips         *trips.Service
	Locations     *locations.Service
	Notifications *notifications.Service
	Dispatch      *dispatch.Service
	Events        *events.Service
}

type Server struct {
	cfg           *config.Config
	db            *gorm.DB
	rdb           *goredis.Client
	auth          auth.Service
	users         *users.Service
	staff         *staff.Service
	drivers       *drivers.Service
	vehicles      *vehicles.Service
	trips         *trips.Service
	locations     *locations.Service
	notifications *notifications.Service
	dispatch      *dispatch.Service
	events        *events.Service
	gin           *gin.Engine
}

func New(deps Dependencies) *Server {
	if deps.Config.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	s := &Server{
		cfg:           deps.Config,
		db:            deps.DB,
		rdb:           deps.Redis,
		auth:          deps.Auth,
		users:         deps.Users,
		staff:         deps.Staff,
		drivers:       deps.Drivers,
		vehicles:      deps.Vehicles,
		trips:         deps.Trips,
		locations:     deps.Locations,
		notifications: deps.Notifications,
		dispatch:      deps.Dispatch,
		events:        deps.Events,
		gin:           gin.New(),
	}
	s.gin.Use(gin.Recovery(), corsMiddleware(deps.Config.CORSAllowedOrigins), requestLogger())
	s.registerRoutes()
	return s
}

func (s *Server) Handler() http.Handler {
	return s.gin
}

func (s *Server) registerRoutes() {
	s.gin.GET("/health", s.handleHealth)
	s.gin.GET("/ready", s.handleReady)

	v1 := s.gin.Group("/api/v1")
	v1.GET("/health", s.handleHealth)

	auth.NewHandler(s.auth, s.users, auth.CookieConfig{
		Secure:   s.cfg.IsProduction(),
		MaxAge:   int(s.cfg.RefreshTokenTTL.Seconds()),
		SameSite: http.SameSiteLaxMode,
	}).RegisterRoutes(v1)
	staff.NewHandler(s.staff, s.auth).RegisterRoutes(v1)
	drivers.NewHandler(s.drivers, s.auth).RegisterRoutes(v1)
	vehicles.NewHandler(s.vehicles, s.auth).RegisterRoutes(v1)
	trips.NewHandler(s.trips, s.auth).RegisterRoutes(v1)
	locations.NewHandler(s.locations, s.auth).RegisterRoutes(v1)
	notifications.NewHandler(s.notifications, s.auth).RegisterRoutes(v1)
	dispatch.NewHandler(s.dispatch, s.auth).RegisterRoutes(v1)
	events.NewHandler(s.events, s.auth).RegisterRoutes(v1)
}
