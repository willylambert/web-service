package handlers_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/willylambert/web-service/internal/handlers"
	"github.com/willylambert/web-service/internal/sncf"
)

func TestMain(m *testing.M) {
	gin.SetMode(gin.TestMode)
	m.Run()
}

func TestHealth(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	handlers.NewRouter(nil).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var body handlers.HealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Status != "ok" {
		t.Fatalf("status = %q, want %q", body.Status, "ok")
	}
}

func TestHelloDefault(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/hello", nil)
	rr := httptest.NewRecorder()

	handlers.NewRouter(nil).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var body handlers.HelloResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Message != "Hello, world!" {
		t.Fatalf("message = %q, want %q", body.Message, "Hello, world!")
	}
}

func TestHelloWithName(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/hello?name=Ada", nil)
	rr := httptest.NewRecorder()

	handlers.NewRouter(nil).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}

	var body handlers.HelloResponse
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Message != "Hello, Ada!" {
		t.Fatalf("message = %q, want %q", body.Message, "Hello, Ada!")
	}
}

func TestIndexPage(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rr := httptest.NewRecorder()

	handlers.NewRouter(nil).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusOK)
	}
	if ct := rr.Header().Get("Content-Type"); !strings.Contains(ct, "text/html") {
		t.Fatalf("content-type = %q, want text/html", ct)
	}
}

func TestTrainsWithoutToken(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/v1/trains", nil)
	rr := httptest.NewRecorder()

	handlers.NewRouter(nil).ServeHTTP(rr, req)

	if rr.Code != http.StatusServiceUnavailable {
		t.Fatalf("status = %d, want %d", rr.Code, http.StatusServiceUnavailable)
	}
}

type stubBoard struct {
	board *sncf.Board
}

func (s stubBoard) Configured() bool { return true }
func (s stubBoard) Departures(ctx context.Context, count int) (*sncf.Board, error) {
	return s.board, nil
}
func (s stubBoard) Arrivals(ctx context.Context, count int) (*sncf.Board, error) {
	return s.board, nil
}

func TestTrainsDepartures(t *testing.T) {
	stub := stubBoard{board: &sncf.Board{
		Station:   "La Ménitré",
		Direction: "departures",
		UpdatedAt: time.Date(2026, 9, 20, 8, 0, 0, 0, time.UTC),
		Trains: []sncf.Train{{
			Time:           "08:42",
			Direction:      "Angers Saint-Laud",
			CommercialMode: "TER",
			Status:         "à l'heure",
		}},
	}}

	req := httptest.NewRequest(http.MethodGet, "/api/v1/trains?direction=departures", nil)
	rr := httptest.NewRecorder()
	handlers.NewRouter(stub).ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d body=%s", rr.Code, http.StatusOK, rr.Body.String())
	}

	var body sncf.Board
	if err := json.NewDecoder(rr.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Trains) != 1 || body.Trains[0].Direction != "Angers Saint-Laud" {
		t.Fatalf("unexpected board: %+v", body)
	}
}
