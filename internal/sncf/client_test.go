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
	client.now = func() time.Time {
		return time.Date(2026, 9, 20, 7, 0, 0, 0, parisLocation)
	}

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

func TestDeparturesKeptUntilAngers(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "87487892") && strings.HasSuffix(r.URL.Path, "/departures") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"departures": []map[string]any{
					{
						"display_informations": map[string]any{"trip_short_name": "860105"},
						"stop_date_time":       map[string]any{"departure_date_time": "20260920T074800"},
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
						"direction": "Angers Saint-Laud (Angers)", "trip_short_name": "860105", "commercial_mode": "Rémi",
					},
					"stop_date_time": map[string]any{
						"departure_date_time": "20260920T074400", "base_departure_date_time": "20260920T074400",
					},
				},
				{
					"display_informations": map[string]any{
						"direction": "Angers Saint-Laud (Angers)", "trip_short_name": "859099", "commercial_mode": "Aléop",
					},
					"stop_date_time": map[string]any{
						"departure_date_time": "20260920T070000", "base_departure_date_time": "20260920T070000",
					},
					"links": []map[string]any{
						{"type": "vehicle_journey", "id": "vj:arrived"},
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.baseURL = server.URL
	client.httpClient = server.Client()
	// After La Ménitré departure (07:44), before Angers arrival (08:10).
	client.now = func() time.Time {
		return time.Date(2026, 9, 20, 7, 55, 0, 0, parisLocation)
	}

	board, err := client.Departures(context.Background(), 5)
	if err != nil {
		t.Fatalf("Departures: %v", err)
	}
	if len(board.Trains) != 1 {
		t.Fatalf("trains = %d, want 1 still en route to Angers", len(board.Trains))
	}
	if board.Trains[0].TripName != "860105" {
		t.Fatalf("unexpected train: %+v", board.Trains[0])
	}
	if board.Trains[0].Status != "en route" {
		t.Fatalf("status = %q, want en route", board.Trains[0].Status)
	}
}

func TestDeparturesShowsCancelledTrains(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "traffic_reports") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"disruptions": []map[string]any{
					{
						"severity": map[string]any{"effect": "NO_SERVICE", "name": "trip canceled"},
						"messages": []map[string]any{{"text": "Défaillance de matériel"}},
						"application_periods": []map[string]any{
							{"begin": "20260920T080000", "end": "20260920T120000"},
						},
						"impacted_objects": []map[string]any{
							{
								"pt_object": map[string]any{
									"embedded_type": "trip",
									"trip":          map[string]any{"name": "860200"},
								},
							},
						},
					},
				},
			})
			return
		}
		if strings.Contains(r.URL.Path, "87487892") {
			_ = json.NewEncoder(w).Encode(map[string]any{"departures": []any{}, "arrivals": []any{}})
			return
		}
		if strings.Contains(r.URL.Path, "87484006") {
			_ = json.NewEncoder(w).Encode(map[string]any{
				"departures": []any{},
				"arrivals": []map[string]any{
					{
						"display_informations": map[string]any{"trip_short_name": "860200"},
						"stop_date_time":       map[string]any{"arrival_date_time": "20260920T091000"},
					},
				},
			})
			return
		}

		// Realtime board: cancelled train already removed.
		if r.URL.Query().Get("data_freshness") == "realtime" || r.URL.Query().Get("data_freshness") == "" {
			_ = json.NewEncoder(w).Encode(map[string]any{"departures": []any{}})
			return
		}
		// Base schedule still lists it.
		_ = json.NewEncoder(w).Encode(map[string]any{
			"departures": []map[string]any{
				{
					"display_informations": map[string]any{
						"direction": "Angers Saint-Laud (Angers)", "trip_short_name": "860200", "commercial_mode": "Rémi",
					},
					"stop_date_time": map[string]any{
						"departure_date_time": "20260920T084000", "base_departure_date_time": "20260920T084000",
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.baseURL = server.URL
	client.httpClient = server.Client()
	client.now = func() time.Time {
		return time.Date(2026, 9, 20, 8, 15, 0, 0, parisLocation)
	}

	board, err := client.Departures(context.Background(), 5)
	if err != nil {
		t.Fatalf("Departures: %v", err)
	}
	if len(board.Trains) != 1 {
		t.Fatalf("trains = %d, want 1 cancelled train", len(board.Trains))
	}
	train := board.Trains[0]
	if train.TripName != "860200" || train.Status != "supprimé" {
		t.Fatalf("unexpected train: %+v", train)
	}
	if train.StatusDetail != "Défaillance de matériel" {
		t.Fatalf("status_detail = %q", train.StatusDetail)
	}
	if train.AngersTime != "09:10" {
		t.Fatalf("expected Angers 09:10, got %+v", train)
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
	client.now = func() time.Time {
		return time.Date(2026, 9, 20, 19, 30, 0, 0, parisLocation)
	}

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

func TestJourneyStops(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.Contains(r.URL.Path, "vehicle_journeys") {
			t.Fatalf("unexpected path %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"vehicle_journeys": []map[string]any{
				{
					"stop_times": []map[string]any{
						{
							"arrival_time":   "084000",
							"departure_time": "084100",
							"stop_point":     map[string]any{"name": "La Ménitré"},
						},
						{
							"arrival_time":   "090500",
							"departure_time": "090500",
							"stop_point":     map[string]any{"name": "Angers Saint-Laud"},
						},
					},
				},
			},
		})
	}))
	defer server.Close()

	client := NewClient("test-token")
	client.baseURL = server.URL
	client.httpClient = server.Client()
	client.now = func() time.Time {
		return time.Date(2026, 9, 20, 8, 0, 0, 0, parisLocation)
	}

	detail, err := client.JourneyStops(context.Background(), "vehicle_journey:SNCF:2026-10-10:test:1187:Train")
	if err != nil {
		t.Fatalf("JourneyStops: %v", err)
	}
	if len(detail.Stops) != 2 {
		t.Fatalf("stops = %d, want 2", len(detail.Stops))
	}
	if detail.Stops[0].Name != "La Ménitré" || detail.Stops[0].DepartureTime != "08:41" {
		t.Fatalf("unexpected first stop: %+v", detail.Stops[0])
	}
	if detail.Stops[0].DepartureDateTime != "2026-10-10T08:41:00" {
		t.Fatalf("expected dated departure, got %+v", detail.Stops[0])
	}
	if detail.Stops[1].ArrivalTime != "09:05" {
		t.Fatalf("unexpected second stop: %+v", detail.Stops[1])
	}
	if detail.ServiceDate != "2026-10-10" {
		t.Fatalf("service_date = %q", detail.ServiceDate)
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
