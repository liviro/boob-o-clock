package domain

import (
	"testing"
	"time"
)

// splitBase anchors split fixtures to a concrete Monday 21:00 for readable times.
func splitBase() time.Time {
	return time.Date(2026, 5, 25, 21, 0, 0, 0, time.Local)
}

// mkEvent builds an event with seq and the given timestamp. SessionID/IDs are
// left zero — the domain planner doesn't depend on them.
func mkEvent(seq int, from State, action Action, to State, ts time.Time) Event {
	return Event{Seq: seq, FromState: from, Action: action, ToState: to, Timestamp: ts}
}

func ptrTime(t time.Time) *time.Time { return &t }

// A night that ran across a full day: opened Mon 21:00, baby slept, woke
// Tue 07:00 (Awake), then the parent kept logging into the over-long night.
// We split at Tue 07:30 — strictly inside the Awake span between the 07:00
// wake and the next event at 09:00.
func nightSpanningEvents() (Session, []Event, time.Time) {
	b := splitBase()
	end := b.Add(20 * time.Hour) // Tue 17:00 — session ended_at
	sess := Session{
		ID:        1,
		Kind:      SessionKindNight,
		StartedAt: b,
		EndedAt:   ptrTime(end),
	}
	events := []Event{
		mkEvent(1, NightOff, StartNight, Awake, b),                          // Mon 21:00
		mkEvent(2, Awake, StartFeed, Feeding, b.Add(30*time.Minute)),        // Mon 21:30
		mkEvent(3, Feeding, DislatchAsleep, SleepingOnMe, b.Add(time.Hour)), // Mon 22:00
		mkEvent(4, SleepingOnMe, BabyWoke, Awake, b.Add(10*time.Hour)),      // Tue 07:00 (rest)
		mkEvent(5, Awake, StartFeed, Feeding, b.Add(12*time.Hour)),          // Tue 09:00
		mkEvent(6, Feeding, DislatchAwake, Awake, b.Add(13*time.Hour)),      // Tue 10:00
	}
	splitAt := b.Add(10*time.Hour + 30*time.Minute) // Tue 07:30
	return sess, events, splitAt
}

func TestSplitSession_NightHappyPath(t *testing.T) {
	sess, events, splitAt := nightSpanningEvents()

	res, err := SplitSession(sess, events, splitAt)
	if err != nil {
		t.Fatalf("SplitSession: %v", err)
	}

	// Original is shortened to end at the split time.
	if !res.OriginalEndedAt.Equal(splitAt) {
		t.Errorf("OriginalEndedAt = %v, want %v", res.OriginalEndedAt, splitAt)
	}

	// Degenerate is a DAY session, [splitAt, splitAt+1s], one synthetic start_day.
	if res.Degenerate.Kind != SessionKindDay {
		t.Errorf("Degenerate.Kind = %s, want day", res.Degenerate.Kind)
	}
	if !res.Degenerate.StartedAt.Equal(splitAt) {
		t.Errorf("Degenerate.StartedAt = %v, want %v", res.Degenerate.StartedAt, splitAt)
	}
	if res.Degenerate.EndedAt == nil || !res.Degenerate.EndedAt.Equal(splitAt.Add(time.Second)) {
		t.Errorf("Degenerate.EndedAt = %v, want %v", res.Degenerate.EndedAt, splitAt.Add(time.Second))
	}
	if res.DegenerateEvent.Action != StartDay || res.DegenerateEvent.ToState != DayAwake ||
		res.DegenerateEvent.FromState != Awake || res.DegenerateEvent.Seq != 1 {
		t.Errorf("DegenerateEvent = %+v, want Awake->start_day->day_awake seq=1", res.DegenerateEvent)
	}
	if !res.DegenerateEvent.Timestamp.Equal(splitAt) {
		t.Errorf("DegenerateEvent.Timestamp = %v, want %v", res.DegenerateEvent.Timestamp, splitAt)
	}

	// Trailing is a NIGHT session starting at splitAt+1s, inheriting original ended_at.
	if res.Trailing.Kind != SessionKindNight {
		t.Errorf("Trailing.Kind = %s, want night", res.Trailing.Kind)
	}
	if !res.Trailing.StartedAt.Equal(splitAt.Add(time.Second)) {
		t.Errorf("Trailing.StartedAt = %v, want %v", res.Trailing.StartedAt, splitAt.Add(time.Second))
	}
	if res.Trailing.EndedAt == nil || !res.Trailing.EndedAt.Equal(*sess.EndedAt) {
		t.Errorf("Trailing.EndedAt = %v, want %v", res.Trailing.EndedAt, sess.EndedAt)
	}
	if res.TrailingStart.Action != StartNight || res.TrailingStart.ToState != Awake ||
		res.TrailingStart.FromState != DayAwake || res.TrailingStart.Seq != 1 {
		t.Errorf("TrailingStart = %+v, want DayAwake->start_night->awake seq=1", res.TrailingStart)
	}
	if !res.TrailingStart.Timestamp.Equal(splitAt.Add(time.Second)) {
		t.Errorf("TrailingStart.Timestamp = %v, want %v", res.TrailingStart.Timestamp, splitAt.Add(time.Second))
	}

	// Events after splitAt (seq 5, 6) move to trailing, re-seq'd from 2.
	if len(res.EventsToReparent) != 2 {
		t.Fatalf("EventsToReparent len = %d, want 2", len(res.EventsToReparent))
	}
	if res.EventsToReparent[0].Seq != 2 || res.EventsToReparent[1].Seq != 3 {
		t.Errorf("reparent seqs = [%d %d], want [2 3]",
			res.EventsToReparent[0].Seq, res.EventsToReparent[1].Seq)
	}
	// Re-parented events keep their original timestamps and transitions.
	if !res.EventsToReparent[0].Timestamp.Equal(splitBase().Add(12 * time.Hour)) {
		t.Errorf("first reparent ts = %v, want Tue 09:00", res.EventsToReparent[0].Timestamp)
	}
}

