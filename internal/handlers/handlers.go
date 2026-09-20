package handlers

import (
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
)

// HealthResponse is returned by the health check endpoint.
type HealthResponse struct {
	Status    string    `json:"status"`
	Timestamp time.Time `json:"timestamp"`
}

// HelloResponse is returned by the hello endpoint.
type HelloResponse struct {
	Message string `json:"message"`
}

// NewRouter builds the Gin engine for the service.
func NewRouter() *gin.Engine {
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())

	r.GET("/healthz", Health)
	r.GET("/api/v1/hello", Hello)
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
