package handlers

import (
	"context"
	"embed"
	"io/fs"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/willylambert/web-service/internal/sncf"
)

//go:embed web
var webFS embed.FS

// BoardProvider fetches train boards.
type BoardProvider interface {
	Configured() bool
	Departures(ctx context.Context, count int) (*sncf.Board, error)
	Arrivals(ctx context.Context, count int) (*sncf.Board, error)
}

// HealthResponse is returned by the health check endpoint.
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// HelloResponse is returned by the hello endpoint.
type HelloResponse struct {
	Message string `json:"message"`
}

// ErrorResponse is a JSON error payload.
type ErrorResponse struct {
	Error string `json:"error"`
}

// NewRouter builds the Gin engine for the service.
func NewRouter(trains BoardProvider) *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	r.GET("/healthz", Health)
	r.GET("/api/v1/hello", Hello)
	r.GET("/api/v1/trains", Trains(trains))

	webRoot, err := fs.Sub(webFS, "web")
	if err == nil {
		r.GET("/", func(c *gin.Context) {
			data, readErr := fs.ReadFile(webRoot, "index.html")
			if readErr != nil {
				c.Status(http.StatusNotFound)
				return
			}
			c.Data(http.StatusOK, "text/html; charset=utf-8", data)
		})
		r.GET("/manifest.webmanifest", func(c *gin.Context) {
			data, readErr := fs.ReadFile(webRoot, "manifest.webmanifest")
			if readErr != nil {
				c.Status(http.StatusNotFound)
				return
			}
			c.Header("Cache-Control", "no-cache")
			c.Data(http.StatusOK, "application/manifest+json", data)
		})
		r.GET("/sw.js", func(c *gin.Context) {
			data, readErr := fs.ReadFile(webRoot, "sw.js")
			if readErr != nil {
				c.Status(http.StatusNotFound)
				return
			}
			c.Header("Cache-Control", "no-cache")
			c.Header("Service-Worker-Allowed", "/")
			c.Data(http.StatusOK, "application/javascript; charset=utf-8", data)
		})
		icons, iconsErr := fs.Sub(webRoot, "icons")
		if iconsErr == nil {
			r.StaticFS("/icons", http.FS(icons))
		}
		assets, assetsErr := fs.Sub(webRoot, "assets")
		if assetsErr == nil {
			r.StaticFS("/assets", http.FS(assets))
		}
	}

	return r
}

// Health reports service liveness.
func Health(c *gin.Context) {
	c.JSON(http.StatusOK, HealthResponse{
		Status:    "ok",
		Timestamp: time.Now().UTC(),
	})
}

// Hello returns a greeting; optional name via ?name=.
func Hello(c *gin.Context) {
	name := c.DefaultQuery("name", "world")
	c.JSON(http.StatusOK, HelloResponse{
		Message: "Hello, " + name + "!",
	})
}

// Trains returns the next departures or arrivals for La Ménitré.
func Trains(provider BoardProvider) gin.HandlerFunc {
	return func(c *gin.Context) {
		if provider == nil || !provider.Configured() {
			c.JSON(http.StatusServiceUnavailable, ErrorResponse{
				Error: "SNCF API token missing. Set SNCF_API_TOKEN (free key at https://numerique.sncf.com/startup/api/).",
			})
			return
		}

		count := 15
		if raw := c.Query("count"); raw != "" {
			if n, err := strconv.Atoi(raw); err == nil {
				count = n
			}
		}

		direction := c.DefaultQuery("direction", "departures")
		var (
			board *sncf.Board
			err   error
		)
		switch direction {
		case "arrivals":
			board, err = provider.Arrivals(c.Request.Context(), count)
		default:
			board, err = provider.Departures(c.Request.Context(), count)
		}
		if err != nil {
			c.JSON(http.StatusBadGateway, ErrorResponse{Error: err.Error()})
			return
		}
		c.JSON(http.StatusOK, board)
	}
}
