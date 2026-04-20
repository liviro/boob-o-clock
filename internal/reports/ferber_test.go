package reports

import (
	"testing"
	"time"

	"github.com/liviro/boob-o-clock/internal/domain"
)

// helper: build an event for test input. Seq is ignored by the stats layer.
func evt(from domain.State, action domain.Action, to domain.State, ts time.Time, meta map[string]string) domain.Event {
	return domain.Event{
		FromState: from,
		Action:    action,
		ToState:   to,
		Timestamp: ts,
		Metadata:  meta,
	}
}

func mins(n int) time.Duration { return time.Duration(n) * time.Minute }

func TestComputeFerberStats_Empty(t *testing.T) {
	got := ComputeFerberStats(nil, time.Now())
	if got != (FerberStats{}) {
		t.Errorf("expected zero stats, got %+v", got)
	}
}

func TestComputeFerberStats_SingleQuickSettle(t *testing.T) {
	// One session: put down at 0, settled at 8 min. No check-ins, all quiet.
	t0 := time.Date(2026, 4, 20, 21, 0, 0, 0, time.UTC)
	events := []domain.Event{
		evt(domain.Awake, domain.PutDownAwakeFerber, domain.Learning, t0, map[string]string{"mood": "quiet"}),
		evt(domain.Learning, domain.Settled, domain.SleepingCrib, t0.Add(8*time.Minute), nil),
	}
	got := ComputeFerberStats(events, t0.Add(10*time.Minute))
	if got.Sessions != 1 {
		t.Errorf("Sessions = %d, want 1", got.Sessions)
	}
	if got.CheckIns != 0 {
		t.Errorf("CheckIns = %d, want 0", got.CheckIns)
	}
	if got.QuietTime != mins(8) {
		t.Errorf("QuietTime = %v, want 8m", got.QuietTime)
	}
	if got.CryTime != 0 || got.FussTime != 0 {
		t.Errorf("expected cry/fuss = 0, got cry=%v fuss=%v", got.CryTime, got.FussTime)
	}
	if got.AvgTimeToSettle != mins(8) {
		t.Errorf("AvgTimeToSettle = %v, want 8m", got.AvgTimeToSettle)
	}
}

func TestComputeFerberStats_MoodChangesAndCheckIn(t *testing.T) {
	// Session with: quiet 0-5m, fussy 5-12m, crying 12-15m (check-in at 15m),
	// check-in duration 1m, then back to quiet 16-20m (settled).
	t0 := time.Date(2026, 4, 20, 21, 0, 0, 0, time.UTC)
	events := []domain.Event{
		evt(domain.Awake, domain.PutDownAwakeFerber, domain.Learning, t0, map[string]string{"mood": "quiet"}),
		evt(domain.Learning, domain.MoodChange, domain.Learning, t0.Add(5*time.Minute), map[string]string{"mood": "fussy"}),
		evt(domain.Learning, domain.MoodChange, domain.Learning, t0.Add(12*time.Minute), map[string]string{"mood": "crying"}),
		evt(domain.Learning, domain.CheckInStart, domain.CheckIn, t0.Add(15*time.Minute), nil),
		evt(domain.CheckIn, domain.EndCheckIn, domain.Learning, t0.Add(16*time.Minute), map[string]string{"mood": "quiet"}),
		evt(domain.Learning, domain.Settled, domain.SleepingCrib, t0.Add(20*time.Minute), nil),
	}
	got := ComputeFerberStats(events, t0.Add(25*time.Minute))
	if got.Sessions != 1 {
		t.Errorf("Sessions = %d, want 1", got.Sessions)
	}
	if got.CheckIns != 1 {
		t.Errorf("CheckIns = %d, want 1", got.CheckIns)
	}
	if got.QuietTime != mins(5)+mins(4) {
		t.Errorf("QuietTime = %v, want 9m", got.QuietTime)
	}
	if got.FussTime != mins(7) {
		t.Errorf("FussTime = %v, want 7m", got.FussTime)
	}
	// Cry time: 3 min in LEARNING + 1 min in CHECK_IN entered from crying mood.
	if got.CryTime != mins(4) {
		t.Errorf("CryTime = %v, want 4m", got.CryTime)
	}
	if got.AvgTimeToSettle != mins(20) {
		t.Errorf("AvgTimeToSettle = %v, want 20m", got.AvgTimeToSettle)
	}
	if got.SessionsAbandoned != 0 {
		t.Errorf("SessionsAbandoned = %d, want 0", got.SessionsAbandoned)
	}
}

