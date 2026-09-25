package sncf

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

const (
	defaultBaseURL = "https://api.sncf.com/v1"
	// LaMenitreStopArea is the Navitia stop_area id for Gare de La Ménitré (UIC 87487884).
	LaMenitreStopArea = "stop_area:SNCF:87487884"
	LaMenitreName     = "La Ménitré"
	// SaintMathurinStopArea is the Navitia stop_area id for Saint-Mathurin (UIC 87487892).
	SaintMathurinStopArea = "stop_area:SNCF:87487892"
	SaintMathurinName     = "Saint-Mathurin"
	// AngersStopArea is the Navitia stop_area id for Angers Saint-Laud (UIC 87484006).
	AngersStopArea = "stop_area:SNCF:87484006"
	AngersName     = "Angers Saint-Laud"
)

// Destination keywords for departures from La Ménitré toward Angers
// (Nantes-bound trains also stop at Angers on this line).
var departureTowardAngers = []string{"angers", "nantes"}

// Client talks to the SNCF (Navitia) Open Data API.
type Client struct {
	token      string
	baseURL    string
	stopAreaID string
	httpClient *http.Client
}

// NewClient builds an SNCF API client. Token is required for live requests.
func NewClient(token string) *Client {
	return &Client{
		token:      strings.TrimSpace(token),
		baseURL:    defaultBaseURL,
		stopAreaID: LaMenitreStopArea,
		httpClient: &http.Client{Timeout: 25 * time.Second},
	}
}

// Configured reports whether an API token is available.
func (c *Client) Configured() bool {
	return c != nil && c.token != ""
}

// Train is a simplified departure or arrival for the UI.
type Train struct {
	Time               string `json:"time"`
	BaseTime           string `json:"base_time,omitempty"`
	Direction          string `json:"direction"`
	Headsign           string `json:"headsign,omitempty"`
	CommercialMode     string `json:"commercial_mode"`
	Network            string `json:"network,omitempty"`
	TripName           string `json:"trip_name,omitempty"`
	Platform           string `json:"platform,omitempty"`
	Status             string `json:"status"`
	DelayMinutes       int    `json:"delay_minutes,omitempty"`
	SaintMathurinTime  string `json:"saint_mathurin_time,omitempty"`
	AngersTime         string `json:"angers_time,omitempty"`
	// Deprecated aliases kept for older UI builds.
	ViaStop     string `json:"via_stop,omitempty"`
	ViaStopTime string `json:"via_stop_time,omitempty"`
}

// Board is a station departure or arrival board.
type Board struct {
	Station   string    `json:"station"`
	StopArea  string    `json:"stop_area"`
	Direction string    `json:"direction"` // "departures" or "arrivals"
	UpdatedAt time.Time `json:"updated_at"`
	Trains    []Train   `json:"trains"`
}

type navitiaBoard struct {
	Departures []navitiaPassage `json:"departures"`
	Arrivals   []navitiaPassage `json:"arrivals"`
}

type navitiaPassage struct {
	DisplayInformations struct {
		Direction      string `json:"direction"`
		Headsign       string `json:"headsign"`
		Label          string `json:"label"`
		Network        string `json:"network"`
		CommercialMode string `json:"commercial_mode"`
		TripShortName  string `json:"trip_short_name"`
		Name           string `json:"name"`
	} `json:"display_informations"`
	StopDateTime struct {
		DepartureDateTime     string `json:"departure_date_time"`
		BaseDepartureDateTime string `json:"base_departure_date_time"`
		ArrivalDateTime       string `json:"arrival_date_time"`
		BaseArrivalDateTime   string `json:"base_arrival_date_time"`
	} `json:"stop_date_time"`
	StopPoint struct {
		Name string `json:"name"`
	} `json:"stop_point"`
	Links []struct {
		Type string `json:"type"`
		ID   string `json:"id"`
	} `json:"links"`
}

type navitiaVehicleJourneys struct {
	VehicleJourneys []struct {
		StopTimes []struct {
			ArrivalTime   string `json:"arrival_time"`
			DepartureTime string `json:"departure_time"`
			StopPoint     struct {
				Name string `json:"name"`
			} `json:"stop_point"`
		} `json:"stop_times"`
	} `json:"vehicle_journeys"`
}

