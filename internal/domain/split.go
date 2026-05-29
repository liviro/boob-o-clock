package domain

import (
	"fmt"
	"time"
)

// SplitResult describes the plan for splitting a session at timestamp t.
// Symmetric over session kind: Degenerate has the OPPOSITE kind of the
// original (a single chain event, ~1s long), Trailing has the SAME kind.
// The storage layer assigns IDs/SessionIDs and persists this atomically.
type SplitResult struct {
	// The shortened original session ends here (caller updates ended_at = t).
	OriginalEndedAt time.Time

	// New degenerate session of the opposite kind, with its single synthetic
	// chain event (start_day for a night-split, start_night for a day-split).
	Degenerate      Session
	DegenerateEvent Event

	// New trailing session of the same kind as the original, with its
	// synthetic chain-back opener and the events re-parented from the original.
	// EventsToReparent are ordered by timestamp with Seq pre-set (starting at
	// 2; the synthetic opener TrailingStart is seq=1). Each carries its
	// existing ID so the store can UPDATE it in place.
	Trailing         Session
	TrailingStart    Event
	EventsToReparent []Event
}

// SplitSession plans the split of a session (night OR day) at timestamp t.
//
// Validity: the derived state at t — walking events with timestamp <= t — must
// equal the session's rest state (Awake for night, DayAwake for day), and the
// most recent such event must not be the session's opening event. t must lie
// strictly inside the session's range, there must be at least one event after
// t to move into the trailing session, and the first such event must not fall
// inside the (t, t+1s] window the degenerate session occupies.
//
// On rejection the error names the derived state at t when relevant, so the
// caller can surface useful UI.
func SplitSession(session Session, events []Event, t time.Time) (SplitResult, error) {
	// 1. Lower bound: t strictly after session start.
	if !t.After(session.StartedAt) {
		return SplitResult{}, fmt.Errorf("split time is before this session started")
	}
	// Upper bound for closed sessions (open sessions are bounded by the
	// caller against "now"; see the API handler).
	if session.EndedAt != nil && !t.Before(*session.EndedAt) {
		return SplitResult{}, fmt.Errorf("split time is after this session ended")
	}

	// 2. Kind-specific flavors.
	validityState := Awake
	openerAction := StartNight
	degenerateKind := SessionKindDay
	degenerateAction := StartDay
	degenerateToState := DayAwake
	trailingAction := StartNight
	if session.Kind == SessionKindDay {
		validityState = DayAwake
		openerAction = StartDay
		degenerateKind = SessionKindNight
		degenerateAction = StartNight
		degenerateToState = Awake
		trailingAction = StartDay
	}

	// 3. Find the most recent event with timestamp <= t (events are seq-ordered,
	//    which equals chronological order within a single session).
	kIdx := -1
	for i, e := range events {
		if !e.Timestamp.After(t) { // timestamp <= t
			kIdx = i
		} else {
			break
		}
	}
	if kIdx < 0 {
		return SplitResult{}, fmt.Errorf("pick a moment after some activity was logged")
	}
	// 4. The most-recent event must not be the opener.
	if events[kIdx].Action == openerAction && kIdx == 0 {
		return SplitResult{}, fmt.Errorf("pick a moment after some activity was logged")
	}
	// 5. Rest-state validity.
	if events[kIdx].ToState != validityState {
		return SplitResult{}, fmt.Errorf("baby was %s at that time", events[kIdx].ToState)
	}

	// 6. Gather events strictly after t (these move to trailing).
	reparent := make([]Event, 0, len(events)-kIdx-1)
	reparent = append(reparent, events[kIdx+1:]...)
	if len(reparent) == 0 {
		return SplitResult{}, fmt.Errorf("pick a moment before this session's last activity")
	}

	degEnd := t.Add(time.Second)
	trailingStartTS := degEnd

	// Re-parenting gap guard: the trailing's synthetic opener sits at t+1s; a
	// re-parented event inside (t, t+1s) would precede its own opener in time.
	// Reject rather than produce a malformed session.
	if reparent[0].Timestamp.Before(trailingStartTS) {
		return SplitResult{}, fmt.Errorf("pick a moment a little earlier — the next activity is too close to the split point")
	}

	// Re-sequence re-parented events from 2 (synthetic opener is seq 1).
	for i := range reparent {
		reparent[i].Seq = i + 2
	}

	return SplitResult{
		OriginalEndedAt: t,
		Degenerate: Session{
			Kind:      degenerateKind,
			StartedAt: t,
			EndedAt:   &degEnd,
			// Degenerate is always mode-disabled (it's a chain artifact).
		},
		DegenerateEvent: Event{
			FromState: validityState,
			Action:    degenerateAction,
			ToState:   degenerateToState,
			Timestamp: t,
			Seq:       1,
		},
		Trailing: Session{
			Kind:      session.Kind,
			StartedAt: trailingStartTS,
			EndedAt:   session.EndedAt, // inherit (nil if original was open)
			// Mode inheritance collapses to copying the original's flags: day
			// sessions always carry false/nil, so this is correct for both kinds.
			FerberEnabled:     session.FerberEnabled,
			FerberNightNumber: session.FerberNightNumber,
			ChairEnabled:      session.ChairEnabled,
		},
		TrailingStart: Event{
			FromState: degenerateToState,
			Action:    trailingAction,
			ToState:   validityState,
			Timestamp: trailingStartTS,
			Seq:       1,
		},
		EventsToReparent: reparent,
	}, nil
}
