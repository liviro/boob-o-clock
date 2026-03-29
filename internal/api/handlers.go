package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/polina/boob-o-clock/internal/domain"
	"github.com/polina/boob-o-clock/internal/reports"
	"github.com/polina/boob-o-clock/internal/store"
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

type sessionResponse struct {
	State        domain.State    `json:"state"`
	ValidActions []domain.Action `json:"validActions"`
	NightID      *int64          `json:"nightId"`
	LastEvent    *eventResponse  `json:"lastEvent"`
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

func buildSessionResponse(state domain.State, nightID *int64, events []domain.Event) sessionResponse {
	resp := sessionResponse{
		State:        state,
		ValidActions: domain.ValidActions(state),
		NightID:      nightID,
	}
	if len(events) > 0 {
		resp.LastEvent = toEventResponse(events[len(events)-1])
	}
	return resp
}

func nightEndTime(n *domain.Night) time.Time {
	if n.EndedAt != nil {
		return *n.EndedAt
	}
	return time.Now()
}

// GetCurrentSession returns the current state and valid actions.
func (h *Handler) GetCurrentSession(w http.ResponseWriter, r *http.Request) {
	night, events, err := h.store.CurrentSession()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	state := domain.DeriveState(events)
	var nightID *int64
	if night != nil {
		nightID = &night.ID
	}

	writeJSON(w, http.StatusOK, buildSessionResponse(state, nightID, events))
}

// PostEvent records a new event and returns the updated session.
func (h *Handler) PostEvent(w http.ResponseWriter, r *http.Request) {
	var req eventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	action := domain.Action(req.Action)

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

	currentState := domain.DeriveState(events)

	nextState, err := domain.Transition(currentState, action, req.Metadata)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	if action == domain.StartNight {
		night, err = h.store.CreateNight(ts)
		if err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	if night == nil {
		writeError(w, http.StatusBadRequest, "no active night session")
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

	var nightID *int64
	if nextState != domain.NightOff {
		nightID = &night.ID
	}

	writeJSON(w, http.StatusOK, buildSessionResponse(nextState, nightID, append(events, *evt)))
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

	popped, err := h.store.PopEvent(night.ID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	if popped.Action == domain.StartNight {
		if err := h.store.DeleteNight(night.ID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, buildSessionResponse(domain.NightOff, nil, nil))
		return
	}

	// Derive state from events minus the popped one
	remaining := events[:len(events)-1]
	state := domain.DeriveState(remaining)
	writeJSON(w, http.StatusOK, buildSessionResponse(state, &night.ID, remaining))
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

	nightEnd := nightEndTime(night)
	timeline := reports.BuildTimeline(events)
	stats := reports.ComputeStats(events, night.StartedAt, nightEnd)

	writeJSON(w, http.StatusOK, map[string]any{
		"night": nightDetailJSON{
			ID:        night.ID,
			StartedAt: night.StartedAt,
			EndedAt:   night.EndedAt,
		},
		"events":   eventJSONs,
		"timeline": timeline,
		"stats":    stats,
	})
}

// GetNights returns a list of nights with summary stats.
func (h *Handler) GetNights(w http.ResponseWriter, r *http.Request) {
	to := time.Now().Add(24 * time.Hour)
	from := to.Add(-30 * 24 * time.Hour)

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

	nights, err := h.store.ListNights(from, to)
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
		// Use GetEvents directly to avoid re-fetching the night row (N+1 fix)
		events, err := h.store.GetEvents(n.ID)
		if err != nil {
			continue
		}

		stats := reports.ComputeStats(events, n.StartedAt, nightEndTime(&n))
		summaries = append(summaries, nightSummary{
			nightDetailJSON: nightDetailJSON{
				ID:        n.ID,
				StartedAt: n.StartedAt,
				EndedAt:   n.EndedAt,
			},
			Stats: stats,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"nights": summaries,
	})
}

type nightDetailJSON struct {
	ID        int64      `json:"id"`
	StartedAt time.Time  `json:"startedAt"`
	EndedAt   *time.Time `json:"endedAt,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}
