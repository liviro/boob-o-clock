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