// Day that ran into the night: opened Mon 07:00, baby awake (DayAwake) at
// 19:00, then logging continued past bedtime. Split at 20:00 (DayAwake span).
func daySpanningEvents() (Session, []Event, time.Time) {
	b := time.Date(2026, 5, 25, 7, 0, 0, 0, time.Local)
	end := b.Add(20 * time.Hour) // Tue 03:00
	sess := Session{ID: 1, Kind: SessionKindDay, StartedAt: b, EndedAt: ptrTime(end)}
	events := []Event{
		mkEvent(1, NightOff, StartDay, DayAwake, b),                          // Mon 07:00
		mkEvent(2, DayAwake, StartSleep, DaySleeping, b.Add(3*time.Hour)),    // 10:00 nap
		mkEvent(3, DaySleeping, BabyWoke, DayAwake, b.Add(4*time.Hour)),      // 11:00 (rest)
		mkEvent(4, DayAwake, StartFeed, DayFeeding, b.Add(14*time.Hour)),     // 21:00
		mkEvent(5, DayFeeding, DislatchAwake, DayAwake, b.Add(15*time.Hour)), // 22:00
	}
	splitAt := b.Add(13 * time.Hour) // 20:00 — inside DayAwake span (11:00→21:00)
	return sess, events, splitAt
}

func TestSplitSession_DayHappyPath(t *testing.T) {
	sess, events, splitAt := daySpanningEvents()
	res, err := SplitSession(sess, events, splitAt)
	if err != nil {
		t.Fatalf("SplitSession: %v", err)
	}
	// Degenerate is a NIGHT session; synthetic DayAwake->start_night->Awake.
	if res.Degenerate.Kind != SessionKindNight {
		t.Errorf("Degenerate.Kind = %s, want night", res.Degenerate.Kind)
	}
	if res.DegenerateEvent.Action != StartNight || res.DegenerateEvent.ToState != Awake ||
		res.DegenerateEvent.FromState != DayAwake {
		t.Errorf("DegenerateEvent = %+v, want DayAwake->start_night->awake", res.DegenerateEvent)
	}
	// Trailing is a DAY session; synthetic Awake->start_day->DayAwake.
	if res.Trailing.Kind != SessionKindDay {
		t.Errorf("Trailing.Kind = %s, want day", res.Trailing.Kind)
	}
	if res.TrailingStart.Action != StartDay || res.TrailingStart.ToState != DayAwake ||
		res.TrailingStart.FromState != Awake {
		t.Errorf("TrailingStart = %+v, want Awake->start_day->day_awake", res.TrailingStart)
	}
	// Events after 20:00 (seq 4,5) move to trailing.
	if len(res.EventsToReparent) != 2 {
		t.Fatalf("EventsToReparent len = %d, want 2", len(res.EventsToReparent))
	}
}

func TestSplitSession_OpenSession(t *testing.T) {
	sess, events, splitAt := nightSpanningEvents()
	sess.EndedAt = nil // open session
	res, err := SplitSession(sess, events, splitAt)
	if err != nil {
		t.Fatalf("SplitSession: %v", err)
	}
	if res.Trailing.EndedAt != nil {
		t.Errorf("Trailing.EndedAt = %v, want nil (open trailing)", res.Trailing.EndedAt)
	}
	// Degenerate is always closed even when the original is open.
	if res.Degenerate.EndedAt == nil {
		t.Error("Degenerate.EndedAt = nil, want closed")
	}
}

func TestSplitSession_ExactTimestampMatch(t *testing.T) {
	sess, events, _ := nightSpanningEvents()
	// Split exactly at the Tue 07:00 Awake-entry event timestamp. That event
	// has timestamp <= t, so it stays in the original as its final event;
	// events strictly after move to trailing.
	splitAt := splitBase().Add(10 * time.Hour) // Tue 07:00, == event seq 4
	res, err := SplitSession(sess, events, splitAt)
	if err != nil {
		t.Fatalf("SplitSession: %v", err)
	}
	// seq 5 and 6 move (09:00, 10:00); seq 4 (07:00) stays in original.
	if len(res.EventsToReparent) != 2 {
		t.Fatalf("EventsToReparent len = %d, want 2", len(res.EventsToReparent))
	}
	if !res.EventsToReparent[0].Timestamp.Equal(splitBase().Add(12 * time.Hour)) {
		t.Errorf("first reparent = %v, want Tue 09:00", res.EventsToReparent[0].Timestamp)
	}
}

func TestSplitSession_RejectGapWindow(t *testing.T) {
	// Construct a session where the first event after t lands inside (t, t+1s).
	b := splitBase()
	splitAt := b.Add(10 * time.Hour) // Tue 07:00
	sess := Session{ID: 1, Kind: SessionKindNight, StartedAt: b, EndedAt: ptrTime(b.Add(20 * time.Hour))}
	events := []Event{
		mkEvent(1, NightOff, StartNight, Awake, b),
		mkEvent(2, SleepingOnMe, BabyWoke, Awake, b.Add(9*time.Hour)),            // earlier rest
		mkEvent(3, Awake, StartFeed, Feeding, splitAt.Add(500*time.Millisecond)), // inside (t, t+1s)!
	}
	// Most recent event <= t is seq 2 (Awake). The next event is +500ms.
	_, err := SplitSession(sess, events, splitAt)
	if err == nil {
		t.Fatal("expected gap-window rejection, got nil")
	}
}
