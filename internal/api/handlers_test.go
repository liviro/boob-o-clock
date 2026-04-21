package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/liviro/boob-o-clock/internal/store"
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

func newTestServerWithStore(t *testing.T) (*httptest.Server, *store.Store) {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := store.New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { s.Close() })

	handler := NewHandler(s)
	router := NewRouter(handler)
	ts := httptest.NewServer(router)
	t.Cleanup(ts.Close)
	return ts, s
}

func makeNightAt(t *testing.T, s *store.Store, startedAt time.Time) int64 {
	t.Helper()
	n, err := s.CreateNight(startedAt, false, 0)
	if err != nil {
		t.Fatalf("CreateNight: %v", err)
	}
	if err := s.EndNight(n.ID, startedAt.Add(time.Hour)); err != nil {
		t.Fatalf("EndNight: %v", err)
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

// SessionResponse matches the JSON returned by GET /api/session/current and POST /api/session/event
type SessionResponse struct {
	State             string   `json:"state"`
	ValidActions      []string `json:"validActions"`
	NightID           *int64   `json:"nightId"`
	SuggestBreast     string   `json:"suggestBreast"`
	CurrentBreast     string   `json:"currentBreast"`
	LastFeedStartedAt string   `json:"lastFeedStartedAt"`
	LastEvent         *struct {
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

	resp := doPost(t, ts, "/api/session/start", map[string]any{})
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
	doPost(t, ts, "/api/session/start", map[string]any{})

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
		var resp *http.Response
		if step.action == "start_night" {
			resp = doPost(t, ts, "/api/session/start", map[string]any{})
		} else {
			body := map[string]any{"action": step.action}
			if step.metadata != nil {
				body["metadata"] = step.metadata
			}
			resp = doPost(t, ts, "/api/session/event", body)
		}
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
	doPost(t, ts, "/api/session/start", map[string]any{})
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
	doPost(t, ts, "/api/session/start", map[string]any{})
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
		"timestamp": "2026-03-29T03:00:00-07:00",
	}
	resp := doPost(t, ts, "/api/session/start", body)
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
	doPost(t, ts, "/api/session/start", map[string]any{})
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
	// Feed count is 0: no crib/stroller sleep in this night, so pre-sleep feeds are excluded
	if fc, ok := detail.Stats["feedCount"].(float64); !ok || fc != 0 {
		t.Errorf("stats.feedCount = %v, want 0", detail.Stats["feedCount"])
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
	doPost(t, ts, "/api/session/start", map[string]any{})
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
	// Feed count is 0: no crib/stroller sleep in this night, so pre-sleep feeds are excluded
	if fc, ok := result.Nights[0].Stats["feedCount"].(float64); !ok || fc != 0 {
		t.Errorf("stats.feedCount = %v, want 0", result.Nights[0].Stats["feedCount"])
	}
}

func TestBreastSuggestion(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	doPost(t, ts, "/api/session/start", map[string]any{})
	doPost(t, ts, "/api/session/event", map[string]any{
		"action":   "start_feed",
		"metadata": map[string]string{"breast": "L"},
	})

	// While feeding L: currentBreast=L, suggestBreast=R
	resp := doGet(t, ts, "/api/session/current")
	var sr SessionResponse
	decodeJSON(t, resp, &sr)
	if sr.CurrentBreast != "L" {
		t.Errorf("currentBreast = %q, want L", sr.CurrentBreast)
	}
	if sr.SuggestBreast != "R" {
		t.Errorf("suggestBreast = %q, want R", sr.SuggestBreast)
	}

	// Switch to R: currentBreast=R, suggestBreast=L
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

func TestLastFeedStartedAtEmpty(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	doPost(t, ts, "/api/session/start", map[string]any{})

	resp := doGet(t, ts, "/api/session/current")
	var sr SessionResponse
	decodeJSON(t, resp, &sr)

	if sr.LastFeedStartedAt != "" {
		t.Errorf("lastFeedStartedAt = %q, want empty before any feed", sr.LastFeedStartedAt)
	}
}

func TestLastFeedStartedAtAfterFeed(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	doPost(t, ts, "/api/session/start", map[string]any{})
	feedTime := "2026-03-29T02:10:00-07:00"
	doPost(t, ts, "/api/session/event", map[string]any{
		"action":    "start_feed",
		"metadata":  map[string]string{"breast": "L"},
		"timestamp": feedTime,
	})
	doPost(t, ts, "/api/session/event", map[string]any{"action": "dislatch_awake"})

	resp := doGet(t, ts, "/api/session/current")
	var sr SessionResponse
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

func TestLastFeedStartedAtIgnoresSwitchBreast(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	doPost(t, ts, "/api/session/start", map[string]any{})
	feedTime := "2026-03-29T02:10:00-07:00"
	doPost(t, ts, "/api/session/event", map[string]any{
		"action":    "start_feed",
		"metadata":  map[string]string{"breast": "L"},
		"timestamp": feedTime,
	})
	doPost(t, ts, "/api/session/event", map[string]any{
		"action":    "switch_breast",
		"metadata":  map[string]string{"breast": "R"},
		"timestamp": "2026-03-29T02:25:00-07:00",
	})

	resp := doGet(t, ts, "/api/session/current")
	var sr SessionResponse
	decodeJSON(t, resp, &sr)

	parsed, err := time.Parse(time.RFC3339, sr.LastFeedStartedAt)
	if err != nil {
		t.Fatalf("lastFeedStartedAt %q: %v", sr.LastFeedStartedAt, err)
	}
	want, _ := time.Parse(time.RFC3339, feedTime)
	if !parsed.Equal(want) {
		t.Errorf("lastFeedStartedAt = %v, want %v (switch_breast should not reset)", parsed, want)
	}
}

func TestGetTrends(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	// Create a complete night
	doPost(t, ts, "/api/session/start", map[string]any{})
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
	doPost(t, ts, "/api/session/start", map[string]any{})
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

	body, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	csvBody := string(body)

	// Should have header + 4 data rows
	lines := strings.Count(csvBody, "\n")
	if lines < 5 {
		t.Errorf("expected at least 5 lines (header + 4 events), got %d", lines)
	}

	if !strings.Contains(csvBody, "night_id") || !strings.Contains(csvBody, "action") || !strings.Contains(csvBody, "breast") {
		t.Error("CSV header missing expected columns")
	}
}

func TestExportCSVEmpty(t *testing.T) {
	ts := newTestServer(t)
	defer ts.Close()

	resp := doGet(t, ts, "/api/export/csv")
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "text/csv" {
		t.Errorf("Content-Type = %s, want text/csv", ct)
	}
	body, _ := io.ReadAll(resp.Body)
	resp.Body.Close()
	if !strings.Contains(string(body), "night_id") {
		t.Error("empty export should still contain CSV header")
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

// TestGetNightsHonorsExplicitRange exercises the from/to query parameters.
func TestGetNightsHonorsExplicitRange(t *testing.T) {
	ts, s := newTestServerWithStore(t)

	now := time.Now()
	makeNightAt(t, s, now.Add(-5*24*time.Hour))
	insideID := makeNightAt(t, s, now.Add(-20*24*time.Hour))
	makeNightAt(t, s, now.Add(-45*24*time.Hour))

	// Bracket only the 20-day-old night.
	from := now.Add(-30 * 24 * time.Hour).Format("2006-01-02")
	to := now.Add(-10 * 24 * time.Hour).Format("2006-01-02")
	resp := doGet(t, ts, "/api/nights?from="+from+"&to="+to)
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var result struct {
		Nights []struct {
			ID int64 `json:"id"`
		} `json:"nights"`
	}
	decodeJSON(t, resp, &result)

	if len(result.Nights) != 1 {
		t.Fatalf("got %d nights, want 1 (only the 20-day-old one); response = %+v", len(result.Nights), result.Nights)
	}
	if result.Nights[0].ID != insideID {
		t.Errorf("got night id %d, want %d", result.Nights[0].ID, insideID)
	}
}

// TestGetNightsIncludesOldNights asserts the default date window includes
// nights well beyond the previous 30-day cutoff. Boundaries (60 in, 100 out)
// are deliberately not pinned to exactly 90 days — the test stays meaningful
// for any future window between roughly 61 and 99 days.
func TestGetNightsIncludesOldNights(t *testing.T) {
	ts, s := newTestServerWithStore(t)

	now := time.Now()
	recentID := makeNightAt(t, s, now.Add(-60*24*time.Hour))
	oldID := makeNightAt(t, s, now.Add(-100*24*time.Hour))

	resp := doGet(t, ts, "/api/nights")
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var result struct {
		Nights []struct {
			ID int64 `json:"id"`
		} `json:"nights"`
	}
	decodeJSON(t, resp, &result)

	ids := map[int64]bool{}
	for _, n := range result.Nights {
		ids[n.ID] = true
	}
	if !ids[recentID] {
		t.Errorf("expected 60-day-old night (id=%d) in response, got %+v", recentID, result.Nights)
	}
	if ids[oldID] {
		t.Errorf("did not expect 100-day-old night (id=%d) in response, got %+v", oldID, result.Nights)
	}
}

func TestStartNightWithFerberMetadata(t *testing.T) {
	ts, store := newTestServerWithStore(t)

	body := strings.NewReader(`{
		"ferber":{"nightNumber":3},
		"timestamp":"2026-04-20T21:00:00Z"
	}`)
	resp, err := http.Post(ts.URL+"/api/session/start", "application/json", body)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("status = %d, body = %s", resp.StatusCode, b)
	}

	night, _, err := store.CurrentSession()
	if err != nil {
		t.Fatalf("CurrentSession: %v", err)
	}
	if night == nil || !night.FerberEnabled || night.FerberNightNumber == nil || *night.FerberNightNumber != 3 {
		t.Errorf("night has wrong Ferber state: %+v", night)
	}
}

func TestSessionResponseIncludesFerberFields(t *testing.T) {
	ts, _ := newTestServerWithStore(t)

	body := strings.NewReader(`{"ferber":{"nightNumber":2}}`)
	_, err := http.Post(ts.URL+"/api/session/start", "application/json", body)
	if err != nil {
		t.Fatalf("POST: %v", err)
	}

	resp, err := http.Get(ts.URL + "/api/session/current")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()

	var got struct {
		Ferber *struct {
			NightNumber int `json:"nightNumber"`
		} `json:"ferber,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.Ferber == nil {
		t.Fatal("ferber = nil, want non-nil on a Ferber night")
	}
	if got.Ferber.NightNumber != 2 {
		t.Errorf("ferber.nightNumber = %d, want 2", got.Ferber.NightNumber)
	}
}

// TestGetTrendsIncludesOldNights confirms /api/trends shares the same date
// window as /api/nights.
func TestGetTrendsIncludesOldNights(t *testing.T) {
	ts, s := newTestServerWithStore(t)

	now := time.Now()
	makeNightAt(t, s, now.Add(-60*24*time.Hour))

	resp := doGet(t, ts, "/api/trends")
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var result struct {
		Trends []struct {
			Date string `json:"date"`
		} `json:"trends"`
	}
	decodeJSON(t, resp, &result)

	if len(result.Trends) == 0 {
		t.Fatalf("expected at least one trend point for the 60-day-old night, got 0")
	}
}

func TestSuggestFerberNight_OnSessionCurrent(t *testing.T) {
	ts, s := newTestServerWithStore(t)

	// No previous nights: field should be absent on the NightOff session.
	resp, err := http.Get(ts.URL + "/api/session/current")
	if err != nil {
		t.Fatalf("GET: %v", err)
	}
	defer resp.Body.Close()
	var got struct {
		State              string `json:"state"`
		SuggestFerberNight *int   `json:"suggestFerberNight,omitempty"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&got); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if got.State != "night_off" {
		t.Fatalf("state = %q, want night_off", got.State)
	}
	if got.SuggestFerberNight != nil {
		t.Errorf("suggestFerberNight = %v, want nil with no prior Ferber nights", *got.SuggestFerberNight)
	}

	// With a previous Ferber night at number 4, the field becomes 5.
	n, _ := s.CreateNight(time.Now().Add(-12*time.Hour), true, 4)
	_ = s.EndNight(n.ID, time.Now().Add(-2*time.Hour))

	resp2, _ := http.Get(ts.URL + "/api/session/current")
	defer resp2.Body.Close()
	_ = json.NewDecoder(resp2.Body).Decode(&got)
	if got.SuggestFerberNight == nil || *got.SuggestFerberNight != 5 {
		t.Errorf("suggestFerberNight = %v, want 5", got.SuggestFerberNight)
	}
}

func TestSuggestFerberNight_AbsentMidNight(t *testing.T) {
	// The suggestion only belongs on NightOff. A mid-night session response
	// must not carry it.
	ts := newTestServer(t)

	_ = doPost(t, ts, "/api/session/start", map[string]any{})

	resp, _ := http.Get(ts.URL + "/api/session/current")
	defer resp.Body.Close()
	var got struct {
		State              string `json:"state"`
		SuggestFerberNight *int   `json:"suggestFerberNight,omitempty"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if got.State == "night_off" {
		t.Fatalf("state = night_off, expected mid-night")
	}
	if got.SuggestFerberNight != nil {
		t.Errorf("suggestFerberNight = %v, want nil mid-night", *got.SuggestFerberNight)
	}
}

// TestValidActions_FerberNight verifies that after starting a Ferber night,
// the AWAKE state's valid actions contain the Ferber variant (put_down_awake_ferber)
// and NOT the plain variant. This ensures the client renders the 🌱 button.
func TestValidActions_FerberNight(t *testing.T) {
	ts := newTestServer(t)

	// Start a Ferber-enabled night.
	resp := doPost(t, ts, "/api/session/start", map[string]any{
		"ferber": map[string]any{"nightNumber": 1},
	})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("start_night: status %d", resp.StatusCode)
	}

	var got sessionResponse
	decodeJSON(t, resp, &got)

	if got.State != "awake" {
		t.Fatalf("state = %q, want awake", got.State)
	}
	if got.Ferber == nil {
		t.Fatal("ferber = nil, want non-nil on a Ferber night")
	}

	has := func(a string) bool {
		for _, x := range got.ValidActions {
			if string(x) == a {
				return true
			}
		}
		return false
	}

	if !has("put_down_awake_ferber") {
		t.Errorf("validActions missing put_down_awake_ferber; got %v", got.ValidActions)
	}
	if has("put_down_awake") {
		t.Errorf("validActions contains put_down_awake; should be hidden on Ferber nights; got %v", got.ValidActions)
	}
}

// TestValidActions_NonFerberNight verifies that a plain (non-Ferber) night
// sees the plain put_down_awake action and NOT the Ferber variant.
func TestValidActions_NonFerberNight(t *testing.T) {
	ts := newTestServer(t)

	resp := doPost(t, ts, "/api/session/start", map[string]any{})
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("start_night: status %d", resp.StatusCode)
	}

	var got sessionResponse
	decodeJSON(t, resp, &got)

	if got.Ferber != nil {
		t.Fatal("ferber != nil, want nil on a non-Ferber night")
	}

	has := func(a string) bool {
		for _, x := range got.ValidActions {
			if string(x) == a {
				return true
			}
		}
		return false
	}

	if !has("put_down_awake") {
		t.Errorf("validActions missing put_down_awake; got %v", got.ValidActions)
	}
	if has("put_down_awake_ferber") {
		t.Errorf("validActions contains put_down_awake_ferber; should be hidden on non-Ferber nights; got %v", got.ValidActions)
	}
}

// TestFerberSessionResponseShape verifies the nested ferber/current structure:
// - ferber.current is absent right after start_night (state=Awake)
// - ferber.current is populated after PutDownAwakeFerber (state=Learning)
// - all current fields carry the expected values
func TestFerberSessionResponseShape(t *testing.T) {
	ts := newTestServer(t)

	if r := doPost(t, ts, "/api/session/start", map[string]any{
		"ferber": map[string]any{"nightNumber": 1},
	}); r.StatusCode != 200 {
		t.Fatalf("start_night: %d", r.StatusCode)
	}

	var awakeResp sessionResponse
	decodeJSON(t, doGet(t, ts, "/api/session/current"), &awakeResp)
	if awakeResp.Ferber == nil {
		t.Fatal("ferber = nil on Ferber night Awake, want non-nil")
	}
	if awakeResp.Ferber.Current != nil {
		t.Error("ferber.current non-nil on Awake, want nil (no active learning session yet)")
	}

	if r := doPost(t, ts, "/api/session/event", map[string]any{
		"action":   "put_down_awake_ferber",
		"metadata": map[string]string{"mood": "fussy"},
	}); r.StatusCode != 200 {
		t.Fatalf("put_down_awake_ferber: %d", r.StatusCode)
	}

	var learningResp sessionResponse
	decodeJSON(t, doGet(t, ts, "/api/session/current"), &learningResp)
	if learningResp.Ferber == nil || learningResp.Ferber.Current == nil {
		t.Fatal("ferber.current = nil on Learning, want populated")
	}
	cur := learningResp.Ferber.Current
	if cur.CheckInCount != 0 {
		t.Errorf("checkInCount = %d, want 0 (no check-ins yet)", cur.CheckInCount)
	}
	if cur.Mood != "fussy" {
		t.Errorf("mood = %q, want fussy", cur.Mood)
	}
	if cur.StartedAt.IsZero() {
		t.Error("startedAt is zero, want populated")
	}
	if cur.CheckInAvailableAt == nil {
		t.Fatal("checkInAvailableAt = nil, want populated in Learning")
	}
	// Night 1, check-in 1: interval is 3 minutes from startedAt.
	wantAvail := cur.StartedAt.Add(3 * time.Minute)
	if !cur.CheckInAvailableAt.Equal(wantAvail) {
		t.Errorf("checkInAvailableAt = %v, want startedAt+3m = %v", cur.CheckInAvailableAt, wantAvail)
	}
}

// TestValidActions_FerberStir verifies that from SLEEPING_CRIB on a Ferber night,
// the stir action exposed is the ferber variant.
func TestValidActions_FerberStir(t *testing.T) {
	ts := newTestServer(t)

	// Start Ferber night, put baby down awake (ferber, requires mood), settle.
	if r := doPost(t, ts, "/api/session/start", map[string]any{
		"ferber": map[string]any{"nightNumber": 1},
	}); r.StatusCode != 200 {
		t.Fatalf("start_night: %d", r.StatusCode)
	} else {
		r.Body.Close()
	}
	if r := doPost(t, ts, "/api/session/event", map[string]any{
		"action":   "put_down_awake_ferber",
		"metadata": map[string]string{"mood": "quiet"},
	}); r.StatusCode != 200 {
		t.Fatalf("put_down_awake_ferber: %d", r.StatusCode)
	} else {
		r.Body.Close()
	}
	r := doPost(t, ts, "/api/session/event", map[string]any{"action": "settled"})
	defer r.Body.Close()
	if r.StatusCode != 200 {
		t.Fatalf("settled: %d", r.StatusCode)
	}

	var got sessionResponse
	decodeJSON(t, r, &got)

	if got.State != "sleeping_crib" {
		t.Fatalf("state = %q, want sleeping_crib", got.State)
	}

	has := func(a string) bool {
		for _, x := range got.ValidActions {
			if string(x) == a {
				return true
			}
		}
		return false
	}
	if !has("baby_stirred_ferber") {
		t.Errorf("validActions missing baby_stirred_ferber; got %v", got.ValidActions)
	}
	if has("baby_stirred") {
		t.Errorf("validActions contains plain baby_stirred on Ferber night; got %v", got.ValidActions)
	}
}
