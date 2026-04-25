package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/liviro/boob-o-clock/internal/domain"
	"github.com/liviro/boob-o-clock/internal/store"
)

// --- test helpers ---

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	ts, _ := newTestServerWithStore(t)
	return ts
}

func newTestServerWithStore(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	return newTestServerWithConfig(t, Config{FerberEnabled: true, ChairEnabled: true})
}

func newTestServerWithConfig(t *testing.T, cfg Config) (*httptest.Server, *store.Store) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	handler := NewHandler(s, cfg)
	router := NewRouter(handler)
	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)
	return ts, s
}

// makeNightAt creates and immediately closes a night session at startedAt.
// Used to seed historical data without routing through the API.
func makeNightAt(t *testing.T, s *store.Store, startedAt time.Time) int64 {
	t.Helper()
	n, err := s.CreateSession(domain.SessionKindNight, startedAt, false, 0, false)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := s.EndSession(n.ID, startedAt.Add(time.Hour)); err != nil {
		t.Fatalf("EndSession: %v", err)
	}
	return n.ID
}

func doGet(t *testing.T, ts *httptest.Server, path string) *http.Response {
	t.Helper()
	resp, err := http.Get(ts.URL + path)
	if err != nil {
		t.Fatalf("GET %s: %v", path, err)
	}
	return resp
}

func doPost(t *testing.T, ts *httptest.Server, path string, body any) *http.Response {
	t.Helper()
	b, _ := json.Marshal(body)
	resp, err := http.Post(ts.URL+path, "application/json", bytes.NewReader(b))
	if err != nil {
		t.Fatalf("POST %s: %v", path, err)
	}
	return resp
}

func decodeJSON(t *testing.T, resp *http.Response, v any) {
	t.Helper()
	defer resp.Body.Close()
	if err := json.NewDecoder(resp.Body).Decode(v); err != nil {
		t.Fatalf("decode response: %v", err)
	}
}

// startNight is a convenience wrapper for the common "start a plain night" call.
func startNight(t *testing.T, ts *httptest.Server) *http.Response {
	t.Helper()
	return doPost(t, ts, "/api/session/start", map[string]any{"kind": "night"})
}

// startDay is a convenience wrapper for "start a day session."
func startDay(t *testing.T, ts *httptest.Server) *http.Response {
	t.Helper()
	return doPost(t, ts, "/api/session/start", map[string]any{"kind": "day"})
}

// SessionResponseJSON mirrors the handler's sessionResponse wire shape.
type SessionResponseJSON struct {
	Kind              *string  `json:"kind"`
	State             string   `json:"state"`
	ValidActions      []string `json:"validActions"`
	SessionID         *int64   `json:"sessionId"`
	SuggestBreast     string   `json:"suggestBreast"`
	CurrentBreast     string   `json:"currentBreast"`
	LastFeedStartedAt string   `json:"lastFeedStartedAt"`
	LastEvent         *struct {
		Action    string            `json:"action"`
		FromState string            `json:"fromState"`
		ToState   string            `json:"toState"`
		Metadata  map[string]string `json:"metadata"`
	} `json:"lastEvent"`
	Ferber *struct {
		NightNumber int `json:"nightNumber"`
		Current     *struct {
			CheckInCount       int    `json:"checkInCount"`
			StartedAt          string `json:"startedAt"`
			CheckInAvailableAt string `json:"checkInAvailableAt,omitempty"`
			Mood               string `json:"mood"`
		} `json:"current,omitempty"`
	} `json:"ferber,omitempty"`
	SuggestFerberNight *int `json:"suggestFerberNight,omitempty"`
	ChairEnabled       bool `json:"chairEnabled,omitempty"`
	SuggestChair       bool `json:"suggestChair,omitempty"`
}

// --- GET /api/session/current ---

func TestGetCurrentSessionNoActive(t *testing.T) {
	ts := newTestServer(t)

	resp := doGet(t, ts, "/api/session/current")
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var sr SessionResponseJSON
	decodeJSON(t, resp, &sr)

	if sr.State != "night_off" {
		t.Errorf("state = %s, want night_off", sr.State)
	}
	// With no prior session, first-start offers both night and day.
	gotActions := map[string]bool{}
	for _, a := range sr.ValidActions {
		gotActions[a] = true
	}
	if !gotActions["start_night"] || !gotActions["start_day"] {
		t.Errorf("validActions = %v, want to include both start_night and start_day", sr.ValidActions)
	}
	if sr.Kind != nil {
		t.Errorf("kind = %v, want nil on night_off", *sr.Kind)
	}
	if sr.SessionID != nil {
		t.Errorf("sessionId = %v, want nil", sr.SessionID)
	}
}

// --- POST /api/session/start ---

func TestStartSessionNight(t *testing.T) {
	ts := newTestServer(t)

	resp := startNight(t, ts)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var sr SessionResponseJSON
	decodeJSON(t, resp, &sr)

	if sr.State != "awake" {
		t.Errorf("state = %s, want awake", sr.State)
	}
	if sr.Kind == nil || *sr.Kind != "night" {
		t.Errorf("kind = %v, want night", sr.Kind)
	}
	if sr.SessionID == nil {
		t.Fatal("expected sessionId")
	}
}

func TestStartSessionDay(t *testing.T) {
	ts := newTestServer(t)

	resp := startDay(t, ts)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var sr SessionResponseJSON
	decodeJSON(t, resp, &sr)

	if sr.State != "day_awake" {
		t.Errorf("state = %s, want day_awake", sr.State)
	}
	if sr.Kind == nil || *sr.Kind != "day" {
		t.Errorf("kind = %v, want day", sr.Kind)
	}
}

func TestStartSessionRejectsFerberWithDay(t *testing.T) {
	ts := newTestServer(t)

	resp := doPost(t, ts, "/api/session/start", map[string]any{
		"kind":   "day",
		"ferber": map[string]any{"nightNumber": 1},
	})
	if resp.StatusCode != 400 {
		t.Errorf("status = %d, want 400 when ferber present on day", resp.StatusCode)
	}
}

func TestStartSessionRejectsBadKind(t *testing.T) {
	ts := newTestServer(t)

	resp := doPost(t, ts, "/api/session/start", map[string]any{"kind": "afternoon"})
	if resp.StatusCode != 400 {
		t.Errorf("status = %d, want 400 for unknown kind", resp.StatusCode)
	}
}

