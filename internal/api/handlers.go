package api

import (
	"encoding/csv"
	"encoding/json"
	"log"
	"maps"
	"net/http"
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

// --- request / response types ---

type eventRequest struct {
	Action    string            `json:"action"`
	Metadata  map[string]string `json:"metadata"`
	Timestamp string            `json:"timestamp"`
}

// ferberCurrent captures the in-progress Ferber learning session. Present only
// when the current state is Learning or CheckIn.
type ferberCurrent struct {
	CheckInCount       int        `json:"checkInCount"`
	StartedAt          time.Time  `json:"startedAt"`
	CheckInAvailableAt *time.Time `json:"checkInAvailableAt,omitempty"`
	Mood               string     `json:"mood"`
}

type ferberNight struct {
	NightNumber int            `json:"nightNumber"`
	Current     *ferberCurrent `json:"current,omitempty"`
}

type sessionResponse struct {
	// Kind is nil iff there is no active session (state == NightOff, no row).
	Kind               *domain.SessionKind `json:"kind"`
	State              domain.State        `json:"state"`
	ValidActions       []domain.Action     `json:"validActions"`
	SessionID          *int64              `json:"sessionId"`
	LastEvent          *eventResponse      `json:"lastEvent"`
	SuggestBreast      string              `json:"suggestBreast,omitempty"`
	CurrentBreast      string              `json:"currentBreast,omitempty"`
	LastFeedStartedAt  *time.Time          `json:"lastFeedStartedAt,omitempty"`
	Ferber             *ferberNight        `json:"ferber,omitempty"`
	SuggestFerberNight *int                `json:"suggestFerberNight,omitempty"`
}

type ferberConfigRequest struct {
	NightNumber int `json:"nightNumber"`
}

// startSessionRequest is the typed body for POST /api/session/start.
// Kind is required; ferber is only valid when kind == "night".
type startSessionRequest struct {
	Kind      string               `json:"kind"`
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

// --- session response builder ---

func (h *Handler) buildSessionResponse(state domain.State, session *domain.Session, events []domain.Event) sessionResponse {
	lastBreast := reports.LastBreastUsed(events)
	resp := sessionResponse{
		State:             state,
		SuggestBreast:     reports.SuggestedBreast(lastBreast),
		CurrentBreast:     lastBreast,
		LastFeedStartedAt: reports.LastFeedStart(events),
	}

	ferberEnabled := session != nil && session.IsNight() && session.FerberEnabled
	resp.ValidActions = reports.SelectActionsForNight(domain.ValidActions(state), ferberEnabled)

	if session != nil {
		resp.Kind = &session.Kind
		resp.SessionID = &session.ID
		if ferberEnabled && session.FerberNightNumber != nil {
			resp.Ferber = &ferberNight{NightNumber: *session.FerberNightNumber}
			if fs := reports.CurrentFerberSession(state, events, *session.FerberNightNumber); fs != nil {
				resp.Ferber.Current = &ferberCurrent{
					CheckInCount:       fs.CheckIns,
					StartedAt:          fs.SessionStart,
					CheckInAvailableAt: fs.CheckInAvailableAt,
					Mood:               fs.Mood,
				}
			}
		}
	}

	if len(events) > 0 {
		resp.LastEvent = toEventResponse(events[len(events)-1])
	}

	// Suggest Ferber night wherever start_night is a valid action: that's
	// NightOff (first-start) AND DayAwake (chain advance at bedtime).
	if state == domain.NightOff || state == domain.DayAwake {
		last, err := h.store.LastSession(domain.SessionKindNight)
		if err != nil {
			log.Printf("buildSessionResponse: LastSession(night) lookup failed: %v", err)
		} else {
			resp.SuggestFerberNight = reports.SuggestFerberNight(last)
		}
	}
	return resp
}

// --- session endpoints ---

// GetCurrentSession returns the current state and valid actions.
func (h *Handler) GetCurrentSession(w http.ResponseWriter, r *http.Request) {
	session, events, err := h.store.CurrentSession()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	state := domain.DeriveState(events)
	writeJSON(w, http.StatusOK, h.buildSessionResponse(state, session, events))
}

// StartSession creates a new session of the specified kind. Atomically closes
// any currently-open session (chain advance). Also handles first-start (no
// prior session) identically.
func (h *Handler) StartSession(w http.ResponseWriter, r *http.Request) {
	var req startSessionRequest
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	var kind domain.SessionKind
	switch req.Kind {
	case string(domain.SessionKindNight):
		kind = domain.SessionKindNight
	case string(domain.SessionKindDay):
		kind = domain.SessionKindDay
	default:
		writeError(w, http.StatusBadRequest, "kind must be 'night' or 'day'")
		return
	}

	if req.Ferber != nil {
		if kind != domain.SessionKindNight {
			writeError(w, http.StatusBadRequest, "ferber config only valid when kind=night")
			return
		}
		if req.Ferber.NightNumber < 1 {
			writeError(w, http.StatusBadRequest, "ferber.nightNumber must be >= 1")
			return
		}
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

	// Derive current state to validate the transition.
	_, priorEvents, err := h.store.CurrentSession()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	currentState := domain.DeriveState(priorEvents)

	startAction := domain.StartNight
	if kind == domain.SessionKindDay {
		startAction = domain.StartDay
	}
	nextState, err := domain.Transition(currentState, startAction, nil)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	ferberEnabled := req.Ferber != nil
	ferberNightNumber := 0
	if ferberEnabled {
		ferberNightNumber = req.Ferber.NightNumber
	}

	startEvent := &domain.Event{
		FromState: currentState,
		Action:    startAction,
		ToState:   nextState,
		Timestamp: ts,
	}
	session, err := h.store.StartNewSession(kind, ferberEnabled, ferberNightNumber, startEvent)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, h.buildSessionResponse(nextState, session, []domain.Event{*startEvent}))
}

// PostEvent records a new within-session event.
func (h *Handler) PostEvent(w http.ResponseWriter, r *http.Request) {
	var req eventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	action := domain.Action(req.Action)
	if action == domain.StartNight || action == domain.StartDay {
		writeError(w, http.StatusBadRequest, "use POST /api/session/start to create a session")
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

	session, events, err := h.store.CurrentSession()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if session == nil {
		writeError(w, http.StatusBadRequest, "no active session")
		return
	}

	currentState := domain.DeriveState(events)
	metadata := req.Metadata

	// Implicit location fill for dislatch_asleep during day feeding: the baby
	// fell asleep on the breast; location is always "on_me" in that moment.
	// (Handler-layer magic, not domain — keeps validators pure.)
	if action == domain.DislatchAsleep && currentState == domain.DayFeeding {
		if metadata == nil {
			metadata = map[string]string{}
		}
		if _, set := metadata["location"]; !set {
			metadata["location"] = string(domain.LocationOnMe)
		}
	}

	nextState, err := domain.Transition(currentState, action, metadata)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	evt := &domain.Event{
		SessionID: session.ID,
		FromState: currentState,
		Action:    action,
		ToState:   nextState,
		Timestamp: ts,
		Metadata:  metadata,
	}
	if err := h.store.AddEvent(evt); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, h.buildSessionResponse(nextState, session, append(events, *evt)))
}

// PostUndo removes the last event and returns the updated session. Chain-aware:
// undoing a start_day/start_night event that is a chain advance reopens the
// prior session.
func (h *Handler) PostUndo(w http.ResponseWriter, r *http.Request) {
	session, events, err := h.store.CurrentSession()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if session == nil {
		writeError(w, http.StatusBadRequest, "no active session to undo")
		return
	}
	if len(events) == 0 {
		writeError(w, http.StatusBadRequest, "no events to undo")
		return
	}

	lastEvent := events[len(events)-1]

	// Chain-advance undo: last event is a start_* AND there's a prior session
	// whose ended_at matches the current session's started_at.
	if lastEvent.Action == domain.StartNight || lastEvent.Action == domain.StartDay {
		prev, err := h.store.PrevSessionBefore(session.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if prev != nil && prev.EndedAt != nil && prev.EndedAt.Equal(session.StartedAt) {
			// Chain-advance undo.
			if err := h.store.UndoChainAdvance(session.ID, prev.ID); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			reopened, reopenedEvents, err := h.store.GetSession(prev.ID)
			if err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			// UndoChainAdvance cleared ended_at in the DB; reflect that
			// locally instead of refetching.
			reopened.EndedAt = nil
			reopenedState := domain.DeriveState(reopenedEvents)
			writeJSON(w, http.StatusOK, h.buildSessionResponse(reopenedState, reopened, reopenedEvents))
			return
		}
		// First-ever start: delete the lone session.
		if err := h.store.DeleteSession(session.ID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, h.buildSessionResponse(domain.NightOff, nil, nil))
		return
	}

	if _, err := h.store.PopEvent(session.ID); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	remaining := events[:len(events)-1]
	state := domain.DeriveState(remaining)
	writeJSON(w, http.StatusOK, h.buildSessionResponse(state, session, remaining))
}

// --- cycle endpoints ---

// GetCycles returns cycles (day, night pairs) with per-cycle stats and moving
// averages for trend charts. Replaces /api/nights and /api/trends.
func (h *Handler) GetCycles(w http.ResponseWriter, r *http.Request) {
	from, to := parseDateRange(r)
	sessions, err := h.store.ListSessions(from, to, "")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	ids := make([]int64, len(sessions))
	for i, s := range sessions {
		ids[i] = s.ID
	}
	eventsMap, err := h.store.GetEventsForSessions(ids)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	// Pair sessions into cycles: walk in chain order; day precedes its paired
	// night. A lone night (no preceding day) is an orphan cycle; a trailing day
	// (no following night) is in-progress "today."
	cycles := pairSessionsIntoCycles(sessions)

	const cycleTrendWindow = 3
	summaries := make([]reports.CycleSummary, len(cycles))
	for i, c := range cycles {
		var dayEvents, nightEvents []domain.Event
		if c.Day != nil {
			dayEvents = eventsMap[c.Day.ID]
		}
		if c.Night != nil {
			nightEvents = eventsMap[c.Night.ID]
		}
		summaries[i] = reports.CycleSummary{
			Day:    toCycleSessionMeta(c.Day),
			Night:  toCycleSessionMeta(c.Night),
			Events: concatCycleEvents(dayEvents, nightEvents),
			Stats:  reports.ComputeCycleStats(c.Day, c.Night, dayEvents, nightEvents),
		}
	}
	reports.AttachMovingAverages(summaries, cycleTrendWindow)

	writeJSON(w, http.StatusOK, map[string]any{
		"cycles": summaries,
		"window": cycleTrendWindow,
	})
}

type cyclePair struct {
	Day   *domain.Session
	Night *domain.Session
}

// pairSessionsIntoCycles walks a chronologically-ordered session list and pairs
// each day with the immediately-following night. Unpaired nights (no preceding
// day) are orphan cycles; unpaired days (no following night) are in-progress.
func pairSessionsIntoCycles(sessions []domain.Session) []cyclePair {
	var cycles []cyclePair
	for i := 0; i < len(sessions); i++ {
		s := &sessions[i]
		switch s.Kind {
		case domain.SessionKindDay:
			cycle := cyclePair{Day: s}
			if i+1 < len(sessions) && sessions[i+1].Kind == domain.SessionKindNight {
				cycle.Night = &sessions[i+1]
				i++ // skip the night — consumed into this pair
			}
			cycles = append(cycles, cycle)
		case domain.SessionKindNight:
			// Orphan night (no preceding day in this range).
			cycles = append(cycles, cyclePair{Night: s})
		}
	}
	return cycles
}

// concatCycleEvents returns day events followed by night events. Both slices
// arrive ordered by seq/timestamp; since day.StartedAt < night.StartedAt by
// chain construction, concatenation preserves timestamp ordering without a
// re-sort. Either slice may be empty (orphan cycles, in-progress today).
func concatCycleEvents(dayEvents, nightEvents []domain.Event) []reports.CycleEvent {
	out := make([]reports.CycleEvent, 0, len(dayEvents)+len(nightEvents))
	for _, e := range dayEvents {
		out = append(out, eventToCycleEvent(e))
	}
	for _, e := range nightEvents {
		out = append(out, eventToCycleEvent(e))
	}
	return out
}

func eventToCycleEvent(e domain.Event) reports.CycleEvent {
	return reports.CycleEvent{
		Action:    string(e.Action),
		FromState: string(e.FromState),
		ToState:   string(e.ToState),
		Metadata:  e.Metadata,
		Timestamp: e.Timestamp,
	}
}

func toCycleSessionMeta(s *domain.Session) *reports.SessionMeta {
	if s == nil {
		return nil
	}
	return &reports.SessionMeta{
		ID:                s.ID,
		Kind:              s.Kind,
		StartedAt:         s.StartedAt,
		EndedAt:           s.EndedAt,
		FerberEnabled:     s.FerberEnabled,
		FerberNightNumber: s.FerberNightNumber,
	}
}

// GetCycleDetail returns the cycle containing the given session ID, with both
// sessions' events, a combined timeline, and full cycle stats.
func (h *Handler) GetCycleDetail(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid session ID")
		return
	}

	session, events, err := h.store.GetSession(id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	if session == nil {
		writeError(w, http.StatusNotFound, "session not found")
		return
	}

	// Branch on the requested session's kind so we can reuse its already-loaded
	// events instead of refetching. Only the *other* half of the cycle (if any)
	// needs a second round-trip.
	var day, night *domain.Session
	var dayEvents, nightEvents []domain.Event
	switch session.Kind {
	case domain.SessionKindNight:
		night, nightEvents = session, events
		prev, err := h.store.PrevSessionBefore(session.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if prev != nil && prev.Kind == domain.SessionKindDay {
			if _, dayEvents, err = h.store.GetSession(prev.ID); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			day = prev
		}
	case domain.SessionKindDay:
		day, dayEvents = session, events
		next, err := h.store.NextSessionAfter(session.ID)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		if next != nil && next.Kind == domain.SessionKindNight {
			if _, nightEvents, err = h.store.GetSession(next.ID); err != nil {
				writeError(w, http.StatusInternalServerError, err.Error())
				return
			}
			night = next
		}
	}

	// Combined event log, ordered by timestamp.
	combined := make([]domain.Event, 0, len(dayEvents)+len(nightEvents))
	combined = append(combined, dayEvents...)
	combined = append(combined, nightEvents...)
	eventJSONs := make([]eventResponse, len(combined))
	for i, e := range combined {
		eventJSONs[i] = *toEventResponse(e)
	}

	// Night timeline (existing behavior) only if night session exists.
	var timeline []reports.TimelineEntry
	if night != nil {
		_, timeline = reports.ComputeStats(nightEvents, night)
	}

	// Day timeline — symmetric to night, but without the Ferber-specific
	// stats computation. BuildTimeline is kind-agnostic (events + end time
	// → segments), so we pass the day's end (or now, if still open).
	var dayTimeline []reports.TimelineEntry
	if day != nil {
		dayEnd := time.Now()
		if day.EndedAt != nil {
			dayEnd = *day.EndedAt
		}
		dayTimeline = reports.BuildTimeline(dayEvents, dayEnd)
	}

	stats := reports.ComputeCycleStats(day, night, dayEvents, nightEvents)

	writeJSON(w, http.StatusOK, map[string]any{
		"cycle": map[string]any{
			"day":   toCycleSessionMeta(day),
			"night": toCycleSessionMeta(night),
		},
		"events":      eventJSONs,
		"timeline":    timeline,    // night segments (preserved name for the night block)
		"dayTimeline": dayTimeline, // day segments
		"stats":       stats,
	})
}

// --- helpers ---

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
	cw.Write([]string{"session_id", "seq", "from_state", "action", "to_state", "timestamp", "breast", "metadata"})

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
			strconv.FormatInt(e.SessionID, 10),
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
