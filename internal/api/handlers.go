package api

import (
	"encoding/csv"
	"encoding/json"
	"log"
	"net/http"
	"maps"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/liviro/boob-o-clock/internal/domain"
	"github.com/liviro/boob-o-clock/internal/reports"
	"github.com/liviro/boob-o-clock/internal/store"
)

type Handler struct {
	store *store.Store
}

func NewHandler(s *store.Store) *Handler {
	return &Handler{store: s}
}

type eventRequest struct {
	Action    string            `json:"action"`
	Metadata  map[string]string `json:"metadata"`
	Timestamp string            `json:"timestamp"`
}

// ferberCurrent captures the in-progress Ferber learning session. Present only
// when the current state is Learning or CheckIn.
type ferberCurrent struct {
	CheckInCount int       `json:"checkInCount"`
	StartedAt    time.Time `json:"startedAt"`
	// CheckInAvailableAt is populated only in Learning state — the moment
	// the next check-in button becomes tappable. Absent during CheckIn.
	CheckInAvailableAt *time.Time `json:"checkInAvailableAt,omitempty"`
	Mood               string     `json:"mood"`
}

// ferberNight is the Ferber-specific context for the current night. Absent
// when the current night is non-Ferber (or there is no current night).
type ferberNight struct {
	NightNumber int            `json:"nightNumber"`
	Current     *ferberCurrent `json:"current,omitempty"`
}

type sessionResponse struct {
	State             domain.State    `json:"state"`
	ValidActions      []domain.Action `json:"validActions"`
	NightID           *int64          `json:"nightId"`
	LastEvent         *eventResponse  `json:"lastEvent"`
	SuggestBreast     string          `json:"suggestBreast,omitempty"`
	CurrentBreast     string          `json:"currentBreast,omitempty"`
	LastFeedStartedAt *time.Time      `json:"lastFeedStartedAt,omitempty"`
	// Present when the current night is a Ferber night; absent otherwise.
	Ferber *ferberNight `json:"ferber,omitempty"`
	// Present on NightOff when a recent Ferber sequence exists: the suggested
	// next Ferber night number (last + 1) for the Start Night form.
	SuggestFerberNight *int `json:"suggestFerberNight,omitempty"`
}

type ferberConfigRequest struct {
	NightNumber int `json:"nightNumber"`
}

// startNightRequest is the typed body for POST /api/session/start.
// Presence of Ferber implies the night runs in Ferber mode with the given
// night number; absence means a plain night.
type startNightRequest struct {
	Timestamp string               `json:"timestamp,omitempty"`
	Ferber    *ferberConfigRequest `json:"ferber,omitempty"`
}