func TestStartSessionChainAdvance(t *testing.T) {
	ts := newTestServer(t)

	// Start night, then start day (chain advance). The old night must close.
	startNight(t, ts)
	resp := startDay(t, ts)
	if resp.StatusCode != 200 {
		t.Fatalf("chain advance status = %d, want 200", resp.StatusCode)
	}

	var sr SessionResponseJSON
	decodeJSON(t, resp, &sr)
	if sr.State != "day_awake" {
		t.Errorf("state = %s, want day_awake", sr.State)
	}
	if *sr.Kind != "day" {
		t.Errorf("kind = %s, want day after chain advance", *sr.Kind)
	}

	// Current session should now be the day session; only one is open.
	resp = doGet(t, ts, "/api/session/current")
	decodeJSON(t, resp, &sr)
	if *sr.Kind != "day" {
		t.Errorf("current kind = %s, want day", *sr.Kind)
	}
}

func TestStartSessionWithTimestamp(t *testing.T) {
	ts := newTestServer(t)

	body := map[string]any{
		"kind":      "night",
		"timestamp": "2026-03-29T03:00:00-07:00",
	}
	resp := doPost(t, ts, "/api/session/start", body)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

// --- POST /api/session/event ---

func TestPostEventRejectsStartActions(t *testing.T) {
	ts := newTestServer(t)

	for _, action := range []string{"start_night", "start_day"} {
		resp := doPost(t, ts, "/api/session/event", map[string]any{"action": action})
		if resp.StatusCode != 400 {
			t.Errorf("POST /event {action: %s} status = %d, want 400", action, resp.StatusCode)
		}
		resp.Body.Close()
	}
}

func TestPostEventInvalidTransition(t *testing.T) {
	ts := newTestServer(t)

	body := map[string]any{"action": "start_feed", "metadata": map[string]string{"breast": "L"}}
	resp := doPost(t, ts, "/api/session/event", body)
	if resp.StatusCode != 400 {
		t.Fatalf("status = %d, want 400 (no active session)", resp.StatusCode)
	}
}

func TestPostEventFeedRequiresBreast(t *testing.T) {
	ts := newTestServer(t)

	startNight(t, ts)
	resp := doPost(t, ts, "/api/session/event", map[string]any{"action": "start_feed"})
	if resp.StatusCode != 400 {
		t.Fatalf("status = %d, want 400 for feed without breast", resp.StatusCode)
	}
}

func TestPostEventStartSleepRequiresLocation(t *testing.T) {
	ts := newTestServer(t)

	startDay(t, ts)
	resp := doPost(t, ts, "/api/session/event", map[string]any{"action": "start_sleep"})
	if resp.StatusCode != 400 {
		t.Errorf("status = %d, want 400 for start_sleep without location", resp.StatusCode)
	}
}

func TestPostEventStartSleepWithLocation(t *testing.T) {
	ts := newTestServer(t)

	startDay(t, ts)
	resp := doPost(t, ts, "/api/session/event", map[string]any{
		"action":   "start_sleep",
		"metadata": map[string]string{"location": "crib"},
	})
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	var sr SessionResponseJSON
	decodeJSON(t, resp, &sr)
	if sr.State != "day_sleeping" {
		t.Errorf("state = %s, want day_sleeping", sr.State)
	}
}

// TestDayDislatchAsleepImplicitLocation verifies the handler-layer magic: when
// a user taps "dislatch asleep" during a day feed, the handler fills
// location=on_me so the nap state gets its required metadata.
func TestDayDislatchAsleepImplicitLocation(t *testing.T) {
	ts := newTestServer(t)

	startDay(t, ts)
	doPost(t, ts, "/api/session/event", map[string]any{
		"action":   "start_feed",
		"metadata": map[string]string{"breast": "L"},
	})
	// No location metadata in the request — handler should fill it.
	resp := doPost(t, ts, "/api/session/event", map[string]any{"action": "dislatch_asleep"})
	if resp.StatusCode != 200 {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, want 200 (body: %s)", resp.StatusCode, body)
	}
	var sr SessionResponseJSON
	decodeJSON(t, resp, &sr)
	if sr.State != "day_sleeping" {
		t.Errorf("state = %s, want day_sleeping", sr.State)
	}
	if sr.LastEvent == nil || sr.LastEvent.Metadata["location"] != "on_me" {
		t.Errorf("last event location = %v, want on_me (implicit fill)", sr.LastEvent)
	}
}

// --- full-flow tests ---

func TestFullNightRoundTrip(t *testing.T) {
	ts := newTestServer(t)

	steps := []struct {
		path      string
		action    string
		kind      string
		metadata  map[string]string
		wantState string
	}{
		{"/api/session/start", "", "night", nil, "awake"},
		{"/api/session/event", "start_feed", "", map[string]string{"breast": "L"}, "feeding"},
		{"/api/session/event", "dislatch_asleep", "", nil, "sleeping_on_me"},
		{"/api/session/event", "start_transfer", "", nil, "transferring"},
		{"/api/session/event", "transfer_success", "", nil, "sleeping_crib"},
		{"/api/session/event", "baby_woke", "", nil, "awake"},
		{"/api/session/event", "start_feed", "", map[string]string{"breast": "R"}, "feeding"},
		{"/api/session/event", "dislatch_awake", "", nil, "awake"},
		// Morning: chain advance instead of end_night.
		{"/api/session/start", "", "day", nil, "day_awake"},
	}

	for _, step := range steps {
		body := map[string]any{}
		if step.kind != "" {
			body["kind"] = step.kind
		} else {
			body["action"] = step.action
		}
		if step.metadata != nil {
			body["metadata"] = step.metadata
		}
		resp := doPost(t, ts, step.path, body)
		if resp.StatusCode != 200 {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("step %s %s: status = %d, body = %s", step.path, step.action+step.kind, resp.StatusCode, b)
		}

		var sr SessionResponseJSON
		decodeJSON(t, resp, &sr)

		if sr.State != step.wantState {
			t.Errorf("step %s%s: state = %s, want %s", step.action, step.kind, sr.State, step.wantState)
		}
	}
}

func TestFullDayRoundTrip(t *testing.T) {
	ts := newTestServer(t)

	// Start the chain in day mode. Run through nap + feed + poop.
	startDay(t, ts)

	// Nap
	resp := doPost(t, ts, "/api/session/event", map[string]any{
		"action":   "start_sleep",
		"metadata": map[string]string{"location": "crib"},
	})
	if resp.StatusCode != 200 {
		t.Fatalf("start_sleep: %d", resp.StatusCode)
	}
	resp = doPost(t, ts, "/api/session/event", map[string]any{"action": "baby_woke"})
	if resp.StatusCode != 200 {
		t.Fatalf("baby_woke: %d", resp.StatusCode)
	}
	// Feed
	resp = doPost(t, ts, "/api/session/event", map[string]any{
		"action":   "start_feed",
		"metadata": map[string]string{"breast": "L"},
	})
	if resp.StatusCode != 200 {
		t.Fatalf("start_feed: %d", resp.StatusCode)
	}
	resp = doPost(t, ts, "/api/session/event", map[string]any{"action": "dislatch_awake"})
	if resp.StatusCode != 200 {
		t.Fatalf("dislatch_awake: %d", resp.StatusCode)
	}
	// Poop
	resp = doPost(t, ts, "/api/session/event", map[string]any{"action": "poop_start"})
	if resp.StatusCode != 200 {
		t.Fatalf("poop_start: %d", resp.StatusCode)
	}
	resp = doPost(t, ts, "/api/session/event", map[string]any{"action": "poop_done"})
	if resp.StatusCode != 200 {
		t.Fatalf("poop_done: %d", resp.StatusCode)
	}

	// End the day with chain advance to night.
	resp = doPost(t, ts, "/api/session/start", map[string]any{"kind": "night"})
	if resp.StatusCode != 200 {
		t.Fatalf("chain advance to night: %d", resp.StatusCode)
	}
	var sr SessionResponseJSON
	decodeJSON(t, resp, &sr)
	if sr.State != "awake" || *sr.Kind != "night" {
		t.Errorf("after chain to night: state=%s kind=%v, want awake/night", sr.State, sr.Kind)
	}
}

// --- POST /api/session/undo ---

func TestUndoPopsLastEvent(t *testing.T) {
	ts := newTestServer(t)

	startNight(t, ts)
	doPost(t, ts, "/api/session/event", map[string]any{
		"action":   "start_feed",
		"metadata": map[string]string{"breast": "L"},
	})

	resp := doPost(t, ts, "/api/session/undo", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("undo status = %d, want 200", resp.StatusCode)
	}
	var sr SessionResponseJSON
	decodeJSON(t, resp, &sr)
	if sr.State != "awake" {
		t.Errorf("after undo: state = %s, want awake", sr.State)
	}
}

func TestUndoFirstStartDeletesSession(t *testing.T) {
	ts := newTestServer(t)

	startNight(t, ts)
	resp := doPost(t, ts, "/api/session/undo", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("undo status = %d, want 200", resp.StatusCode)
	}

	var sr SessionResponseJSON
	decodeJSON(t, resp, &sr)
	if sr.State != "night_off" {
		t.Errorf("after undo first-start: state = %s, want night_off", sr.State)
	}

	resp = doGet(t, ts, "/api/session/current")
	decodeJSON(t, resp, &sr)
	if sr.SessionID != nil {
		t.Error("expected nil sessionId after undoing first-start")
	}
}

func TestUndoChainAdvanceNightToDay(t *testing.T) {
	ts := newTestServer(t)

	// Start night, go through some events, then chain advance to day.
	startNight(t, ts)
	doPost(t, ts, "/api/session/event", map[string]any{
		"action":   "start_feed",
		"metadata": map[string]string{"breast": "L"},
	})
	doPost(t, ts, "/api/session/event", map[string]any{"action": "dislatch_awake"})
	startDay(t, ts) // chain advance

	// Undo the chain advance — prior night should reopen at Awake.
	resp := doPost(t, ts, "/api/session/undo", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("undo status = %d, want 200", resp.StatusCode)
	}

	var sr SessionResponseJSON
	decodeJSON(t, resp, &sr)
	if sr.State != "awake" {
		t.Errorf("after undo chain: state = %s, want awake (night reopened)", sr.State)
	}
	if sr.Kind == nil || *sr.Kind != "night" {
		t.Errorf("kind = %v, want night after chain undo", sr.Kind)
	}

	// Confirm current session also reflects the reopened night.
	resp = doGet(t, ts, "/api/session/current")
	decodeJSON(t, resp, &sr)
	if *sr.Kind != "night" || sr.State != "awake" {
		t.Errorf("current after chain undo: kind=%v state=%s", sr.Kind, sr.State)
	}
}

func TestUndoChainAdvanceDayToNight(t *testing.T) {
	ts := newTestServer(t)

	startDay(t, ts)
	doPost(t, ts, "/api/session/start", map[string]any{"kind": "night"}) // chain advance

	resp := doPost(t, ts, "/api/session/undo", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("undo status = %d", resp.StatusCode)
	}

	var sr SessionResponseJSON
	decodeJSON(t, resp, &sr)
	if sr.State != "day_awake" {
		t.Errorf("after undo chain day-to-night: state = %s, want day_awake", sr.State)
	}
	if *sr.Kind != "day" {
		t.Errorf("kind = %s, want day", *sr.Kind)
	}
}

func TestUndoNoSession(t *testing.T) {
	ts := newTestServer(t)

	resp := doPost(t, ts, "/api/session/undo", nil)
	if resp.StatusCode != 400 {
		t.Fatalf("undo with no session: status = %d, want 400", resp.StatusCode)
	}
}

// --- GET /api/cycles ---

func TestGetCyclesEmpty(t *testing.T) {
	ts := newTestServer(t)

	resp := doGet(t, ts, "/api/cycles")
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var result struct {
		Cycles []any `json:"cycles"`
		Window int   `json:"window"`
	}
	decodeJSON(t, resp, &result)

	if len(result.Cycles) != 0 {
		t.Errorf("got %d cycles, want 0", len(result.Cycles))
	}
	if result.Window != 3 {
		t.Errorf("window = %d, want 3", result.Window)
	}
}

func TestGetCyclesWithSessions(t *testing.T) {
	ts, s := newTestServerWithStore(t)

	now := time.Now()
	// Seed a day+night pair (one complete cycle).
	d, _ := s.CreateSession(domain.SessionKindDay, now.Add(-16*time.Hour), false, 0, false)
	_ = s.EndSession(d.ID, now.Add(-8*time.Hour))
	n, _ := s.CreateSession(domain.SessionKindNight, now.Add(-8*time.Hour), false, 0, false)
	_ = s.EndSession(n.ID, now.Add(-time.Hour))

	resp := doGet(t, ts, "/api/cycles")
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var result struct {
		Cycles []struct {
			Day   *struct{ ID int64 `json:"id"` } `json:"day"`
			Night *struct{ ID int64 `json:"id"` } `json:"night"`
		} `json:"cycles"`
	}
	decodeJSON(t, resp, &result)

	if len(result.Cycles) != 1 {
		t.Fatalf("got %d cycles, want 1", len(result.Cycles))
	}
	if result.Cycles[0].Day == nil || result.Cycles[0].Day.ID != d.ID {
		t.Errorf("cycle day = %+v, want id %d", result.Cycles[0].Day, d.ID)
	}
	if result.Cycles[0].Night == nil || result.Cycles[0].Night.ID != n.ID {
		t.Errorf("cycle night = %+v, want id %d", result.Cycles[0].Night, n.ID)
	}
}

func TestGetCyclesOrphanHistoricalNight(t *testing.T) {
	ts, s := newTestServerWithStore(t)

	// Seed a night with no preceding day (historical pre-feature case).
	makeNightAt(t, s, time.Now().Add(-24*time.Hour))

	resp := doGet(t, ts, "/api/cycles")
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	var result struct {
		Cycles []struct {
			Day   any `json:"day"`
			Night *struct{ ID int64 `json:"id"` } `json:"night"`
		} `json:"cycles"`
	}
	decodeJSON(t, resp, &result)

	if len(result.Cycles) != 1 {
		t.Fatalf("got %d cycles, want 1", len(result.Cycles))
	}
	if result.Cycles[0].Day != nil {
		t.Errorf("orphan cycle: day = %v, want nil", result.Cycles[0].Day)
	}
	if result.Cycles[0].Night == nil {
		t.Fatal("orphan cycle: night = nil, want populated")
	}
}

// TestGetCyclesIncludesEventsInline verifies that each CycleSummary carries
// its events (day + night, timestamp-ordered) in the response. Enables the
// stacked 24h timeline view to render without per-cycle fetches.
func TestGetCyclesIncludesEventsInline(t *testing.T) {
	ts, s := newTestServerWithStore(t)

	// Create a night with two events.
	n, _ := s.CreateSession(domain.SessionKindNight, time.Now().Add(-24*time.Hour), false, 0, false)
	_ = s.AddEvent(&domain.Event{
		SessionID: n.ID, FromState: domain.NightOff,
		Action: domain.StartNight, ToState: domain.Awake,
		Timestamp: time.Now().Add(-24 * time.Hour),
	})
	_ = s.AddEvent(&domain.Event{
		SessionID: n.ID, FromState: domain.Awake,
		Action: domain.StartFeed, ToState: domain.Feeding,
		Timestamp: time.Now().Add(-23 * time.Hour),
		Metadata:  map[string]string{"breast": "L"},
	})
	_ = s.EndSession(n.ID, time.Now().Add(-16*time.Hour))

	resp := doGet(t, ts, "/api/cycles")
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	var result struct {
		Cycles []struct {
			Events []struct {
				Action    string            `json:"action"`
				FromState string            `json:"fromState"`
				ToState   string            `json:"toState"`
				Metadata  map[string]string `json:"metadata"`
				Timestamp string            `json:"timestamp"`
			} `json:"events"`
		} `json:"cycles"`
	}
	decodeJSON(t, resp, &result)

	if len(result.Cycles) != 1 {
		t.Fatalf("got %d cycles, want 1", len(result.Cycles))
	}
	if len(result.Cycles[0].Events) != 2 {
		t.Fatalf("cycle events = %d, want 2", len(result.Cycles[0].Events))
	}
	if result.Cycles[0].Events[0].Action != "start_night" {
		t.Errorf("first event action = %s, want start_night", result.Cycles[0].Events[0].Action)
	}
	if result.Cycles[0].Events[1].Action != "start_feed" || result.Cycles[0].Events[1].Metadata["breast"] != "L" {
		t.Errorf("second event = %+v, want start_feed with breast=L", result.Cycles[0].Events[1])
	}
}

// TestGetCyclesDayEventsPrecedeNight verifies that for a full cycle
// (day + night), events are emitted in timestamp order — day first.
func TestGetCyclesDayEventsPrecedeNight(t *testing.T) {
	ts, s := newTestServerWithStore(t)

	now := time.Now()
	// Day session.
	d, _ := s.CreateSession(domain.SessionKindDay, now.Add(-18*time.Hour), false, 0, false)
	_ = s.AddEvent(&domain.Event{
		SessionID: d.ID, FromState: domain.NightOff,
		Action: domain.StartDay, ToState: domain.DayAwake,
		Timestamp: now.Add(-18 * time.Hour),
	})
	_ = s.EndSession(d.ID, now.Add(-10*time.Hour))
	// Night session.
	n, _ := s.CreateSession(domain.SessionKindNight, now.Add(-10*time.Hour), false, 0, false)
	_ = s.AddEvent(&domain.Event{
		SessionID: n.ID, FromState: domain.DayAwake,
		Action: domain.StartNight, ToState: domain.Awake,
		Timestamp: now.Add(-10 * time.Hour),
	})
	_ = s.EndSession(n.ID, now.Add(-2*time.Hour))

	resp := doGet(t, ts, "/api/cycles")
	var result struct {
		Cycles []struct {
			Events []struct {
				Action    string `json:"action"`
				Timestamp string `json:"timestamp"`
			} `json:"events"`
		} `json:"cycles"`
	}
	decodeJSON(t, resp, &result)

	if len(result.Cycles) != 1 {
		t.Fatalf("got %d cycles, want 1 (paired)", len(result.Cycles))
	}
	evts := result.Cycles[0].Events
	if len(evts) != 2 {
		t.Fatalf("events = %d, want 2", len(evts))
	}
	if evts[0].Action != "start_day" || evts[1].Action != "start_night" {
		t.Errorf("events ordered as %v, want [start_day, start_night]", []string{evts[0].Action, evts[1].Action})
	}
}

func TestGetCyclesHonorsExplicitRange(t *testing.T) {
	ts, s := newTestServerWithStore(t)

	now := time.Now()
	makeNightAt(t, s, now.Add(-5*24*time.Hour))
	insideID := makeNightAt(t, s, now.Add(-20*24*time.Hour))
	makeNightAt(t, s, now.Add(-45*24*time.Hour))

	from := now.Add(-30 * 24 * time.Hour).Format("2006-01-02")
	to := now.Add(-10 * 24 * time.Hour).Format("2006-01-02")
	resp := doGet(t, ts, "/api/cycles?from="+from+"&to="+to)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	var result struct {
		Cycles []struct {
			Night *struct{ ID int64 `json:"id"` } `json:"night"`
		} `json:"cycles"`
	}
	decodeJSON(t, resp, &result)

	if len(result.Cycles) != 1 {
		t.Fatalf("got %d cycles, want 1 (bracketed range)", len(result.Cycles))
	}
	if result.Cycles[0].Night.ID != insideID {
		t.Errorf("got night id %d, want %d", result.Cycles[0].Night.ID, insideID)
	}
}

// TestGetCyclesIncludesOldSessions confirms the default 90-day window.
func TestGetCyclesIncludesOldSessions(t *testing.T) {
	ts, s := newTestServerWithStore(t)

	now := time.Now()
	recentID := makeNightAt(t, s, now.Add(-60*24*time.Hour))
	oldID := makeNightAt(t, s, now.Add(-100*24*time.Hour))

	resp := doGet(t, ts, "/api/cycles")
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	var result struct {
		Cycles []struct {
			Night *struct{ ID int64 `json:"id"` } `json:"night"`
		} `json:"cycles"`
	}
	decodeJSON(t, resp, &result)

	ids := map[int64]bool{}
	for _, c := range result.Cycles {
		if c.Night != nil {
			ids[c.Night.ID] = true
		}
	}
	if !ids[recentID] {
		t.Errorf("expected 60-day-old night (id=%d) in default window", recentID)
	}
	if ids[oldID] {
		t.Errorf("did not expect 100-day-old night (id=%d) in default window", oldID)
	}
}

// --- GET /api/cycles/{id} ---

func TestGetCycleDetailByNightId(t *testing.T) {
	ts, s := newTestServerWithStore(t)

	// Day + night pair; request via night id.
	d, _ := s.CreateSession(domain.SessionKindDay, time.Now().Add(-16*time.Hour), false, 0, false)
	_ = s.EndSession(d.ID, time.Now().Add(-8*time.Hour))
	n, _ := s.CreateSession(domain.SessionKindNight, time.Now().Add(-8*time.Hour), false, 0, false)
	_ = s.EndSession(n.ID, time.Now().Add(-time.Hour))

	resp := doGet(t, ts, fmt.Sprintf("/api/cycles/%d", n.ID))
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var detail struct {
		Cycle struct {
			Day   *struct{ ID int64 `json:"id"` } `json:"day"`
			Night *struct{ ID int64 `json:"id"` } `json:"night"`
		} `json:"cycle"`
	}
	decodeJSON(t, resp, &detail)

	if detail.Cycle.Day == nil || detail.Cycle.Day.ID != d.ID {
		t.Errorf("cycle.day = %+v, want id %d", detail.Cycle.Day, d.ID)
	}
	if detail.Cycle.Night == nil || detail.Cycle.Night.ID != n.ID {
		t.Errorf("cycle.night = %+v, want id %d", detail.Cycle.Night, n.ID)
	}
}

func TestGetCycleDetailByDayId(t *testing.T) {
	ts, s := newTestServerWithStore(t)

	d, _ := s.CreateSession(domain.SessionKindDay, time.Now().Add(-16*time.Hour), false, 0, false)
	_ = s.EndSession(d.ID, time.Now().Add(-8*time.Hour))
	n, _ := s.CreateSession(domain.SessionKindNight, time.Now().Add(-8*time.Hour), false, 0, false)
	_ = s.EndSession(n.ID, time.Now().Add(-time.Hour))

	resp := doGet(t, ts, fmt.Sprintf("/api/cycles/%d", d.ID))
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	var detail struct {
		Cycle struct {
			Day   *struct{ ID int64 `json:"id"` } `json:"day"`
			Night *struct{ ID int64 `json:"id"` } `json:"night"`
		} `json:"cycle"`
	}
	decodeJSON(t, resp, &detail)
	if detail.Cycle.Day.ID != d.ID || detail.Cycle.Night.ID != n.ID {
		t.Errorf("passing day id should return full pair; got day=%+v night=%+v", detail.Cycle.Day, detail.Cycle.Night)
	}
}

func TestGetCycleDetailOrphanHistoricalNight(t *testing.T) {
	ts, s := newTestServerWithStore(t)

	// Two consecutive nights (historical pre-feature stream). The second's
	// "prev" is another night, not a day — day stays nil.
	n1, _ := s.CreateSession(domain.SessionKindNight, time.Now().Add(-48*time.Hour), false, 0, false)
	_ = s.EndSession(n1.ID, time.Now().Add(-40*time.Hour))
	n2, _ := s.CreateSession(domain.SessionKindNight, time.Now().Add(-24*time.Hour), false, 0, false)
	_ = s.EndSession(n2.ID, time.Now().Add(-16*time.Hour))

	resp := doGet(t, ts, fmt.Sprintf("/api/cycles/%d", n2.ID))
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	var detail struct {
		Cycle struct {
			Day   any `json:"day"`
			Night *struct{ ID int64 `json:"id"` } `json:"night"`
		} `json:"cycle"`
	}
	decodeJSON(t, resp, &detail)

	if detail.Cycle.Day != nil {
		t.Errorf("orphan cycle: day = %v, want nil", detail.Cycle.Day)
	}
	if detail.Cycle.Night == nil || detail.Cycle.Night.ID != n2.ID {
		t.Errorf("orphan cycle: night = %+v, want id %d", detail.Cycle.Night, n2.ID)
	}
}

func TestGetCycleDetailInProgressDay(t *testing.T) {
	ts := newTestServer(t)

	startDay(t, ts)

	resp := doGet(t, ts, "/api/session/current")
	var sr SessionResponseJSON
	decodeJSON(t, resp, &sr)
	dayID := *sr.SessionID

	// Day is open, no night yet. Cycle detail should return day-only.
	resp = doGet(t, ts, fmt.Sprintf("/api/cycles/%d", dayID))
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}

	var detail struct {
		Cycle struct {
			Day   *struct{ ID int64 `json:"id"` } `json:"day"`
			Night any `json:"night"`
		} `json:"cycle"`
	}
	decodeJSON(t, resp, &detail)

	if detail.Cycle.Day == nil || detail.Cycle.Day.ID != dayID {
		t.Errorf("in-progress cycle: day = %+v, want id %d", detail.Cycle.Day, dayID)
	}
	if detail.Cycle.Night != nil {
		t.Errorf("in-progress cycle: night = %v, want nil", detail.Cycle.Night)
	}
}

