package server

import (
	"net/http"

	"github.com/gin-gonic/gin"
	goredis "github.com/redis/go-redis/v9"
	"gorm.io/gorm"

	"staff-transport/internal/auth"
	"staff-transport/internal/config"
	"staff-transport/internal/users"
)

type Server struct {
	cfg   *config.Config
	db    *gorm.DB
	rdb   *goredis.Client
	auth  auth.Service
	users *users.Service
	gin   *gin.Engine
}

func New(cfg *config.Config, db *gorm.DB, rdb *goredis.Client, authSvc auth.Service, usersSvc *users.Service) *Server {
	if cfg.IsProduction() {
		gin.SetMode(gin.ReleaseMode)
	}

	s := &Server{
		cfg:   cfg,
		db:    db,
		rdb:   rdb,
		auth:  authSvc,
		users: usersSvc,
		gin:   gin.New(),
	}
	s.gin.Use(gin.Recovery(), corsMiddleware(cfg.CORSAllowedOrigins), requestLogger())
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

	auth.NewHandler(s.auth, s.users).RegisterRoutes(v1)

	// Module routes are registered here as each module is implemented:
	//   staff.RegisterRoutes(v1, ...)
	//   trips.RegisterRoutes(v1, ...)
}
