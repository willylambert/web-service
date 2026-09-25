package sncf

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFormatNavitiaTime(t *testing.T) {
	if got := formatNavitiaTime("20260920T084215"); got != "08:42" {
		t.Fatalf("got %q, want 08:42", got)
	}
}

func TestDelayMinutes(t *testing.T) {
	if got := delayMinutes("20260920T084200", "20260920T084700"); got != 5 {
		t.Fatalf("delay = %d, want 5", got)
	}
}

func TestDeparturesParsesBoard(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, _, ok := r.BasicAuth()
		if !ok || user != "test-token" {
			t.Fatalf("auth = %q ok=%v", user, ok)
		}

		if strings.Contains(r.URL.Path, "87487892") && strings.HasSuffix(r.URL.Path, "/departures") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"departures": []map[string]any{
					{
						"display_informations": map[string]any{"trip_short_name": "860105"},
						"stop_date_time":       map[string]any{"departure_date_time": "20260920T074800"},
					},
					{
						"display_informations": map[string]any{"trip_short_name": "859023"},
						"stop_date_time":       map[string]any{"departure_date_time": "20260920T091900"},
					},
				},
			})
			return
		}
		if strings.Contains(r.URL.Path, "87487892") && strings.HasSuffix(r.URL.Path, "/arrivals") {
			_ = json.NewEncoder(w).Encode(map[string]any{"arrivals": []any{}})
			return
		}
		if strings.Contains(r.URL.Path, "87484006") && strings.HasSuffix(r.URL.Path, "/arrivals") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"arrivals": []map[string]any{
					{
						"display_informations": map[string]any{"trip_short_name": "860105"},
						"stop_date_time":       map[string]any{"arrival_date_time": "20260920T081000"},
					},
					{
						"display_informations": map[string]any{"trip_short_name": "859023"},
						"stop_date_time":       map[string]any{"arrival_date_time": "20260920T094000"},
					},
				},
			})
			return
		}
		if strings.Contains(r.URL.Path, "87484006") && strings.HasSuffix(r.URL.Path, "/departures") {
			_ = json.NewEncoder(w).Encode(map[string]any{"departures": []any{}})
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"departures": []map[string]any{
				{
					"display_informations": map[string]any{
						"direction": "Saumur (Saumur)", "trip_short_name": "859022", "commercial_mode": "Aléop",
					},
					"stop_date_time": map[string]any{
						"departure_date_time": "20260920T091000", "base_departure_date_time": "20260920T091000",
					},
				},
				{
					"display_informations": map[string]any{
						"direction": "Nantes (Nantes)", "trip_short_name": "860105", "commercial_mode": "Rémi",
					},
					"stop_date_time": map[string]any{
						"departure_date_time": "20260920T074400", "base_departure_date_time": "20260920T074400",
					},
				},
				{
					"display_informations": map[string]any{
						"direction": "Angers Saint-Laud (Angers)", "trip_short_name": "859023", "commercial_mode": "Aléop",
					},
					"stop_date_time": map[string]any{
						"departure_date_time": "20260920T091500", "base_departure_date_time": "20260920T091000",
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.baseURL = server.URL
	client.httpClient = server.Client()

	board, err := client.Departures(context.Background(), 5)
	if err != nil {
		t.Fatalf("Departures: %v", err)
	}
	if len(board.Trains) != 2 {
		t.Fatalf("trains = %d, want 2", len(board.Trains))
	}
	if board.Trains[0].SaintMathurinTime != "07:48" || board.Trains[1].SaintMathurinTime != "09:19" {
		t.Fatalf("unexpected Saint-Mathurin times: %+v %+v", board.Trains[0], board.Trains[1])
	}
	if board.Trains[0].AngersTime != "08:10" || board.Trains[1].AngersTime != "09:40" {
		t.Fatalf("unexpected Angers times: %+v %+v", board.Trains[0], board.Trains[1])
	}
}

func TestArrivalsFromAngersOnly(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "87484006") && strings.HasSuffix(r.URL.Path, "/departures") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"departures": []map[string]any{
					{
						"display_informations": map[string]any{"trip_short_name": "859028"},
						"stop_date_time":       map[string]any{"departure_date_time": "20260920T194500"},
					},
				},
			})
			return
		}
		if strings.Contains(r.URL.Path, "87484006") && strings.HasSuffix(r.URL.Path, "/arrivals") {
			_ = json.NewEncoder(w).Encode(map[string]any{"arrivals": []any{}})
			return
		}
		if strings.Contains(r.URL.Path, "87487892") && strings.HasSuffix(r.URL.Path, "/departures") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"departures": []map[string]any{
					{
						"display_informations": map[string]any{"trip_short_name": "859028"},
						"stop_date_time":       map[string]any{"departure_date_time": "20260920T195500"},
					},
				},
			})
			return
		}
		if strings.Contains(r.URL.Path, "87487892") && strings.HasSuffix(r.URL.Path, "/arrivals") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"arrivals": []map[string]any{
					{
						"display_informations": map[string]any{"trip_short_name": "859028"},
						"stop_date_time":       map[string]any{"arrival_date_time": "20260920T195400"},
					},
				},
			})
			return
		}

		_ = json.NewEncoder(w).Encode(map[string]any{
			"arrivals": []map[string]any{
				{
					"display_informations": map[string]any{
						"direction": "Angers Saint-Laud (Angers)", "trip_short_name": "860133", "commercial_mode": "Rémi",
					},
					"stop_date_time": map[string]any{
						"arrival_date_time": "20260920T193400", "base_arrival_date_time": "20260920T193400",
					},
				},
				{
					"display_informations": map[string]any{
						"direction": "Saumur (Saumur)", "trip_short_name": "859028", "commercial_mode": "Aléop",
					},
					"stop_date_time": map[string]any{
						"arrival_date_time": "20260920T200500", "base_arrival_date_time": "20260920T200500",
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.baseURL = server.URL
	client.httpClient = server.Client()

	board, err := client.Arrivals(context.Background(), 10)
	if err != nil {
		t.Fatalf("Arrivals: %v", err)
	}
	if len(board.Trains) != 1 {
		t.Fatalf("trains = %d, want only Angers-origin train", len(board.Trains))
	}
	train := board.Trains[0]
	if train.TripName != "859028" || train.AngersTime != "19:45" {
		t.Fatalf("unexpected train: %+v", train)
	}
	if train.SaintMathurinTime != "19:54" {
		t.Fatalf("expected Saint-Mathurin 19:54, got %+v", train)
	}
}

func TestPickViaTime(t *testing.T) {
	ref := time.Date(2026, 9, 20, 20, 5, 0, 0, time.UTC)
	got, ok := pickViaTime(ref, "arrivals", []time.Time{
		time.Date(2026, 9, 20, 19, 45, 0, 0, time.UTC),
		time.Date(2026, 9, 20, 18, 10, 0, 0, time.UTC),
		time.Date(2026, 9, 20, 20, 20, 0, 0, time.UTC),
	})
	if !ok || got.Format("15:04") != "19:45" {
		t.Fatalf("got %v ok=%v, want 19:45", got, ok)
	}
}

func TestTowardAngers(t *testing.T) {
	if !towardAngers(Train{Direction: "Nantes (Nantes)"}) {
		t.Fatal("expected Nantes match")
	}
	if towardAngers(Train{Direction: "Saumur (Saumur)"}) {
		t.Fatal("Saumur should not match")
	}
}

func TestConfigured(t *testing.T) {
	if NewClient("").Configured() {
		t.Fatal("empty token should not be configured")
	}
}