func TestGetCycleDetailNotFound(t *testing.T) {
	ts := newTestServer(t)

	resp := doGet(t, ts, "/api/cycles/999")
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

// --- suggestions (breast, ferber) ---

func TestBreastSuggestion(t *testing.T) {
	ts := newTestServer(t)

	startNight(t, ts)
	doPost(t, ts, "/api/session/event", map[string]any{
		"action":   "start_feed",
		"metadata": map[string]string{"breast": "L"},
	})

	// While feeding L: currentBreast=L, suggestBreast=R.
	resp := doGet(t, ts, "/api/session/current")
	var sr SessionResponseJSON
	decodeJSON(t, resp, &sr)
	if sr.CurrentBreast != "L" {
		t.Errorf("currentBreast = %q, want L", sr.CurrentBreast)
	}
	if sr.SuggestBreast != "R" {
		t.Errorf("suggestBreast = %q, want R", sr.SuggestBreast)
	}

	// Switch to R: currentBreast=R, suggestBreast=L.
	doPost(t, ts, "/api/session/event", map[string]any{
		"action":   "switch_breast",
		"metadata": map[string]string{"breast": "R"},
	})
	resp = doGet(t, ts, "/api/session/current")
	decodeJSON(t, resp, &sr)
	if sr.CurrentBreast != "R" {
		t.Errorf("currentBreast after switch = %q, want R", sr.CurrentBreast)
	}
	if sr.SuggestBreast != "L" {
		t.Errorf("suggestBreast after switch = %q, want L", sr.SuggestBreast)
	}
}

func TestLastFeedStartedAtAfterFeed(t *testing.T) {
	ts := newTestServer(t)

	startNight(t, ts)
	feedTime := "2026-03-29T02:10:00-07:00"
	doPost(t, ts, "/api/session/event", map[string]any{
		"action":    "start_feed",
		"metadata":  map[string]string{"breast": "L"},
		"timestamp": feedTime,
	})
	doPost(t, ts, "/api/session/event", map[string]any{"action": "dislatch_awake"})

	resp := doGet(t, ts, "/api/session/current")
	var sr SessionResponseJSON
	decodeJSON(t, resp, &sr)

	parsed, err := time.Parse(time.RFC3339, sr.LastFeedStartedAt)
	if err != nil {
		t.Fatalf("lastFeedStartedAt %q: %v", sr.LastFeedStartedAt, err)
	}
	want, _ := time.Parse(time.RFC3339, feedTime)
	if !parsed.Equal(want) {
		t.Errorf("lastFeedStartedAt = %v, want %v", parsed, want)
	}
}

func TestSuggestFerberNight_OnNightOff(t *testing.T) {
	ts, s := newTestServerWithStore(t)

	// No previous sessions: field absent.
	resp := doGet(t, ts, "/api/session/current")
	var sr SessionResponseJSON
	decodeJSON(t, resp, &sr)
	if sr.State != "night_off" {
		t.Fatalf("state = %q, want night_off", sr.State)
	}
	if sr.SuggestFerberNight != nil {
		t.Errorf("suggestFerberNight = %v, want nil with no prior nights", *sr.SuggestFerberNight)
	}

	// Seed a previous Ferber night at number 4 → suggestion becomes 5.
	n, _ := s.CreateSession(domain.SessionKindNight, time.Now().Add(-12*time.Hour), true, 4, false)
	_ = s.EndSession(n.ID, time.Now().Add(-2*time.Hour))

	resp = doGet(t, ts, "/api/session/current")
	decodeJSON(t, resp, &sr)
	if sr.SuggestFerberNight == nil || *sr.SuggestFerberNight != 5 {
		t.Errorf("suggestFerberNight = %v, want 5", sr.SuggestFerberNight)
	}
}

// TestSuggestFerberNight_OnDayAwake verifies the suggestion also appears when
// the user is in DayAwake (about to tap "Start night" for the chain advance).
func TestSuggestFerberNight_OnDayAwake(t *testing.T) {
	ts, s := newTestServerWithStore(t)

	// Seed a prior Ferber night.
	n, _ := s.CreateSession(domain.SessionKindNight, time.Now().Add(-36*time.Hour), true, 4, false)
	_ = s.EndSession(n.ID, time.Now().Add(-28*time.Hour))

	// Start a day session via the chain.
	startDay(t, ts)

	resp := doGet(t, ts, "/api/session/current")
	var sr SessionResponseJSON
	decodeJSON(t, resp, &sr)
	if sr.State != "day_awake" {
		t.Fatalf("state = %q, want day_awake", sr.State)
	}
	if sr.SuggestFerberNight == nil || *sr.SuggestFerberNight != 5 {
		t.Errorf("suggestFerberNight on day_awake = %v, want 5", sr.SuggestFerberNight)
	}
}

func TestSuggestFerberNight_AbsentMidNight(t *testing.T) {
	ts := newTestServer(t)

	startNight(t, ts)

	resp := doGet(t, ts, "/api/session/current")
	var sr SessionResponseJSON
	decodeJSON(t, resp, &sr)
	if sr.State == "night_off" {
		t.Fatalf("state = night_off, expected mid-night")
	}
	if sr.SuggestFerberNight != nil {
		t.Errorf("suggestFerberNight = %v, want nil mid-night", *sr.SuggestFerberNight)
	}
}

// --- Ferber end-to-end ---

func TestStartSessionNightWithFerber(t *testing.T) {
	ts, s := newTestServerWithStore(t)

	body := map[string]any{
		"kind":      "night",
		"ferber":    map[string]any{"nightNumber": 3},
		"timestamp": "2026-04-20T21:00:00Z",
	}
	resp := doPost(t, ts, "/api/session/start", body)
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, b)
	}

	sess, _, err := s.CurrentSession()
	if err != nil {
		t.Fatalf("CurrentSession: %v", err)
	}
	if sess == nil || !sess.FerberEnabled || sess.FerberNightNumber == nil || *sess.FerberNightNumber != 3 {
		t.Errorf("session has wrong Ferber state: %+v", sess)
	}
}

