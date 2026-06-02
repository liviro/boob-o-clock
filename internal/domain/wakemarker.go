package domain

import (
	"fmt"
	"time"
)

// AlreadyAwakeError reports that the most-recent event at the requested time
// already leaves the baby in the session's awake state. A wake marker would be
// redundant there — the moment is already a valid split point.
type AlreadyAwakeError struct {
	State State
}

func (e *AlreadyAwakeError) Error() string {
	return fmt.Sprintf("baby was already %s at that time", e.State)
}

// PlanWakeMarker builds the synthetic "baby woke" event to splice into a
// session at time t, so that t becomes a valid split point.
//
// It is the data-repair counterpart to a wake the user never logged: the state
// machine permits jumping straight from a sleep state into POOP (or a feed),
// so a parent who taps Poop the instant the baby wakes leaves no AWAKE moment
// for the splitter to land on. This reconstructs that missing wake.
//
// The returned event's Seq is the slot it must occupy; the store shifts every
// existing event at or after that seq up by one to make room. FromState is set
// to the state the baby was actually in — cosmetic, since DeriveState reads
// only ToState, but kept honest. t must fall strictly after its preceding
// event so seq order stays aligned with chronological order.
func PlanWakeMarker(session Session, events []Event, t time.Time) (Event, error) {
	awake := Awake
	if session.Kind == SessionKindDay {
		awake = DayAwake
	}

	if !t.After(session.StartedAt) {
		return Event{}, fmt.Errorf("time is before this session started")
	}
	if session.EndedAt != nil && !t.Before(*session.EndedAt) {
		return Event{}, fmt.Errorf("time is after this session ended")
	}

	// Most-recent event with timestamp <= t. Events are seq-ordered, which is
	// chronological order within a single session.
	kIdx := -1
	for i, e := range events {
		if !e.Timestamp.After(t) { // timestamp <= t
			kIdx = i
		} else {
			break
		}
	}
	if kIdx < 0 {
		return Event{}, fmt.Errorf("pick a moment after some activity was logged")
	}

	prev := events[kIdx]
	if prev.ToState == awake {
		return Event{}, &AlreadyAwakeError{State: prev.ToState}
	}
	// Strictly after the preceding event so the inserted seq slot lines up with
	// time order (a tie would make seq and timestamp disagree).
	if !t.After(prev.Timestamp) {
		return Event{}, fmt.Errorf("an event is already logged at that exact time; nudge the time a second later")
	}

	return Event{
		SessionID: session.ID,
		FromState: prev.ToState,
		Action:    BabyWoke,
		ToState:   awake,
		Timestamp: t,
		Seq:       prev.Seq + 1,
	}, nil
}