// Departures fetches the next departures from La Ménitré toward Angers/Nantes.
func (c *Client) Departures(ctx context.Context, count int) (*Board, error) {
	return c.fetchBoard(ctx, "departures", count)
}

// Arrivals fetches arrivals at La Ménitré that previously stopped in Angers.
func (c *Client) Arrivals(ctx context.Context, count int) (*Board, error) {
	return c.fetchBoard(ctx, "arrivals", count)
}

func (c *Client) fetchBoard(ctx context.Context, kind string, count int) (*Board, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("SNCF API token not configured")
	}
	if count <= 0 {
		count = 15
	}
	if count > 40 {
		count = 40
	}

	// Over-fetch so destination filters still fill the board.
	fetchCount := 40

	path := fmt.Sprintf("/coverage/sncf/stop_areas/%s/%s", url.PathEscape(c.stopAreaID), kind)
	q := url.Values{}
	q.Set("count", fmt.Sprintf("%d", fetchCount))
	q.Set("data_freshness", "realtime")

	var raw navitiaBoard
	if err := c.getJSON(ctx, path, q, &raw); err != nil {
		return nil, err
	}

	passages := raw.Departures
	if kind == "arrivals" {
		passages = raw.Arrivals
	}

	var (
		saintMathurinIndex map[string][]time.Time
		angersIndex        map[string][]time.Time
		smArrivalIndex     map[string][]time.Time
	)
	saintMathurinIndex = c.stopTimesIndex(ctx, SaintMathurinStopArea)
	angersIndex = c.stopTimesIndex(ctx, AngersStopArea)
	if kind == "arrivals" {
		smArrivalIndex = c.stopArrivalsIndex(ctx, SaintMathurinStopArea)
	}

	trains := make([]Train, 0, len(passages))
	for _, p := range passages {
		train := mapPassage(p, kind)
		if kind == "departures" && !towardAngers(train) {
			continue
		}

		ref := passageRefTime(p, kind)
		if kind == "departures" {
			if when, ok := pickViaTime(ref, "departures", saintMathurinIndex[train.TripName]); ok {
				train.SaintMathurinTime = when.Format("15:04")
			}
			if when, ok := pickViaTime(ref, "departures", angersIndex[train.TripName]); ok {
				train.AngersTime = when.Format("15:04")
			}
			if train.SaintMathurinTime == "" || train.AngersTime == "" {
				c.enrichFromVehicleJourney(ctx, &train, p, ref)
			}
		} else {
			when, ok := pickViaTime(ref, "arrivals", angersIndex[train.TripName])
			if !ok {
				// Keep only trains that stopped in Angers before La Ménitré.
				continue
			}
			train.AngersTime = when.Format("15:04")
			if sm, ok := pickViaTime(ref, "arrivals", smArrivalIndex[train.TripName]); ok {
				angersAt := parseClockOnRef(ref, train.AngersTime)
				if angersAt.IsZero() || sm.After(angersAt) {
					train.SaintMathurinTime = sm.Format("15:04")
				}
			}
			if train.SaintMathurinTime == "" {
				if sm, ok := pickViaTime(ref, "arrivals", saintMathurinIndex[train.TripName]); ok {
					angersAt := parseClockOnRef(ref, train.AngersTime)
					if angersAt.IsZero() || sm.After(angersAt) {
						train.SaintMathurinTime = sm.Format("15:04")
					}
				}
			}
			if train.SaintMathurinTime == "" {
				c.enrichFromVehicleJourney(ctx, &train, p, ref)
			}
			train.ViaStop = AngersName
			train.ViaStopTime = train.AngersTime
		}

		trains = append(trains, train)
		if len(trains) >= count {
			break
		}
	}

	return &Board{
		Station:   LaMenitreName,
		StopArea:  c.stopAreaID,
		Direction: kind,
		UpdatedAt: time.Now().UTC(),
		Trains:    trains,
	}, nil
}

func passageRefTime(p navitiaPassage, kind string) time.Time {
	raw := p.StopDateTime.DepartureDateTime
	if kind == "arrivals" {
		raw = p.StopDateTime.ArrivalDateTime
	}
	return parseNavitiaDateTime(raw)
}