func TestFerberSessionResponseShape(t *testing.T) {
	ts := newTestServer(t)

	if r := doPost(t, ts, "/api/session/start", map[string]any{
		"kind":   "night",
		"ferber": map[string]any{"nightNumber": 1},
	}); r.StatusCode != 200 {
		t.Fatalf("start_night: %d", r.StatusCode)
	}

	var awakeResp SessionResponseJSON
	decodeJSON(t, doGet(t, ts, "/api/session/current"), &awakeResp)
	if awakeResp.Ferber == nil {
		t.Fatal("ferber = nil on Ferber night Awake, want non-nil")
	}
	if awakeResp.Ferber.Current != nil {
		t.Error("ferber.current non-nil on Awake, want nil")
	}

	if r := doPost(t, ts, "/api/session/event", map[string]any{
		"action":   "put_down_awake_ferber",
		"metadata": map[string]string{"mood": "fussy"},
	}); r.StatusCode != 200 {
		t.Fatalf("put_down_awake_ferber: %d", r.StatusCode)
	}

	var learningResp SessionResponseJSON
	decodeJSON(t, doGet(t, ts, "/api/session/current"), &learningResp)
	if learningResp.Ferber == nil || learningResp.Ferber.Current == nil {
		t.Fatal("ferber.current = nil on Learning, want populated")
	}
	cur := learningResp.Ferber.Current
	if cur.CheckInCount != 0 {
		t.Errorf("checkInCount = %d, want 0", cur.CheckInCount)
	}
	if cur.Mood != "fussy" {
		t.Errorf("mood = %q, want fussy", cur.Mood)
	}
	if cur.StartedAt == "" {
		t.Error("startedAt empty, want populated")
	}
	if cur.CheckInAvailableAt == "" {
		t.Fatal("checkInAvailableAt empty, want populated in Learning")
	}
}

