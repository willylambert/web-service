package sncf

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
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

	// boardLookback keeps recently departed trains visible until they reach destination.
	boardLookback = 90 * time.Minute
	// destinationGrace keeps a train a short while after the scheduled destination time.
	destinationGrace = 2 * time.Minute
)

// Destination keywords for departures from La Ménitré toward Angers
// (Nantes-bound trains also stop at Angers on this line).
var departureTowardAngers = []string{"angers", "nantes"}

var parisLocation = mustLoadLocation("Europe/Paris")

func mustLoadLocation(name string) *time.Location {
	loc, err := time.LoadLocation(name)
	if err != nil {
		return time.FixedZone("CET", 3600)
	}
	return loc
}

// Client talks to the SNCF (Navitia) Open Data API.
type Client struct {
	token      string
	baseURL    string
	stopAreaID string
	httpClient *http.Client
	now        func() time.Time
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

func (c *Client) clock() time.Time {
	if c != nil && c.now != nil {
		return c.now()
	}
	return time.Now()
}

// navitiaNow returns the current Paris wall-clock as a UTC-shaped time, matching
// parseNavitiaDateTime (Navitia timestamps are local Europe/Paris without zone).
func (c *Client) navitiaNow() time.Time {
	now := c.clock().In(parisLocation)
	return time.Date(now.Year(), now.Month(), now.Day(), now.Hour(), now.Minute(), now.Second(), 0, time.UTC)
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
	PhysicalMode       string `json:"physical_mode,omitempty"`
	Company            string `json:"company,omitempty"`
	Line               string `json:"line,omitempty"`
	TripName           string `json:"trip_name,omitempty"`
	VehicleJourneyID   string `json:"vehicle_journey_id,omitempty"`
	Platform           string `json:"platform,omitempty"`
	Status             string `json:"status"`
	StatusDetail       string `json:"status_detail,omitempty"`
	DelayMinutes       int    `json:"delay_minutes,omitempty"`
	SaintMathurinTime  string `json:"saint_mathurin_time,omitempty"`
	AngersTime         string `json:"angers_time,omitempty"`
	// Deprecated aliases kept for older UI builds.
	ViaStop     string `json:"via_stop,omitempty"`
	ViaStopTime string `json:"via_stop_time,omitempty"`
}

// JourneyStop is one call on a vehicle journey.
type JourneyStop struct {
	Name               string `json:"name"`
	ArrivalTime        string `json:"arrival_time,omitempty"`
	DepartureTime      string `json:"departure_time,omitempty"`
	ArrivalDateTime    string `json:"arrival_datetime,omitempty"`
	DepartureDateTime  string `json:"departure_datetime,omitempty"`
	BaseArrivalTime    string `json:"base_arrival_time,omitempty"`
	BaseDepartureTime  string `json:"base_departure_time,omitempty"`
}

// JourneyDetail is the stop list for a vehicle journey.
type JourneyDetail struct {
	VehicleJourneyID string        `json:"vehicle_journey_id"`
	ServiceDate      string        `json:"service_date,omitempty"`
	Stops            []JourneyStop `json:"stops"`
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
		PhysicalMode   string `json:"physical_mode"`
		Company        string `json:"company"`
		Code           string `json:"code"`
		TripShortName  string `json:"trip_short_name"`
		Name           string `json:"name"`
	} `json:"display_informations"`
	StopDateTime struct {
		DepartureDateTime      string   `json:"departure_date_time"`
		BaseDepartureDateTime  string   `json:"base_departure_date_time"`
		ArrivalDateTime        string   `json:"arrival_date_time"`
		BaseArrivalDateTime    string   `json:"base_arrival_date_time"`
		AdditionalInformations []string `json:"additional_informations"`
		DepartureStatus        string   `json:"departure_status"`
		ArrivalStatus          string   `json:"arrival_status"`
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
	Disruptions []navitiaJourneyDisruption `json:"disruptions"`
}

type navitiaJourneyDisruption struct {
	Severity struct {
		Effect string `json:"effect"`
	} `json:"severity"`
	ImpactedObjects []struct {
		ImpactedStops []struct {
			StopPoint struct {
				Name string `json:"name"`
			} `json:"stop_point"`
			BaseArrivalTime       string `json:"base_arrival_time"`
			BaseDepartureTime     string `json:"base_departure_time"`
			AmendedArrivalTime    string `json:"amended_arrival_time"`
			AmendedDepartureTime  string `json:"amended_departure_time"`
			ArrivalStatus         string `json:"arrival_status"`
			DepartureStatus       string `json:"departure_status"`
		} `json:"impacted_stops"`
	} `json:"impacted_objects"`
}

type navitiaTrafficReports struct {
	Disruptions []struct {
		Severity struct {
			Effect string `json:"effect"`
			Name   string `json:"name"`
		} `json:"severity"`
		Messages []struct {
			Text string `json:"text"`
		} `json:"messages"`
		ApplicationPeriods []struct {
			Begin string `json:"begin"`
			End   string `json:"end"`
		} `json:"application_periods"`
		ImpactedObjects []struct {
			PtObject struct {
				EmbeddedType string `json:"embedded_type"`
				Trip         struct {
					Name string `json:"name"`
				} `json:"trip"`
			} `json:"pt_object"`
		} `json:"impacted_objects"`
	} `json:"disruptions"`
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

	// Over-fetch so destination filters still fill the board, including trains
	// already past La Ménitré but not yet at destination.
	fetchCount := 40
	now := c.navitiaNow()
	from := now.Add(-boardLookback)

	path := fmt.Sprintf("/coverage/sncf/stop_areas/%s/%s", url.PathEscape(c.stopAreaID), kind)
	realtimeQ := boardQuery(from)
	realtimeQ.Set("count", fmt.Sprintf("%d", fetchCount))

	var raw navitiaBoard
	if err := c.getJSON(ctx, path, realtimeQ, &raw); err != nil {
		return nil, err
	}

	passages := raw.Departures
	if kind == "arrivals" {
		passages = raw.Arrivals
	}

	// Base schedule keeps theoretical calls that realtime drops when a trip is cancelled.
	baseQ := boardQuery(from)
	baseQ.Set("count", fmt.Sprintf("%d", fetchCount))
	baseQ.Set("data_freshness", "base_schedule")
	var baseRaw navitiaBoard
	_ = c.getJSON(ctx, path, baseQ, &baseRaw)
	basePassages := baseRaw.Departures
	if kind == "arrivals" {
		basePassages = baseRaw.Arrivals
	}

	cancelled := c.cancelledTripNames(ctx, now)

	var (
		saintMathurinIndex map[string][]time.Time
		angersIndex        map[string][]time.Time
		smArrivalIndex     map[string][]time.Time
	)
	saintMathurinIndex = c.stopTimesIndex(ctx, SaintMathurinStopArea, from)
	angersIndex = c.stopTimesIndex(ctx, AngersStopArea, from)
	if kind == "arrivals" {
		smArrivalIndex = c.stopArrivalsIndex(ctx, SaintMathurinStopArea, from)
	}

	type candidate struct {
		train Train
		ref   time.Time
	}
	seen := make(map[string]int, len(passages)+len(basePassages))
	candidates := make([]candidate, 0, len(passages)+len(basePassages))

	appendTrain := func(p navitiaPassage, forceCancelled bool) {
		train, ref, ok := c.buildTrain(ctx, p, kind, saintMathurinIndex, angersIndex, smArrivalIndex)
		if !ok {
			return
		}
		reason := cancelled[train.TripName]
		if forceCancelled || reason != "" || passageDeleted(p, kind) {
			train.Status = "supprimé"
			train.DelayMinutes = 0
			if reason != "" {
				train.StatusDetail = reason
			}
		}
		if !stillEnRoute(train, kind, ref, now) {
			return
		}
		if train.Status != "supprimé" {
			markEnRouteStatus(&train, kind, ref, now)
		}

		key := train.TripName
		if key == "" {
			key = train.Time + "|" + train.Direction
		}
		if idx, exists := seen[key]; exists {
			// Prefer cancelled / richer enrichment if we see the trip again.
			if train.Status == "supprimé" || candidates[idx].train.AngersTime == "" {
				candidates[idx] = candidate{train: train, ref: ref}
			}
			return
		}
		seen[key] = len(candidates)
		candidates = append(candidates, candidate{train: train, ref: ref})
	}

	for _, p := range passages {
		appendTrain(p, false)
	}
	for _, p := range basePassages {
		trip := firstNonEmpty(p.DisplayInformations.TripShortName, p.DisplayInformations.Headsign)
		if trip == "" {
			continue
		}
		if _, isCancelled := cancelled[trip]; !isCancelled && !passageDeleted(p, kind) {
			continue
		}
		appendTrain(p, true)
	}

	sort.SliceStable(candidates, func(i, j int) bool {
		if candidates[i].ref.Equal(candidates[j].ref) {
			return candidates[i].train.Time < candidates[j].train.Time
		}
		return candidates[i].ref.Before(candidates[j].ref)
	})

	trains := make([]Train, 0, count)
	for _, item := range candidates {
		trains = append(trains, item.train)
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

func (c *Client) buildTrain(
	ctx context.Context,
	p navitiaPassage,
	kind string,
	saintMathurinIndex, angersIndex, smArrivalIndex map[string][]time.Time,
) (Train, time.Time, bool) {
	train := mapPassage(p, kind)
	if kind == "departures" && !towardAngers(train) {
		return Train{}, time.Time{}, false
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
		return train, ref, true
	}

	when, ok := pickViaTime(ref, "arrivals", angersIndex[train.TripName])
	if !ok {
		// Keep only trains that stopped in Angers before La Ménitré.
		// Cancelled trips may be absent from realtime indexes: try vehicle journey.
		c.enrichFromVehicleJourney(ctx, &train, p, ref)
		if train.AngersTime == "" {
			return Train{}, time.Time{}, false
		}
		angersAt := parseClockOnRef(ref, train.AngersTime)
		if angersAt.IsZero() || !angersAt.Before(ref) {
			return Train{}, time.Time{}, false
		}
	} else {
		train.AngersTime = when.Format("15:04")
	}
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
	return train, ref, true
}

func passageRefTime(p navitiaPassage, kind string) time.Time {
	raw := p.StopDateTime.DepartureDateTime
	if kind == "arrivals" {
		raw = p.StopDateTime.ArrivalDateTime
	}
	return parseNavitiaDateTime(raw)
}

func (c *Client) enrichFromVehicleJourney(ctx context.Context, train *Train, p navitiaPassage, ref time.Time) {
	vjID := passageVehicleJourneyID(p)
	if vjID == "" || ref.IsZero() {
		return
	}
	if train.VehicleJourneyID == "" {
		train.VehicleJourneyID = vjID
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

func (c *Client) stopTimesIndex(ctx context.Context, stopAreaID string, from time.Time) map[string][]time.Time {
	out := make(map[string][]time.Time)
	q := boardQuery(from)

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

func (c *Client) stopArrivalsIndex(ctx context.Context, stopAreaID string, from time.Time) map[string][]time.Time {
	out := make(map[string][]time.Time)
	q := boardQuery(from)

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

func boardQuery(from time.Time) url.Values {
	q := url.Values{}
	q.Set("count", "40")
	q.Set("data_freshness", "realtime")
	if !from.IsZero() {
		q.Set("from_datetime", from.Format("20060102T150405"))
	}
	return q
}

// cancelledTripNames returns today's NO_SERVICE trip short names → message.
func (c *Client) cancelledTripNames(ctx context.Context, now time.Time) map[string]string {
	out := make(map[string]string)
	q := url.Values{}
	q.Set("count", "200")
	var raw navitiaTrafficReports
	if err := c.getJSON(ctx, "/coverage/sncf/traffic_reports", q, &raw); err != nil {
		return out
	}
	day := now.Format("20060102")
	for _, dis := range raw.Disruptions {
		if !strings.EqualFold(dis.Severity.Effect, "NO_SERVICE") {
			continue
		}
		active := false
		for _, period := range dis.ApplicationPeriods {
			begin := period.Begin
			end := period.End
			if len(begin) >= 8 && begin[:8] <= day {
				if end == "" || len(end) < 8 || day <= end[:8] {
					active = true
					break
				}
			}
		}
		if !active {
			continue
		}
		msg := ""
		if len(dis.Messages) > 0 {
			msg = strings.TrimSpace(dis.Messages[0].Text)
		}
		for _, obj := range dis.ImpactedObjects {
			if !strings.EqualFold(obj.PtObject.EmbeddedType, "trip") {
				continue
			}
			name := strings.TrimSpace(obj.PtObject.Trip.Name)
			if name == "" {
				continue
			}
			if _, exists := out[name]; !exists || (out[name] == "" && msg != "") {
				out[name] = msg
			}
		}
	}
	return out
}

func passageDeleted(p navitiaPassage, kind string) bool {
	status := p.StopDateTime.DepartureStatus
	if kind == "arrivals" {
		status = p.StopDateTime.ArrivalStatus
	}
	if strings.EqualFold(status, "deleted") {
		return true
	}
	for _, info := range p.StopDateTime.AdditionalInformations {
		switch strings.ToLower(info) {
		case "deleted", "deleted_departure", "deleted_arrival", "skipped_stop":
			return true
		}
	}
	return false
}

// stillEnRoute keeps a train visible until it has reached its destination.
// Departures: destination is Angers. Arrivals: destination is La Ménitré.
func stillEnRoute(train Train, kind string, ref, now time.Time) bool {
	if now.IsZero() {
		return true
	}
	destination := destinationTime(train, kind, ref)
	if destination.IsZero() {
		// Without a destination clock, drop trains already past the local stop
		// (plus a short grace), otherwise keep upcoming ones.
		local := ref
		if local.IsZero() {
			local = parseClockOnRef(now, train.Time)
		}
		if local.IsZero() {
			return true
		}
		return !now.After(local.Add(destinationGrace))
	}
	return !now.After(destination.Add(destinationGrace))
}

func destinationTime(train Train, kind string, ref time.Time) time.Time {
	if kind == "departures" {
		return parseClockOnRef(ref, train.AngersTime)
	}
	if !ref.IsZero() {
		return ref
	}
	return parseClockOnRef(time.Time{}, train.Time)
}

func markEnRouteStatus(train *Train, kind string, ref, now time.Time) {
	if train == nil || now.IsZero() || ref.IsZero() {
		return
	}
	destination := destinationTime(*train, kind, ref)
	if destination.IsZero() || !now.Before(destination) {
		return
	}
	leftOrigin := false
	switch kind {
	case "departures":
		leftOrigin = !now.Before(ref)
	case "arrivals":
		angersAt := parseClockOnRef(ref, train.AngersTime)
		leftOrigin = !angersAt.IsZero() && !now.Before(angersAt)
	}
	if leftOrigin {
		train.Status = "en route"
	}
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
		Time:             formatNavitiaTime(when),
		BaseTime:         formatNavitiaTime(base),
		Direction:        firstNonEmpty(p.DisplayInformations.Direction, p.DisplayInformations.Headsign),
		Headsign:         p.DisplayInformations.Headsign,
		CommercialMode:   mode,
		Network:          p.DisplayInformations.Network,
		PhysicalMode:     p.DisplayInformations.PhysicalMode,
		Company:          p.DisplayInformations.Company,
		Line:             firstNonEmpty(p.DisplayInformations.Code, p.DisplayInformations.Label),
		TripName:         p.DisplayInformations.TripShortName,
		VehicleJourneyID: passageVehicleJourneyID(p),
		Status:           status,
		DelayMinutes:     delay,
	}
}

func passageVehicleJourneyID(p navitiaPassage) string {
	for _, link := range p.Links {
		if link.Type == "vehicle_journey" && link.ID != "" {
			return link.ID
		}
	}
	return ""
}

// JourneyStops fetches ordered stops for a vehicle journey id.
func (c *Client) JourneyStops(ctx context.Context, vehicleJourneyID string) (*JourneyDetail, error) {
	if !c.Configured() {
		return nil, fmt.Errorf("SNCF API token not configured")
	}
	vjID := strings.TrimSpace(vehicleJourneyID)
	if vjID == "" {
		return nil, fmt.Errorf("vehicle_journey_id required")
	}

	path := "/coverage/sncf/vehicle_journeys/" + url.PathEscape(vjID)
	var raw navitiaVehicleJourneys
	if err := c.getJSON(ctx, path, nil, &raw); err != nil {
		return nil, err
	}
	if len(raw.VehicleJourneys) == 0 {
		return &JourneyDetail{VehicleJourneyID: vjID, Stops: []JourneyStop{}}, nil
	}

	serviceDay := serviceDateFromVehicleJourneyID(vjID)
	if serviceDay.IsZero() {
		serviceDay = c.navitiaNow()
	}
	amended := amendedStopTimes(raw.Disruptions)

	refDay := serviceDay
	var prev time.Time
	stops := make([]JourneyStop, 0, len(raw.VehicleJourneys[0].StopTimes))
	for _, stop := range raw.VehicleJourneys[0].StopTimes {
		name := strings.TrimSpace(stop.StopPoint.Name)
		if name == "" {
			continue
		}
		arr := clockOnDate(refDay, stop.ArrivalTime)
		dep := clockOnDate(refDay, stop.DepartureTime)
		point := dep
		if point.IsZero() {
			point = arr
		}
		// Vehicle journeys can cross midnight: bump the service day when clocks go backwards.
		if !prev.IsZero() && !point.IsZero() && point.Before(prev.Add(-3*time.Hour)) {
			refDay = refDay.Add(24 * time.Hour)
			arr = clockOnDate(refDay, stop.ArrivalTime)
			dep = clockOnDate(refDay, stop.DepartureTime)
			point = dep
			if point.IsZero() {
				point = arr
			}
		}
		if !point.IsZero() {
			prev = point
		}

		item := JourneyStop{Name: name}
		if !arr.IsZero() {
			item.ArrivalTime = arr.Format("15:04")
			item.ArrivalDateTime = arr.Format("2006-01-02T15:04:05")
		}
		if !dep.IsZero() {
			item.DepartureTime = dep.Format("15:04")
			item.DepartureDateTime = dep.Format("2006-01-02T15:04:05")
		}
		if patch, ok := amended[normalizeStopName(name)]; ok {
			if patch.baseArr != "" {
				item.BaseArrivalTime = formatHHMMSS(patch.baseArr)
			}
			if patch.baseDep != "" {
				item.BaseDepartureTime = formatHHMMSS(patch.baseDep)
			}
			if patch.amendedArr != "" {
				if t := clockOnDate(refDay, patch.amendedArr); !t.IsZero() {
					item.ArrivalTime = t.Format("15:04")
					item.ArrivalDateTime = t.Format("2006-01-02T15:04:05")
				}
			}
			if patch.amendedDep != "" {
				if t := clockOnDate(refDay, patch.amendedDep); !t.IsZero() {
					item.DepartureTime = t.Format("15:04")
					item.DepartureDateTime = t.Format("2006-01-02T15:04:05")
				}
			}
		}
		stops = append(stops, item)
	}
	return &JourneyDetail{
		VehicleJourneyID: vjID,
		ServiceDate:      serviceDay.Format("2006-01-02"),
		Stops:            stops,
	}, nil
}

type stopTimePatch struct {
	baseArr, baseDep, amendedArr, amendedDep string
}

func amendedStopTimes(disruptions []navitiaJourneyDisruption) map[string]stopTimePatch {
	out := make(map[string]stopTimePatch)
	for _, dis := range disruptions {
		for _, obj := range dis.ImpactedObjects {
			for _, st := range obj.ImpactedStops {
				name := normalizeStopName(st.StopPoint.Name)
				if name == "" {
					continue
				}
				out[name] = stopTimePatch{
					baseArr:     st.BaseArrivalTime,
					baseDep:     st.BaseDepartureTime,
					amendedArr:  st.AmendedArrivalTime,
					amendedDep:  st.AmendedDepartureTime,
				}
			}
		}
	}
	return out
}

func normalizeStopName(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

func serviceDateFromVehicleJourneyID(id string) time.Time {
	// vehicle_journey:SNCF:2026-10-10:859041:1187:LongDistanceTrain...
	parts := strings.Split(id, ":")
	for _, part := range parts {
		if t, err := time.Parse("2006-01-02", part); err == nil {
			return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, time.UTC)
		}
	}
	return time.Time{}
}

func formatHHMMSS(raw string) string {
	t := clockOnDate(time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC), raw)
	if t.IsZero() {
		return ""
	}
	return t.Format("15:04")
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