func (c *Client) enrichFromVehicleJourney(ctx context.Context, train *Train, p navitiaPassage, ref time.Time) {
	vjID := ""
	for _, link := range p.Links {
		if link.Type == "vehicle_journey" && link.ID != "" {
			vjID = link.ID
			break
		}
	}
	if vjID == "" || ref.IsZero() {
		return
	}

	path := "/coverage/sncf/vehicle_journeys/" + url.PathEscape(vjID)
	var raw navitiaVehicleJourneys
	if err := c.getJSON(ctx, path, nil, &raw); err != nil || len(raw.VehicleJourneys) == 0 {
		return
	}

	for _, stop := range raw.VehicleJourneys[0].StopTimes {
		name := strings.ToLower(stop.StopPoint.Name)
		switch {
		case train.SaintMathurinTime == "" && strings.Contains(name, "mathurin"):
			if t := clockOnDate(ref, firstNonEmpty(stop.ArrivalTime, stop.DepartureTime)); !t.IsZero() {
				train.SaintMathurinTime = t.Format("15:04")
			}
		case train.AngersTime == "" && strings.Contains(name, "angers"):
			if t := clockOnDate(ref, firstNonEmpty(stop.ArrivalTime, stop.DepartureTime)); !t.IsZero() {
				train.AngersTime = t.Format("15:04")
			}
		}
	}
}