func TestValidActions_FerberNight(t *testing.T) {
	ts := newTestServer(t)

	resp := doPost(t, ts, "/api/session/start", map[string]any{
		"kind":   "night",
		"ferber": map[string]any{"nightNumber": 1},
	})
	if resp.StatusCode != 200 {
		t.Fatalf("start_night: %d", resp.StatusCode)
	}

	var got SessionResponseJSON
	decodeJSON(t, resp, &got)

	has := func(a string) bool {
		for _, x := range got.ValidActions {
			if x == a {
				return true
			}
		}
		return false
	}

	if !has("put_down_awake_ferber") {
		t.Errorf("validActions missing put_down_awake_ferber; got %v", got.ValidActions)
	}
	if has("put_down_awake") {
		t.Errorf("validActions contains plain put_down_awake on Ferber night; got %v", got.ValidActions)
	}
}

func TestValidActions_NonFerberNight(t *testing.T) {
	ts := newTestServer(t)

	resp := startNight(t, ts)
	if resp.StatusCode != 200 {
		t.Fatalf("start_night: %d", resp.StatusCode)
	}

	var got SessionResponseJSON
	decodeJSON(t, resp, &got)

	if got.Ferber != nil {
		t.Fatal("ferber != nil on non-Ferber night")
	}

	has := func(a string) bool {
		for _, x := range got.ValidActions {
			if x == a {
				return true
			}
		}
		return false
	}

	if !has("put_down_awake") {
		t.Errorf("validActions missing put_down_awake; got %v", got.ValidActions)
	}
	if has("put_down_awake_ferber") {
		t.Errorf("validActions contains put_down_awake_ferber on plain night; got %v", got.ValidActions)
	}
}

