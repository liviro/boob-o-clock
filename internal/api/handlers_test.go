package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/polina/boob-o-clock/internal/store"
)

func newTestServer(t *testing.T) *httptest.Server {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	handler := NewHandler(s)
	router := NewRouter(handler)
	return httptest.NewServer(router)
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

// SessionResponse matches the JSON returned by GET /api/session/current and POST /api/session/event
type SessionResponse struct {
	State         string   `json:"state"`
	ValidActions  []string `json:"validActions"`
	NightID       *int64   `json:"nightId"`
	SuggestBreast string   `json:"suggestBreast"`
	LastEvent     *struct {
		Action    string            `json:"action"`
		FromState string            `json:"fromState"`
		ToState   string            `json:"toState"`
		Metadata  map[string]string `json:"metadata"`
	} `json:"lastEvent"`
}

func TestGetCurrentSessionNoNight(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	resp := doGet(t, ts, "/api/session/current")
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var sr SessionResponse
	decodeJSON(t, resp, &sr)

	if sr.State != "night_off" {
		t.Errorf("state = %s, want night_off", sr.State)
	}
	if len(sr.ValidActions) != 1 || sr.ValidActions[0] != "start_night" {
		t.Errorf("validActions = %v, want [start_night]", sr.ValidActions)
	}
	if sr.NightID != nil {
		t.Errorf("nightId = %v, want nil", sr.NightID)
	}
}

func TestPostEventStartNight(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	body := map[string]any{"action": "start_night"}
	resp := doPost(t, ts, "/api/session/event", body)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var sr SessionResponse
	decodeJSON(t, resp, &sr)

	if sr.State != "awake" {
		t.Errorf("state = %s, want awake", sr.State)
	}
	if sr.NightID == nil {
		t.Fatal("expected nightId")
	}
}

func TestPostEventInvalidTransition(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	body := map[string]any{"action": "start_feed", "metadata": map[string]string{"breast": "L"}}
	resp := doPost(t, ts, "/api/session/event", body)
	if resp.StatusCode != 400 {
		t.Fatalf("status = %d, want 400", resp.StatusCode)
	}
}

func TestPostEventFeedRequiresBreast(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Start night first
	doPost(t, ts, "/api/session/event", map[string]any{"action": "start_night"})

	// Try to feed without breast
	resp := doPost(t, ts, "/api/session/event", map[string]any{"action": "start_feed"})
	if resp.StatusCode != 400 {
		t.Fatalf("status = %d, want 400 for feed without breast", resp.StatusCode)
	}
}

func TestFullNightRoundTrip(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	steps := []struct {
		action    string
		metadata  map[string]string
		wantState string
	}{
		{"start_night", nil, "awake"},
		{"start_feed", map[string]string{"breast": "L"}, "feeding"},
		{"dislatch_asleep", nil, "sleeping_on_me"},
		{"start_transfer", nil, "transferring"},
		{"transfer_success", nil, "sleeping_crib"},
		{"baby_woke", nil, "awake"},
		{"start_feed", map[string]string{"breast": "R"}, "feeding"},
		{"dislatch_awake", nil, "awake"},
		{"end_night", nil, "night_off"},
	}

	for _, step := range steps {
		body := map[string]any{"action": step.action}
		if step.metadata != nil {
			body["metadata"] = step.metadata
		}

		resp := doPost(t, ts, "/api/session/event", body)
		if resp.StatusCode != 200 {
			t.Fatalf("step %s: status = %d, want 200", step.action, resp.StatusCode)
		}

		var sr SessionResponse
		decodeJSON(t, resp, &sr)

		if sr.State != step.wantState {
			t.Errorf("step %s: state = %s, want %s", step.action, sr.State, step.wantState)
		}
	}

	// Verify session is now inactive
	resp := doGet(t, ts, "/api/session/current")
	var sr SessionResponse
	decodeJSON(t, resp, &sr)
	if sr.State != "night_off" {
		t.Errorf("after end: state = %s, want night_off", sr.State)
	}
}

func TestUndo(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Start night + feed
	doPost(t, ts, "/api/session/event", map[string]any{"action": "start_night"})
	doPost(t, ts, "/api/session/event", map[string]any{
		"action":   "start_feed",
		"metadata": map[string]string{"breast": "L"},
	})

	// Undo feed -> should be back to awake
	resp := doPost(t, ts, "/api/session/undo", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("undo status = %d, want 200", resp.StatusCode)
	}

	var sr SessionResponse
	decodeJSON(t, resp, &sr)
	if sr.State != "awake" {
		t.Errorf("after undo: state = %s, want awake", sr.State)
	}
}

func TestUndoStartNight(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Start night, then undo it
	doPost(t, ts, "/api/session/event", map[string]any{"action": "start_night"})
	resp := doPost(t, ts, "/api/session/undo", nil)
	if resp.StatusCode != 200 {
		t.Fatalf("undo status = %d, want 200", resp.StatusCode)
	}

	var sr SessionResponse
	decodeJSON(t, resp, &sr)
	if sr.State != "night_off" {
		t.Errorf("after undo start_night: state = %s, want night_off", sr.State)
	}

	// Night should be deleted
	resp = doGet(t, ts, "/api/session/current")
	decodeJSON(t, resp, &sr)
	if sr.NightID != nil {
		t.Error("expected nil nightId after undoing start_night")
	}
}

func TestPostEventWithTimestamp(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	body := map[string]any{
		"action":    "start_night",
		"timestamp": "2026-03-29T03:00:00-07:00",
	}
	resp := doPost(t, ts, "/api/session/event", body)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestUndoNoSession(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	resp := doPost(t, ts, "/api/session/undo", nil)
	if resp.StatusCode != 400 {
		t.Fatalf("undo with no session: status = %d, want 400", resp.StatusCode)
	}
}

func TestGetNightDetail(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Create a complete night
	doPost(t, ts, "/api/session/event", map[string]any{"action": "start_night"})
	resp := doPost(t, ts, "/api/session/event", map[string]any{
		"action":   "start_feed",
		"metadata": map[string]string{"breast": "L"},
	})
	var sr SessionResponse
	decodeJSON(t, resp, &sr)
	nightID := *sr.NightID

	doPost(t, ts, "/api/session/event", map[string]any{"action": "dislatch_awake"})
	doPost(t, ts, "/api/session/event", map[string]any{"action": "end_night"})

	// Get night detail
	resp = doGet(t, ts, fmt.Sprintf("/api/nights/%d", nightID))
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var detail struct {
		Night    map[string]any   `json:"night"`
		Events   []map[string]any `json:"events"`
		Timeline []map[string]any `json:"timeline"`
		Stats    map[string]any   `json:"stats"`
	}
	decodeJSON(t, resp, &detail)

	if len(detail.Events) != 4 {
		t.Errorf("got %d events, want 4", len(detail.Events))
	}
	if detail.Timeline == nil {
		t.Error("expected timeline in response")
	}
	if detail.Stats == nil {
		t.Error("expected stats in response")
	}
	if fc, ok := detail.Stats["feedCount"].(float64); !ok || fc != 1 {
		t.Errorf("stats.feedCount = %v, want 1", detail.Stats["feedCount"])
	}
}

func TestGetNightDetailNotFound(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	resp := doGet(t, ts, "/api/nights/999")
	if resp.StatusCode != 404 {
		t.Fatalf("status = %d, want 404", resp.StatusCode)
	}
}

func TestGetNights(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Create a complete night
	doPost(t, ts, "/api/session/event", map[string]any{"action": "start_night"})
	doPost(t, ts, "/api/session/event", map[string]any{
		"action":   "start_feed",
		"metadata": map[string]string{"breast": "L"},
	})
	doPost(t, ts, "/api/session/event", map[string]any{"action": "dislatch_awake"})
	doPost(t, ts, "/api/session/event", map[string]any{"action": "end_night"})

	resp := doGet(t, ts, "/api/nights")
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var result struct {
		Nights []struct {
			ID    int64          `json:"id"`
			Stats map[string]any `json:"stats"`
		} `json:"nights"`
	}
	decodeJSON(t, resp, &result)

	if len(result.Nights) != 1 {
		t.Fatalf("got %d nights, want 1", len(result.Nights))
	}
	if fc, ok := result.Nights[0].Stats["feedCount"].(float64); !ok || fc != 1 {
		t.Errorf("stats.feedCount = %v, want 1", result.Nights[0].Stats["feedCount"])
	}
}

func TestBreastSuggestion(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	doPost(t, ts, "/api/session/event", map[string]any{"action": "start_night"})
	doPost(t, ts, "/api/session/event", map[string]any{
		"action":   "start_feed",
		"metadata": map[string]string{"breast": "L"},
	})

	// While feeding L, should suggest R next
	resp := doGet(t, ts, "/api/session/current")
	var sr SessionResponse
	decodeJSON(t, resp, &sr)
	if sr.SuggestBreast != "R" {
		t.Errorf("suggestBreast = %q, want R", sr.SuggestBreast)
	}

	// Switch to R, should now suggest L
	doPost(t, ts, "/api/session/event", map[string]any{
		"action":   "switch_breast",
		"metadata": map[string]string{"breast": "R"},
	})
	resp = doGet(t, ts, "/api/session/current")
	decodeJSON(t, resp, &sr)
	if sr.SuggestBreast != "L" {
		t.Errorf("suggestBreast after switch = %q, want L", sr.SuggestBreast)
	}
}

func TestGetTrends(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Create a complete night
	doPost(t, ts, "/api/session/event", map[string]any{"action": "start_night"})
	doPost(t, ts, "/api/session/event", map[string]any{
		"action":   "start_feed",
		"metadata": map[string]string{"breast": "L"},
	})
	doPost(t, ts, "/api/session/event", map[string]any{"action": "dislatch_awake"})
	doPost(t, ts, "/api/session/event", map[string]any{"action": "end_night"})

	resp := doGet(t, ts, "/api/trends")
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var result struct {
		Trends []map[string]any `json:"trends"`
		Window int              `json:"window"`
	}
	decodeJSON(t, resp, &result)

	if result.Window != 3 {
		t.Errorf("window = %d, want 3", result.Window)
	}
	if len(result.Trends) != 1 {
		t.Fatalf("got %d trend points, want 1", len(result.Trends))
	}
}

func TestExportCSV(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Create a night with a feed
	doPost(t, ts, "/api/session/event", map[string]any{"action": "start_night"})
	doPost(t, ts, "/api/session/event", map[string]any{
		"action":   "start_feed",
		"metadata": map[string]string{"breast": "L"},
	})
	doPost(t, ts, "/api/session/event", map[string]any{"action": "dislatch_awake"})
	doPost(t, ts, "/api/session/event", map[string]any{"action": "end_night"})

	resp := doGet(t, ts, "/api/export/csv")
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/csv" {
		t.Errorf("Content-Type = %s, want text/csv", ct)
	}

	body := make([]byte, 4096)
	n, _ := resp.Body.Read(body)
	resp.Body.Close()
	csv := string(body[:n])

	// Should have header + 4 data rows
	lines := 0
	for _, c := range csv {
		if c == '\n' {
			lines++
		}
	}
	if lines < 5 {
		t.Errorf("expected at least 5 lines (header + 4 events), got %d", lines)
	}

	// Header should contain expected columns
	if !contains(csv, "night_id") || !contains(csv, "action") || !contains(csv, "breast") {
		t.Error("CSV header missing expected columns")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestExportCSVEmpty(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	resp := doGet(t, ts, "/api/export/csv")
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
}

func TestGetNightsEmpty(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	resp := doGet(t, ts, "/api/nights")
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var result struct {
		Nights []any `json:"nights"`
	}
	decodeJSON(t, resp, &result)

	if len(result.Nights) != 0 {
		t.Errorf("got %d nights, want 0", len(result.Nights))
	}
}
