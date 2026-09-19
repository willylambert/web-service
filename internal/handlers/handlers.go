package handlers

import (
	"encoding/json"
	"net/http"
	"time"
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

// NewMux builds the HTTP router for the service.
func NewMux() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /healthz", Health)
	mux.HandleFunc("GET /api/v1/hello", Hello)
	return mux
}

// Health reports service liveness.
func Health(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, HealthResponse{
		Status:    "ok",
		Timestamp: time.Now().UTC(),
	})
}

// Hello returns a greeting; optional name via ?name=.
func Hello(w http.ResponseWriter, r *http.Request) {
	name := r.URL.Query().Get("name")
	if name == "" {
		name = "world"
	}
	writeJSON(w, http.StatusOK, HelloResponse{
		Message: "Hello, " + name + "!",
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}