type eventResponse struct {
	Action    domain.Action     `json:"action"`
	FromState domain.State      `json:"fromState"`
	ToState   domain.State      `json:"toState"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

func toEventResponse(e domain.Event) *eventResponse {
	return &eventResponse{
		Action:    e.Action,
		FromState: e.FromState,
		ToState:   e.ToState,
		Metadata:  e.Metadata,
		Timestamp: e.Timestamp,
	}
}

func (h *Handler) buildSessionResponse(state domain.State, night *domain.Night, events []domain.Event) sessionResponse {
	lastBreast := reports.LastBreastUsed(events)
	resp := sessionResponse{
		State:             state,
		ValidActions:      reports.SelectActionsForNight(domain.ValidActions(state), night != nil && night.FerberEnabled),
		SuggestBreast:     reports.SuggestedBreast(lastBreast),
		CurrentBreast:     lastBreast,
		LastFeedStartedAt: reports.LastFeedStart(events),
	}
	if night != nil {
		resp.NightID = &night.ID
		if night.FerberEnabled && night.FerberNightNumber != nil {
			resp.Ferber = &ferberNight{NightNumber: *night.FerberNightNumber}
			if session := reports.CurrentFerberSession(state, events, *night.FerberNightNumber); session != nil {
				resp.Ferber.Current = &ferberCurrent{
					CheckInCount:       session.CheckIns,
					StartedAt:          session.SessionStart,
					CheckInAvailableAt: session.CheckInAvailableAt,
					Mood:               session.Mood,
				}
			}
		}
	}
	if len(events) > 0 {
		resp.LastEvent = toEventResponse(events[len(events)-1])
	}
	if state == domain.NightOff {
		last, err := h.store.LastNight()
		if err != nil {
			// Don't fail the whole session fetch over a missing hint, but log so
			// the developer can see if this starts happening systematically.
			log.Printf("buildSessionResponse: LastNight lookup failed: %v", err)
		} else {
			resp.SuggestFerberNight = reports.SuggestFerberNight(last)
		}
	}
	return resp
}

// GetCurrentSession returns the current state and valid actions.
func (h *Handler) GetCurrentSession(w http.ResponseWriter, r *http.Request) {
	night, events, err := h.store.CurrentSession()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	state := domain.DeriveState(events)

	writeJSON(w, http.StatusOK, h.buildSessionResponse(state, night, events))
}

// PostEvent records a new event and returns the updated session.
func (h *Handler) PostEvent(w http.ResponseWriter, r *http.Request) {
	var req eventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	action := domain.Action(req.Action)
	if action == domain.StartNight {
		writeError(w, http.StatusBadRequest, "use POST /api/session/start to start a night")
		return
	}

	ts := time.Now()
	if req.Timestamp != "" {
		parsed, err := time.Parse(time.RFC3339, req.Timestamp)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid timestamp format (use RFC3339)")
			return
		}
		ts = parsed
	}

	night, events, err := h.store.CurrentSession()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if night == nil {
		writeError(w, http.StatusBadRequest, "no active night session")
		return
	}

	currentState := domain.DeriveState(events)

	nextState, err := domain.Transition(currentState, action, req.Metadata)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	evt := &domain.Event{
		NightID:   night.ID,
		FromState: currentState,
		Action:    action,
		ToState:   nextState,
		Timestamp: ts,
		Metadata:  req.Metadata,
	}
	if err := h.store.AddEvent(evt); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if action == domain.EndNight {
		if err := h.store.EndNight(night.ID, ts); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	var responseNight *domain.Night
	if nextState != domain.NightOff {
		responseNight = night
	}
	writeJSON(w, http.StatusOK, h.buildSessionResponse(nextState, responseNight, append(events, *evt)))
}

// StartNight creates a new night with optional Ferber config and records the
// start_night event. Separate from PostEvent because start_night is the only
// action that creates a nights row, and its config is typed (not string-keyed
// event metadata).
func (h *Handler) StartNight(w http.ResponseWriter, r *http.Request) {
	var req startNightRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if req.Ferber != nil && req.Ferber.NightNumber < 1 {
		writeError(w, http.StatusBadRequest, "ferber.nightNumber must be >= 1")
		return
	}

	ts := time.Now()
	if req.Timestamp != "" {
		parsed, err := time.Parse(time.RFC3339, req.Timestamp)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid timestamp format (use RFC3339)")
			return
		}
		ts = parsed
	}

	_, events, err := h.store.CurrentSession()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	currentState := domain.DeriveState(events)
	nextState, err := domain.Transition(currentState, domain.StartNight, nil)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ferberEnabled := req.Ferber != nil
	ferberNightNumber := 0
	if ferberEnabled {
		ferberNightNumber = req.Ferber.NightNumber
	}
	night, err := h.store.CreateNight(ts, ferberEnabled, ferberNightNumber)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	evt := &domain.Event{
		NightID:   night.ID,
		FromState: currentState,
		Action:    domain.StartNight,
		ToState:   nextState,
		Timestamp: ts,
	}
	if err := h.store.AddEvent(evt); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, h.buildSessionResponse(nextState, night, []domain.Event{*evt}))
}

// PostUndo removes the last event and returns the updated session.
func (h *Handler) PostUndo(w http.ResponseWriter, r *http.Request) {
	night, events, err := h.store.CurrentSession()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if night == nil {
		writeError(w, http.StatusBadRequest, "no active night session to undo")
		return
	}

	if len(events) == 0 {
		writeError(w, http.StatusBadRequest, "no events to undo")
		return
	}

	lastEvent := events[len(events)-1]

	if lastEvent.Action == domain.StartNight {
		if err := h.store.DeleteNight(night.ID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, h.buildSessionResponse(domain.NightOff, nil, nil))
		return
	}

	if _, err := h.store.PopEvent(night.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	remaining := events[:len(events)-1]
	state := domain.DeriveState(remaining)
	writeJSON(w, http.StatusOK, h.buildSessionResponse(state, night, remaining))
}

// GetNightDetail returns a night with its timeline, stats, and raw events.
func (h *Handler) GetNightDetail(w http.ResponseWriter, r *http.Request) {
	idStr := r.PathValue("id")
	id, err := strconv.ParseInt(idStr, 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid night ID")
		return
	}

	night, events, err := h.store.GetNight(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if night == nil {
		writeError(w, http.StatusNotFound, "night not found")
		return
	}

	eventJSONs := make([]eventResponse, len(events))
	for i, e := range events {
		eventJSONs[i] = *toEventResponse(e)
	}

	stats, timeline := reports.ComputeStats(events, night)

	writeJSON(w, http.StatusOK, map[string]any{
		"night": nightDetailJSON{
			ID:                night.ID,
			StartedAt:         night.StartedAt,
			EndedAt:           night.EndedAt,
			FerberEnabled:     night.FerberEnabled,
			FerberNightNumber: night.FerberNightNumber,
		},
		"events":   eventJSONs,
		"timeline": timeline,
		"stats":    stats,
	})
}

func parseDateRange(r *http.Request) (from, to time.Time) {
	to = time.Now().Add(24 * time.Hour)
	from = to.Add(-90 * 24 * time.Hour)

	if f := r.URL.Query().Get("from"); f != "" {
		if parsed, err := time.Parse("2006-01-02", f); err == nil {
			from = parsed
		}
	}
	if t := r.URL.Query().Get("to"); t != "" {
		if parsed, err := time.Parse("2006-01-02", t); err == nil {
			to = parsed.Add(24 * time.Hour)
		}
	}
	return
}

// loadNightsWithEvents fetches nights and their events for a date range.
func (h *Handler) loadNightsWithEvents(r *http.Request) ([]domain.Night, map[int64][]domain.Event, error) {
	from, to := parseDateRange(r)
	nights, err := h.store.ListNights(from, to)
	if err != nil {
		return nil, nil, err
	}

	nightIDs := make([]int64, len(nights))
	for i, n := range nights {
		nightIDs[i] = n.ID
	}
	eventsMap, err := h.store.GetEventsForNights(nightIDs)
	if err != nil {
		return nil, nil, err
	}
	return nights, eventsMap, nil
}

// GetNights returns a list of nights with summary stats.
func (h *Handler) GetNights(w http.ResponseWriter, r *http.Request) {
	nights, eventsMap, err := h.loadNightsWithEvents(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	type nightSummary struct {
		nightDetailJSON
		Stats reports.NightStats `json:"stats"`
	}

	summaries := make([]nightSummary, 0, len(nights))
	for _, n := range nights {
		stats, _ := reports.ComputeStats(eventsMap[n.ID], &n)
		summaries = append(summaries, nightSummary{
			nightDetailJSON: nightDetailJSON{
				ID:                n.ID,
				StartedAt:         n.StartedAt,
				EndedAt:           n.EndedAt,
				FerberEnabled:     n.FerberEnabled,
				FerberNightNumber: n.FerberNightNumber,
			},
			Stats: stats,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"nights": summaries,
	})
}

// GetTrends returns trend data with moving averages for charting.
func (h *Handler) GetTrends(w http.ResponseWriter, r *http.Request) {
	nights, eventsMap, err := h.loadNightsWithEvents(r)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	points := make([]reports.NightDataPoint, len(nights))
	for i, n := range nights {
		stats, _ := reports.ComputeStats(eventsMap[n.ID], &n)
		points[i] = reports.NightDataPoint{
			Date:  n.StartedAt,
			Stats: stats,
		}
	}

	window := 3
	trends := reports.ComputeTrends(points, window)

	writeJSON(w, http.StatusOK, map[string]any{
		"trends": trends,
		"window": window,
	})
}

// ExportCSV streams all events as a CSV file.
func (h *Handler) ExportCSV(w http.ResponseWriter, r *http.Request) {
	events, err := h.store.GetAllEvents()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	w.Header().Set("Content-Type", "text/csv")
	w.Header().Set("Content-Disposition", `attachment; filename="boob-o-clock-export.csv"`)

	cw := csv.NewWriter(w)
	cw.Write([]string{"night_id", "seq", "from_state", "action", "to_state", "timestamp", "breast", "metadata"})

	for _, e := range events {
		breast := e.Metadata["breast"]
		var metaParts []string
		for _, k := range slices.Sorted(maps.Keys(e.Metadata)) {
			if k == "breast" {
				continue
			}
			metaParts = append(metaParts, k+"="+e.Metadata[k])
		}
		cw.Write([]string{
			strconv.FormatInt(e.NightID, 10),
			strconv.Itoa(e.Seq),
			string(e.FromState),
			string(e.Action),
			string(e.ToState),
			e.Timestamp.Format(time.RFC3339),
			breast,
			strings.Join(metaParts, ";"),
		})
	}
	cw.Flush()
	if err := cw.Error(); err != nil {
		log.Printf("CSV flush error: %v", err)
	}
}

type nightDetailJSON struct {
	ID                int64      `json:"id"`
	StartedAt         time.Time  `json:"startedAt"`
	EndedAt           *time.Time `json:"endedAt,omitempty"`
	FerberEnabled     bool       `json:"ferberEnabled"`
	FerberNightNumber *int       `json:"ferberNightNumber,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("JSON encode error: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