func TestComputeFerberStats_AbandonedSession(t *testing.T) {
	t0 := time.Date(2026, 4, 20, 21, 0, 0, 0, time.UTC)
	events := []domain.Event{
		evt(domain.Awake, domain.PutDownAwakeFerber, domain.Learning, t0, map[string]string{"mood": "crying"}),
		evt(domain.Learning, domain.ExitFerber, domain.Awake, t0.Add(10*time.Minute), nil),
	}
	got := ComputeFerberStats(events, t0.Add(15*time.Minute))
	if got.SessionsAbandoned != 1 {
		t.Errorf("SessionsAbandoned = %d, want 1", got.SessionsAbandoned)
	}
	// Abandoned sessions should NOT contribute to AvgTimeToSettle.
	if got.AvgTimeToSettle != 0 {
		t.Errorf("AvgTimeToSettle = %v, want 0 (abandoned)", got.AvgTimeToSettle)
	}
	if got.CryTime != mins(10) {
		t.Errorf("CryTime = %v, want 10m", got.CryTime)
	}
}

func TestComputeFerberStats_MultipleSessions(t *testing.T) {
	// Two sessions in one night. First: 8m to settle. Second: 12m.
	t0 := time.Date(2026, 4, 20, 21, 0, 0, 0, time.UTC)
	events := []domain.Event{
		// Session 1
		evt(domain.Awake, domain.PutDownAwakeFerber, domain.Learning, t0, map[string]string{"mood": "quiet"}),
		evt(domain.Learning, domain.Settled, domain.SleepingCrib, t0.Add(8*time.Minute), nil),
		// (baby sleeps, stirs after an hour)
		evt(domain.SleepingCrib, domain.BabyStirredFerber, domain.Learning, t0.Add(68*time.Minute), map[string]string{"mood": "fussy"}),
		evt(domain.Learning, domain.Settled, domain.SleepingCrib, t0.Add(80*time.Minute), nil),
	}
	got := ComputeFerberStats(events, t0.Add(85*time.Minute))
	if got.Sessions != 2 {
		t.Errorf("Sessions = %d, want 2", got.Sessions)
	}
	if got.AvgTimeToSettle != mins(10) {
		t.Errorf("AvgTimeToSettle = %v, want 10m", got.AvgTimeToSettle)
	}
}

func TestComputeFerberStats_OpenSession(t *testing.T) {
	// Session in progress: entered LEARNING 5m ago, no exit yet.
	t0 := time.Date(2026, 4, 20, 21, 0, 0, 0, time.UTC)
	events := []domain.Event{
		evt(domain.Awake, domain.PutDownAwakeFerber, domain.Learning, t0, map[string]string{"mood": "fussy"}),
	}
	nightEnd := t0.Add(5 * time.Minute)
	got := ComputeFerberStats(events, nightEnd)
	if got.Sessions != 1 {
		t.Errorf("Sessions = %d, want 1", got.Sessions)
	}
	if got.FussTime != mins(5) {
		t.Errorf("FussTime = %v, want 5m (fussy through nightEnd)", got.FussTime)
	}
	// Open sessions do not contribute to AvgTimeToSettle.
	if got.AvgTimeToSettle != 0 {
		t.Errorf("AvgTimeToSettle = %v, want 0 (session open)", got.AvgTimeToSettle)
	}
}
