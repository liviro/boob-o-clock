package domain

import (
	"errors"
	"testing"
	"time"
)

// nightWithSleepThenPoop reproduces Xayer's shape: the baby goes straight from
// sleeping into a diaper change with no AWAKE state ever derived, so the
// splitter has nothing to land on.
func nightWithSleepThenPoop() (Session, []Event) {
	b := time.Date(2026, 5, 25, 21, 0, 0, 0, time.UTC)
	end := b.Add(50 * time.Hour)
	sess := Session{ID: 1, Kind: SessionKindNight, StartedAt: b, EndedAt: &end}
	events := []Event{
		{SessionID: 1, FromState: NightOff, Action: StartNight, ToState: Awake, Timestamp: b, Seq: 1},
		{SessionID: 1, FromState: Awake, Action: StartResettle, ToState: Resettling, Timestamp: b.Add(30 * time.Minute), Seq: 2},
		{SessionID: 1, FromState: Resettling, Action: Settled, ToState: SleepingCrib, Timestamp: b.Add(time.Hour), Seq: 3},
		// Morning: baby wakes and is changed — sleep -> poop, no awake logged.
		{SessionID: 1, FromState: SleepingCrib, Action: PoopStart, ToState: Poop, Timestamp: b.Add(10 * time.Hour), Seq: 4},
		{SessionID: 1, FromState: Poop, Action: PoopDone, ToState: Awake, Timestamp: b.Add(10*time.Hour + 5*time.Minute), Seq: 5},
	}
	return sess, events
}

func TestPlanWakeMarker_InsertsBeforePoop(t *testing.T) {
	sess, events := nightWithSleepThenPoop()
	at := events[3].Timestamp.Add(-time.Minute) // a minute before the poop

	marker, err := PlanWakeMarker(sess, events, at)
	if err != nil {
		t.Fatalf("PlanWakeMarker: %v", err)
	}
	if marker.ToState != Awake {
		t.Errorf("ToState = %q, want awake", marker.ToState)
	}
	if marker.Action != BabyWoke {
		t.Errorf("Action = %q, want baby_woke", marker.Action)
	}
	// Most-recent event at `at` is the SleepingCrib settle (seq 3), so the
	// marker slots in at seq 4 and FromState mirrors the prior state.
	if marker.Seq != 4 {
		t.Errorf("Seq = %d, want 4", marker.Seq)
	}
	if marker.FromState != SleepingCrib {
		t.Errorf("FromState = %q, want sleeping_crib", marker.FromState)
	}
	if !marker.Timestamp.Equal(at) {
		t.Errorf("Timestamp = %v, want %v", marker.Timestamp, at)
	}
}

func TestPlanWakeMarker_DayUsesDayAwake(t *testing.T) {
	b := time.Date(2026, 5, 25, 8, 0, 0, 0, time.UTC)
	end := b.Add(40 * time.Hour)
	sess := Session{ID: 2, Kind: SessionKindDay, StartedAt: b, EndedAt: &end}
	events := []Event{
		{SessionID: 2, FromState: NightOff, Action: StartDay, ToState: DayAwake, Timestamp: b, Seq: 1},
		{SessionID: 2, FromState: DayAwake, Action: StartSleep, ToState: DaySleeping, Timestamp: b.Add(time.Hour), Seq: 2},
		{SessionID: 2, FromState: DaySleeping, Action: PoopStart, ToState: DayPoop, Timestamp: b.Add(12 * time.Hour), Seq: 3},
	}
	marker, err := PlanWakeMarker(sess, events, b.Add(11*time.Hour))
	if err != nil {
		t.Fatalf("PlanWakeMarker: %v", err)
	}
	if marker.ToState != DayAwake {
		t.Errorf("ToState = %q, want day_awake", marker.ToState)
	}
	if marker.Seq != 3 {
		t.Errorf("Seq = %d, want 3", marker.Seq)
	}
}

func TestPlanWakeMarker_AlreadyAwake(t *testing.T) {
	sess, events := nightWithSleepThenPoop()
	// Just after PoopDone the baby is already Awake — no marker needed.
	at := events[4].Timestamp.Add(time.Minute)

	_, err := PlanWakeMarker(sess, events, at)
	var aae *AlreadyAwakeError
	if !errors.As(err, &aae) {
		t.Fatalf("err = %v, want AlreadyAwakeError", err)
	}
	if aae.State != Awake {
		t.Errorf("AlreadyAwakeError.State = %q, want awake", aae.State)
	}
}

func TestPlanWakeMarker_OutOfBounds(t *testing.T) {
	sess, events := nightWithSleepThenPoop()
	if _, err := PlanWakeMarker(sess, events, sess.StartedAt); err == nil {
		t.Error("expected error for time == start")
	}
	if _, err := PlanWakeMarker(sess, events, sess.EndedAt.Add(time.Hour)); err == nil {
		t.Error("expected error for time after end")
	}
}

func TestPlanWakeMarker_ExactTimeCollision(t *testing.T) {
	sess, events := nightWithSleepThenPoop()
	// Exactly on the poop event — ambiguous seq ordering, so rejected.
	if _, err := PlanWakeMarker(sess, events, events[3].Timestamp); err != nil {
		// poop is at seq 4; landing exactly on it means prev=poop (not awake),
		// and t == prev.Timestamp, so we expect the collision error.
		var aae *AlreadyAwakeError
		if errors.As(err, &aae) {
			t.Fatalf("unexpected AlreadyAwakeError: %v", err)
		}
		return // some error is expected
	}
	t.Error("expected error for time landing exactly on an existing event")
}