func clockOnDate(ref time.Time, hhmmss string) time.Time {
	if len(hhmmss) < 4 {
		return time.Time{}
	}
	// Navitia may return "193800" or "19:38:00".
	digits := strings.Map(func(r rune) rune {
		if r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, hhmmss)
	if len(digits) < 4 {
		return time.Time{}
	}
	if len(digits) < 6 {
		digits += strings.Repeat("0", 6-len(digits))
	}
	parsed, err := time.Parse("150405", digits[:6])
	if err != nil {
		return time.Time{}
	}
	return time.Date(ref.Year(), ref.Month(), ref.Day(), parsed.Hour(), parsed.Minute(), parsed.Second(), 0, time.UTC)
}

func parseClockOnRef(ref time.Time, hhmm string) time.Time {
	if ref.IsZero() || hhmm == "" {
		return time.Time{}
	}
	parsed, err := time.Parse("15:04", hhmm)
	if err != nil {
		return time.Time{}
	}
	return time.Date(ref.Year(), ref.Month(), ref.Day(), parsed.Hour(), parsed.Minute(), 0, 0, time.UTC)
}

func towardAngers(train Train) bool {
	haystack := strings.ToLower(train.Direction + " " + train.Headsign)
	for _, keyword := range departureTowardAngers {
		if strings.Contains(haystack, keyword) {
			return true
		}
	}
	return false
}

func (c *Client) stopTimesIndex(ctx context.Context, stopAreaID string) map[string][]time.Time {
	out := make(map[string][]time.Time)
	q := url.Values{}
	q.Set("count", "40")
	q.Set("data_freshness", "realtime")

	depPath := fmt.Sprintf("/coverage/sncf/stop_areas/%s/departures", url.PathEscape(stopAreaID))
	var depRaw navitiaBoard
	if err := c.getJSON(ctx, depPath, q, &depRaw); err == nil {
		for _, p := range depRaw.Departures {
			trip := firstNonEmpty(p.DisplayInformations.TripShortName, p.DisplayInformations.Headsign)
			when := parseNavitiaDateTime(p.StopDateTime.DepartureDateTime)
			if trip == "" || when.IsZero() {
				continue
			}
			out[trip] = append(out[trip], when)
		}
	}

	arrPath := fmt.Sprintf("/coverage/sncf/stop_areas/%s/arrivals", url.PathEscape(stopAreaID))
	var arrRaw navitiaBoard
	if err := c.getJSON(ctx, arrPath, q, &arrRaw); err == nil {
		for _, p := range arrRaw.Arrivals {
			trip := firstNonEmpty(p.DisplayInformations.TripShortName, p.DisplayInformations.Headsign)
			when := parseNavitiaDateTime(firstNonEmpty(p.StopDateTime.ArrivalDateTime, p.StopDateTime.DepartureDateTime))
			if trip == "" || when.IsZero() {
				continue
			}
			out[trip] = append(out[trip], when)
		}
	}
	return out
}

func (c *Client) stopArrivalsIndex(ctx context.Context, stopAreaID string) map[string][]time.Time {
	out := make(map[string][]time.Time)
	q := url.Values{}
	q.Set("count", "40")
	q.Set("data_freshness", "realtime")

	arrPath := fmt.Sprintf("/coverage/sncf/stop_areas/%s/arrivals", url.PathEscape(stopAreaID))
	var arrRaw navitiaBoard
	if err := c.getJSON(ctx, arrPath, q, &arrRaw); err != nil {
		return out
	}
	for _, p := range arrRaw.Arrivals {
		trip := firstNonEmpty(p.DisplayInformations.TripShortName, p.DisplayInformations.Headsign)
		when := parseNavitiaDateTime(firstNonEmpty(p.StopDateTime.ArrivalDateTime, p.StopDateTime.DepartureDateTime))
		if trip == "" || when.IsZero() {
			continue
		}
		out[trip] = append(out[trip], when)
	}
	return out
}

func pickViaTime(ref time.Time, kind string, candidates []time.Time) (time.Time, bool) {
	if ref.IsZero() || len(candidates) == 0 {
		return time.Time{}, false
	}
	var best time.Time
	found := false
	for _, candidate := range candidates {
		if kind == "departures" {
			// Toward Angers: via/destination stops are after La Ménitré.
			if !candidate.After(ref) {
				continue
			}
			if !found || candidate.Before(best) {
				best = candidate
				found = true
			}
			continue
		}
		// Arrivals from Angers: Angers call must be before La Ménitré.
		if !candidate.Before(ref) {
			continue
		}
		if ref.Sub(candidate) > 3*time.Hour {
			continue
		}
		if !found || candidate.After(best) {
			best = candidate
			found = true
		}
	}
	return best, found
}

func (c *Client) getJSON(ctx context.Context, path string, q url.Values, dest any) error {
	u, err := url.Parse(c.baseURL + path)
	if err != nil {
		return err
	}
	if q != nil {
		u.RawQuery = q.Encode()
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return err
	}
	req.SetBasicAuth(c.token, "")
	req.Header.Set("Accept", "application/json")

	res, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("sncf request: %w", err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(io.LimitReader(res.Body, 2<<20))
	if err != nil {
		return fmt.Errorf("sncf read: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return fmt.Errorf("sncf api status %d: %s", res.StatusCode, truncate(string(body), 200))
	}
	if err := json.Unmarshal(body, dest); err != nil {
		return fmt.Errorf("sncf decode: %w", err)
	}
	return nil
}

func mapPassage(p navitiaPassage, kind string) Train {
	var when, base string
	if kind == "arrivals" {
		when = p.StopDateTime.ArrivalDateTime
		base = p.StopDateTime.BaseArrivalDateTime
	} else {
		when = p.StopDateTime.DepartureDateTime
		base = p.StopDateTime.BaseDepartureDateTime
	}

	mode := p.DisplayInformations.CommercialMode
	if mode == "" {
		mode = p.DisplayInformations.Label
	}
	if mode == "" {
		mode = p.DisplayInformations.Name
	}

	delay := delayMinutes(base, when)
	status := "à l'heure"
	if delay > 0 {
		status = fmt.Sprintf("+%d min", delay)
	} else if delay < 0 {
		status = "avance"
		delay = 0
	}

	return Train{
		Time:           formatNavitiaTime(when),
		BaseTime:       formatNavitiaTime(base),
		Direction:      firstNonEmpty(p.DisplayInformations.Direction, p.DisplayInformations.Headsign),
		Headsign:       p.DisplayInformations.Headsign,
		CommercialMode: mode,
		Network:        p.DisplayInformations.Network,
		TripName:       p.DisplayInformations.TripShortName,
		Status:         status,
		DelayMinutes:   delay,
	}
}

func parseNavitiaDateTime(raw string) time.Time {
	if raw == "" {
		return time.Time{}
	}
	t, err := time.Parse("20060102T150405", raw)
	if err != nil {
		return time.Time{}
	}
	return t
}

func formatNavitiaTime(raw string) string {
	t := parseNavitiaDateTime(raw)
	if t.IsZero() {
		if raw == "" {
			return ""
		}
		return raw
	}
	return t.Format("15:04")
}

func delayMinutes(base, actual string) int {
	if base == "" || actual == "" || base == actual {
		return 0
	}
	b := parseNavitiaDateTime(base)
	a := parseNavitiaDateTime(actual)
	if b.IsZero() || a.IsZero() {
		return 0
	}
	return int(a.Sub(b).Minutes())
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