func TestValidActions_DayAwakeIncludesStartNight(t *testing.T) {
	ts := newTestServer(t)

	resp := startDay(t, ts)
	var got SessionResponseJSON
	decodeJSON(t, resp, &got)

	has := func(a string) bool {
		for _, x := range got.ValidActions {
			if x == a {
				return true
			}
		}
		return false
	}

	// Day-awake actions: start_feed, start_sleep, poop_start, AND start_night (chain advance).
	for _, want := range []string{"start_feed", "start_sleep", "poop_start", "start_night"} {
		if !has(want) {
			t.Errorf("DayAwake missing valid action %q; got %v", want, got.ValidActions)
		}
	}
	if has("end_night") {
		t.Errorf("DayAwake has stale end_night; got %v", got.ValidActions)
	}
}

// --- CSV export ---

func TestExportCSV(t *testing.T) {
	ts := newTestServer(t)

	startNight(t, ts)
	doPost(t, ts, "/api/session/event", map[string]any{
		"action":   "start_feed",
		"metadata": map[string]string{"breast": "L"},
	})
	doPost(t, ts, "/api/session/event", map[string]any{"action": "dislatch_awake"})
	doPost(t, ts, "/api/session/start", map[string]any{"kind": "day"}) // chain advance

	resp := doGet(t, ts, "/api/export/csv")
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/csv" {
		t.Errorf("Content-Type = %s, want text/csv", ct)
	}

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	csvBody := string(body)

	if !strings.Contains(csvBody, "session_id") || !strings.Contains(csvBody, "action") || !strings.Contains(csvBody, "breast") {
		t.Errorf("CSV header missing expected columns; body: %s", csvBody)
	}
	if strings.Contains(csvBody, "night_id") {
		t.Error("CSV still contains legacy 'night_id' column")
	}
}

