package api

import (
	"net/http"
	"testing"
)

// --- GET /api/config ---

type configResponseJSON struct {
	Features struct {
		Ferber bool `json:"ferber"`
	} `json:"features"`
}

func TestGetConfigFerberDisabled(t *testing.T) {
	ts, _ := newTestServerWithConfig(t, Config{FerberEnabled: false})

	resp := doGet(t, ts, "/api/config")
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var c configResponseJSON
	decodeJSON(t, resp, &c)
	if c.Features.Ferber {
		t.Errorf("features.ferber = true, want false")
	}
}

func TestGetConfigFerberEnabled(t *testing.T) {
	ts, _ := newTestServerWithConfig(t, Config{FerberEnabled: true})

	resp := doGet(t, ts, "/api/config")
	if resp.StatusCode != 200 {
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	var c configResponseJSON
	decodeJSON(t, resp, &c)
	if !c.Features.Ferber {
		t.Errorf("features.ferber = false, want true")
	}
}

// --- POST /api/session/start with Ferber flag off ---

func TestStartSessionRejectsFerberWhenDisabled(t *testing.T) {
	ts, _ := newTestServerWithConfig(t, Config{FerberEnabled: false})

	resp := doPost(t, ts, "/api/session/start", map[string]any{
		"kind":   "night",
		"ferber": map[string]any{"nightNumber": 1},
	})
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 400 when Ferber is disabled", resp.StatusCode)
	}
}

func TestStartSessionPlainNightWorksWhenFerberDisabled(t *testing.T) {
	ts, _ := newTestServerWithConfig(t, Config{FerberEnabled: false})

	// Plain night (no ferber config) must still succeed when the flag is off —
	// we only block Ferber *opt-in*, not normal night starts.
	resp := startNight(t, ts)
	if resp.StatusCode != http.StatusOK {
		t.Errorf("plain start_night status = %d, want 200", resp.StatusCode)
	}
}

// --- sanity: newTestServer uses Ferber-enabled default, so Ferber starts still work ---

func TestStartSessionFerberWorksWithDefaultTestConfig(t *testing.T) {
	ts := newTestServer(t)

	resp := doPost(t, ts, "/api/session/start", map[string]any{
		"kind":   "night",
		"ferber": map[string]any{"nightNumber": 2},
	})
	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200", resp.StatusCode)
	}
}

