package api

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/polina/boob-o-clock/internal/domain"
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

// GetCurrentSession returns the current state and valid actions.
func (h *Handler) GetCurrentSession(w http.ResponseWriter, r *http.Request) {
	night, events, err := h.store.CurrentSession()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	state := domain.DeriveState(events)
	resp := sessionResponse{
		State:        state,
		ValidActions: domain.ValidActions(state),
	}
	if night != nil {
		resp.NightID = &night.ID
	}
	if len(events) > 0 {
		last := events[len(events)-1]
		resp.LastEvent = &eventResponse{
			Action:    last.Action,
			FromState: last.FromState,
			ToState:   last.ToState,
			Metadata:  last.Metadata,
			Timestamp: last.Timestamp,
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// PostEvent records a new event and returns the updated session.
func (h *Handler) PostEvent(w http.ResponseWriter, r *http.Request) {
	var req eventRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	action := domain.Action(req.Action)

	// Parse timestamp (default to now)
	ts := time.Now()
	if req.Timestamp != "" {
		parsed, err := time.Parse(time.RFC3339, req.Timestamp)
		if err != nil {
			writeError(w, http.StatusBadRequest, "invalid timestamp format (use RFC3339)")
			return
		}
		ts = parsed
	}

	// Load current session
	night, events, err := h.store.CurrentSession()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	currentState := domain.DeriveState(events)

	// Validate transition
	nextState, err := domain.Transition(currentState, action, req.Metadata)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	// Handle night lifecycle
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

	// Store event
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

	// End night if applicable
	if action == domain.EndNight {
		if err := h.store.EndNight(night.ID, ts); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
	}

	// Return updated session
	resp := sessionResponse{
		State:        nextState,
		ValidActions: domain.ValidActions(nextState),
		LastEvent: &eventResponse{
			Action:    evt.Action,
			FromState: evt.FromState,
			ToState:   evt.ToState,
			Metadata:  evt.Metadata,
			Timestamp: evt.Timestamp,
		},
	}
	if nextState != domain.NightOff {
		resp.NightID = &night.ID
	}

	writeJSON(w, http.StatusOK, resp)
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

	// If we undid start_night, delete the night entirely
	if popped.Action == domain.StartNight {
		if err := h.store.DeleteNight(night.ID); err != nil {
			writeError(w, http.StatusInternalServerError, err.Error())
			return
		}
		resp := sessionResponse{
			State:        domain.NightOff,
			ValidActions: domain.ValidActions(domain.NightOff),
		}
		writeJSON(w, http.StatusOK, resp)
		return
	}

	// Reload events after pop
	_, events, err = h.store.CurrentSession()
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	state := domain.DeriveState(events)
	resp := sessionResponse{
		State:        state,
		ValidActions: domain.ValidActions(state),
		NightID:      &night.ID,
	}
	if len(events) > 0 {
		last := events[len(events)-1]
		resp.LastEvent = &eventResponse{
			Action:    last.Action,
			FromState: last.FromState,
			ToState:   last.ToState,
			Metadata:  last.Metadata,
			Timestamp: last.Timestamp,
		}
	}

	writeJSON(w, http.StatusOK, resp)
}

// GetNightDetail returns a night and its events.
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

	type nightJSON struct {
		ID        int64      `json:"id"`
		StartedAt time.Time  `json:"startedAt"`
		EndedAt   *time.Time `json:"endedAt"`
	}

	eventJSONs := make([]eventResponse, len(events))
	for i, e := range events {
		eventJSONs[i] = eventResponse{
			Action:    e.Action,
			FromState: e.FromState,
			ToState:   e.ToState,
			Metadata:  e.Metadata,
			Timestamp: e.Timestamp,
		}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"night": nightJSON{
			ID:        night.ID,
			StartedAt: night.StartedAt,
			EndedAt:   night.EndedAt,
		},
		"events": eventJSONs,
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