func TestExportCSVEmpty(t *testing.T) {
	ts := newTestServer(t)

	resp := doGet(t, ts, "/api/export/csv")
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d", resp.StatusCode)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "session_id") {
		t.Error("empty export should still contain CSV header")
	}
}

// --- Chair end-to-end ---

func TestStartSessionNightWithChair(t *testing.T) {
	ts, s := newTestServerWithStore(t)

	body := map[string]any{
		"kind":      "night",
		"chair":     true,
		"timestamp": "2026-04-25T21:00:00Z",
	}
	resp := doPost(t, ts, "/api/session/start", body)
	if resp.StatusCode != 200 {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, b)
	}

	sess, _, err := s.CurrentSession()
	if err != nil {
		t.Fatalf("CurrentSession: %v", err)
	}
	if sess == nil || !sess.ChairEnabled {
		t.Errorf("session ChairEnabled = false, want true; got %+v", sess)
	}
	if sess != nil && sess.FerberEnabled {
		t.Error("FerberEnabled should be false on a chair session")
	}
}

func TestStartSession_Chair_FeatureFlagDisabled_400(t *testing.T) {
	ts, _ := newTestServerWithConfig(t, Config{ChairEnabled: false})

	resp := doPost(t, ts, "/api/session/start", map[string]any{
		"kind":  "night",
		"chair": true,
	})
	if resp.StatusCode != 400 {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestStartSession_Chair_KindDay_400(t *testing.T) {
	ts := newTestServer(t)

	resp := doPost(t, ts, "/api/session/start", map[string]any{
		"kind":  "day",
		"chair": true,
	})
	if resp.StatusCode != 400 {
		t.Errorf("status = %d, want 400", resp.StatusCode)
	}
}

func TestStartSession_FerberAndChair_MutuallyExclusive_400(t *testing.T) {
	ts := newTestServer(t)

	resp := doPost(t, ts, "/api/session/start", map[string]any{
		"kind":   "night",
		"ferber": map[string]any{"nightNumber": 1},
		"chair":  true,
	})
	if resp.StatusCode != 400 {
		t.Errorf("status = %d, want 400 (mutually exclusive)", resp.StatusCode)
	}
}

func TestSessionResponse_ValidActions_ChairNight(t *testing.T) {
	ts := newTestServer(t)

	if r := doPost(t, ts, "/api/session/start", map[string]any{
		"kind":  "night",
		"chair": true,
	}); r.StatusCode != 200 {
		t.Fatalf("start_night chair: %d", r.StatusCode)
	}

	var sr SessionResponseJSON
	decodeJSON(t, doGet(t, ts, "/api/session/current"), &sr)

	if !slices.Contains(sr.ValidActions, "sit_chair") {
		t.Errorf("chair-night Awake actions missing sit_chair: %v", sr.ValidActions)
	}
	if slices.Contains(sr.ValidActions, "put_down_awake") {
		t.Errorf("chair-night Awake actions should not include put_down_awake (sit_chair takes its slot): %v", sr.ValidActions)
	}
	if !sr.ChairEnabled {
		t.Error("chairEnabled = false on chair night, want true")
	}
}

// TestSessionResponse_ChairFlagOff_StoredChairSessionStillRendersChairActions
// is the in-progress-survives-flag-flip guarantee: a session created when the
// flag was on must continue to render chair actions even after the operator
// flips the flag off. Action selection reads from the session row, not config.
func TestSessionResponse_ChairFlagOff_StoredChairSessionStillRendersChairActions(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	// Seed an in-progress chair session directly (bypasses API; mimics a
	// session that was started when the flag was on).
	startedAt := time.Now().Add(-time.Hour)
	sess, err := s.CreateSession(domain.SessionKindNight, startedAt, false, 0, true)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	startEvt := &domain.Event{
		SessionID: sess.ID,
		FromState: domain.NightOff,
		Action:    domain.StartNight,
		ToState:   domain.Awake,
		Timestamp: startedAt,
		Seq:       1,
	}
	if err := s.AddEvent(startEvt); err != nil {
		t.Fatalf("AddEvent: %v", err)
	}

	// Now wire a server with CHAIR flag OFF.
	handler := NewHandler(s, Config{ChairEnabled: false})
	ts := httptest.NewServer(NewRouter(handler))
	t.Cleanup(ts.Close)

	var sr SessionResponseJSON
	decodeJSON(t, doGet(t, ts, "/api/session/current"), &sr)

	if !slices.Contains(sr.ValidActions, "sit_chair") {
		t.Errorf("stored chair session should still expose sit_chair when flag is off: %v", sr.ValidActions)
	}
}

func TestSuggestChair_LastNightWasChair_FlagOn(t *testing.T) {
	ts, s := newTestServerWithStore(t)

	// Seed a closed chair night.
	startedAt := time.Now().Add(-24 * time.Hour)
	n, err := s.CreateSession(domain.SessionKindNight, startedAt, false, 0, true)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := s.EndSession(n.ID, startedAt.Add(8*time.Hour)); err != nil {
		t.Fatalf("EndSession: %v", err)
	}

	var sr SessionResponseJSON
	decodeJSON(t, doGet(t, ts, "/api/session/current"), &sr)
	if !sr.SuggestChair {
		t.Error("suggestChair = false, want true after a chair night")
	}
}

func TestSuggestChair_FlagOff_Suppressed(t *testing.T) {
	ts, s := newTestServerWithConfig(t, Config{ChairEnabled: false})

	// Seed a closed chair night even though the flag is currently off.
	startedAt := time.Now().Add(-24 * time.Hour)
	n, err := s.CreateSession(domain.SessionKindNight, startedAt, false, 0, true)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := s.EndSession(n.ID, startedAt.Add(8*time.Hour)); err != nil {
		t.Fatalf("EndSession: %v", err)
	}

	var sr SessionResponseJSON
	decodeJSON(t, doGet(t, ts, "/api/session/current"), &sr)
	if sr.SuggestChair {
		t.Error("suggestChair = true with flag off, want false (suppressed)")
	}
}
