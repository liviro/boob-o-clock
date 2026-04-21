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

func TestSuggestFerberNight(t *testing.T) {
	if got := SuggestFerberNight(nil); got != nil {
		t.Errorf("nil night → want nil, got %v", *got)
	}
	nonFerber := &domain.Night{FerberEnabled: false}
	if got := SuggestFerberNight(nonFerber); got != nil {
		t.Errorf("non-Ferber → want nil, got %v", *got)
	}
	n := 4
	ferber := &domain.Night{FerberEnabled: true, FerberNightNumber: &n}
	got := SuggestFerberNight(ferber)
	if got == nil || *got != 5 {
		t.Errorf("Ferber night 4 → want 5, got %v", got)
	}
	// Enabled without a number (shouldn't happen in practice, but guard) → nil.
	enabledNoNumber := &domain.Night{FerberEnabled: true}
	if got := SuggestFerberNight(enabledNoNumber); got != nil {
		t.Errorf("enabled with nil number → want nil, got %v", *got)
	}
}

func TestCurrentFerberSession_NotInLearningOrCheckIn(t *testing.T) {
	t0 := time.Now()
	events := []domain.Event{
		evt(domain.Awake, domain.PutDownAwakeFerber, domain.Learning, t0, map[string]string{"mood": "quiet"}),
	}
	if got := CurrentFerberSession(domain.Awake, events, 1); got != nil {
		t.Errorf("want nil outside Learning/CheckIn, got %+v", got)
	}
}

func TestCurrentFerberSession_NoEntry(t *testing.T) {
	if got := CurrentFerberSession(domain.Learning, nil, 1); got != nil {
		t.Errorf("want nil with no events, got %+v", got)
	}
}

func TestCurrentFerberSession_CheckInAbsentDuringCheckIn(t *testing.T) {
	// During CheckIn state, the countdown field is absent: the current
	// check-in is in progress, no "next" to count toward.
	t0 := time.Now()
	events := []domain.Event{
		evt(domain.Awake, domain.PutDownAwakeFerber, domain.Learning, t0, map[string]string{"mood": "quiet"}),
		evt(domain.Learning, domain.MoodChange, domain.Learning, t0.Add(mins(3)), map[string]string{"mood": "fussy"}),
		evt(domain.Learning, domain.CheckInStart, domain.CheckIn, t0.Add(mins(5)), nil),
		evt(domain.CheckIn, domain.EndCheckIn, domain.Learning, t0.Add(mins(7)), map[string]string{"mood": "crying"}),
		evt(domain.Learning, domain.CheckInStart, domain.CheckIn, t0.Add(mins(12)), nil),
	}
	got := CurrentFerberSession(domain.CheckIn, events, 1)
	if got == nil {
		t.Fatal("want non-nil live session")
	}
	if got.CheckIns != 2 {
		t.Errorf("CheckIns = %d, want 2", got.CheckIns)
	}
	if !got.SessionStart.Equal(t0) {
		t.Errorf("SessionStart = %v, want %v", got.SessionStart, t0)
	}
	if got.CheckInAvailableAt != nil {
		t.Errorf("CheckInAvailableAt = %v, want nil during CheckIn", got.CheckInAvailableAt)
	}
	if got.Mood != "crying" {
		t.Errorf("Mood = %q, want crying", got.Mood)
	}
}

func TestCurrentFerberSession_CheckInAvailableAtDuringLearning(t *testing.T) {
	// Night 1, no check-ins yet: next check-in is available at sessionStart + 3m.
	t0 := time.Now()
	events := []domain.Event{
		evt(domain.Awake, domain.PutDownAwakeFerber, domain.Learning, t0, map[string]string{"mood": "quiet"}),
	}
	got := CurrentFerberSession(domain.Learning, events, 1)
	if got == nil {
		t.Fatal("want non-nil live session")
	}
	if got.CheckInAvailableAt == nil {
		t.Fatal("CheckInAvailableAt = nil, want non-nil during Learning")
	}
	if !got.CheckInAvailableAt.Equal(t0.Add(mins(3))) {
		t.Errorf("CheckInAvailableAt = %v, want sessionStart+3m (night 1, check-in 1)", got.CheckInAvailableAt)
	}

	// After two check-ins, the base resets to the most recent EndCheckIn and
	// the interval bumps to the third column (10m on night 1).
	events = append(events,
		evt(domain.Learning, domain.CheckInStart, domain.CheckIn, t0.Add(mins(3)), nil),
		evt(domain.CheckIn, domain.EndCheckIn, domain.Learning, t0.Add(mins(4)), map[string]string{"mood": "fussy"}),
		evt(domain.Learning, domain.CheckInStart, domain.CheckIn, t0.Add(mins(9)), nil),
		evt(domain.CheckIn, domain.EndCheckIn, domain.Learning, t0.Add(mins(10)), map[string]string{"mood": "fussy"}),
	)
	got = CurrentFerberSession(domain.Learning, events, 1)
	if got == nil || got.CheckInAvailableAt == nil {
		t.Fatal("want CheckInAvailableAt populated")
	}
	if !got.CheckInAvailableAt.Equal(t0.Add(mins(20))) {
		t.Errorf("CheckInAvailableAt = %v, want lastEndCheckIn+10m = sessionStart+20m", got.CheckInAvailableAt)
	}
}

func TestIntervalFor(t *testing.T) {
	cases := []struct {
		night, checkIn int
		wantMins       int
	}{
		{1, 1, 3},
		{1, 2, 5},
		{1, 3, 10},
		{1, 4, 10}, // column clamps to 3
		{2, 1, 5},
		{7, 3, 30},
		{8, 1, 20},  // night clamps to 7
		{99, 99, 30}, // both clamp
		{0, 0, 3},   // clamps up to night 1, check-in 1
	}
	for _, c := range cases {
		got := IntervalFor(c.night, c.checkIn)
		if got != mins(c.wantMins) {
			t.Errorf("IntervalFor(%d, %d) = %v, want %dm", c.night, c.checkIn, got, c.wantMins)
		}
	}
}

func TestCurrentFerberSession_OnlyMostRecentSession(t *testing.T) {
	// Two sessions in the log; only the most recent should be returned.
	t0 := time.Now()
	events := []domain.Event{
		evt(domain.Awake, domain.PutDownAwakeFerber, domain.Learning, t0, map[string]string{"mood": "quiet"}),
		evt(domain.Learning, domain.ExitFerber, domain.Awake, t0.Add(mins(10)), nil),
		evt(domain.SleepingCrib, domain.BabyStirredFerber, domain.Learning, t0.Add(mins(60)), map[string]string{"mood": "fussy"}),
	}
	got := CurrentFerberSession(domain.Learning, events, 2)
	if got == nil {
		t.Fatal("want non-nil live session")
	}
	if !got.SessionStart.Equal(t0.Add(mins(60))) {
		t.Errorf("SessionStart = %v, want second session start", got.SessionStart)
	}
	if got.Mood != "fussy" {
		t.Errorf("Mood = %q, want fussy (from second session entry)", got.Mood)
	}
	if got.CheckIns != 0 {
		t.Errorf("CheckIns = %d, want 0 in fresh session", got.CheckIns)
	}
}
