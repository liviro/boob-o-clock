package domain

import (
	"fmt"
	"sort"
)

type transitionKey struct {
	From   State
	Action Action
}

// transitions is the complete state machine transition table.
// Night subgraph + day subgraph + two cross-boundary edges joining them.
var transitions = map[transitionKey]State{
	// === Night subgraph ===

	// 1: NIGHT_OFF → AWAKE
	{NightOff, StartNight}: Awake,

	// 2: AWAKE → FEEDING
	{Awake, StartFeed}: Feeding,
	// 3: AWAKE → RESETTLING
	{Awake, StartResettle}: Resettling,
	// 4: AWAKE → STROLLING
	{Awake, StartStrolling}: Strolling,
	// 5: AWAKE → POOP
	{Awake, PoopStart}: Poop,
	// 6: AWAKE → SELF_SOOTHING
	{Awake, PutDownAwake}: SelfSoothing,

	// 7: FEEDING → AWAKE
	{Feeding, DislatchAwake}: Awake,
	// 8: FEEDING → SLEEPING_ON_ME
	{Feeding, DislatchAsleep}: SleepingOnMe,
	// 9: FEEDING → FEEDING (switch breast)
	{Feeding, SwitchBreast}: Feeding,

	// 10: SLEEPING_ON_ME → TRANSFERRING
	{SleepingOnMe, StartTransfer}: Transferring,
	// 11: SLEEPING_ON_ME → FEEDING
	{SleepingOnMe, StartFeed}: Feeding,
	// 12: SLEEPING_ON_ME → AWAKE
	{SleepingOnMe, BabyWoke}: Awake,
	// 13: SLEEPING_ON_ME → POOP
	{SleepingOnMe, PoopStart}: Poop,

	// 14: TRANSFERRING → SLEEPING_CRIB
	{Transferring, TransferSuccess}: SleepingCrib,
	// 15: TRANSFERRING → RESETTLING
	{Transferring, TransferNeedResettle}: Resettling,
	// 16: TRANSFERRING → AWAKE
	{Transferring, TransferFailed}: Awake,

	// 17: RESETTLING → SLEEPING_CRIB
	{Resettling, Settled}: SleepingCrib,
	// 18: RESETTLING → AWAKE
	{Resettling, ResettleFailed}: Awake,
	// 19: RESETTLING → POOP
	{Resettling, PoopStart}: Poop,

	// 20: SLEEPING_CRIB → AWAKE
	{SleepingCrib, BabyWoke}: Awake,
	// 21: SLEEPING_CRIB → POOP
	{SleepingCrib, PoopStart}: Poop,
	// 22: SLEEPING_CRIB → SELF_SOOTHING
	{SleepingCrib, BabyStirred}: SelfSoothing,

	// 23: SELF_SOOTHING → SLEEPING_CRIB
	{SelfSoothing, Settled}: SleepingCrib,
	// 24: SELF_SOOTHING → AWAKE
	{SelfSoothing, BabyWoke}: Awake,
	// 25: SELF_SOOTHING → POOP
	{SelfSoothing, PoopStart}: Poop,

	// 26: STROLLING → SLEEPING_STROLLER
	{Strolling, FellAsleep}: SleepingStroller,
	// 27: STROLLING → AWAKE
	{Strolling, GiveUp}: Awake,
	// 28: STROLLING → POOP
	{Strolling, PoopStart}: Poop,

	// 29: SLEEPING_STROLLER → AWAKE
	{SleepingStroller, BabyWoke}: Awake,
	// 30: SLEEPING_STROLLER → POOP
	{SleepingStroller, PoopStart}: Poop,

	// 31: POOP → AWAKE
	{Poop, PoopDone}: Awake,

	// Ferber sub-machine (still within night subgraph).
	// 32: AWAKE → LEARNING
	{Awake, PutDownAwakeFerber}: Learning,
	// 33: SLEEPING_CRIB → LEARNING
	{SleepingCrib, BabyStirredFerber}: Learning,
	// 34: LEARNING → LEARNING (mood change)
	{Learning, MoodChange}: Learning,
	// 35: LEARNING → CHECK_IN
	{Learning, CheckInStart}: CheckIn,
	// 36: LEARNING → SLEEPING_CRIB
	{Learning, Settled}: SleepingCrib,
	// 37: LEARNING → AWAKE
	{Learning, ExitFerber}: Awake,
	// 38: CHECK_IN → LEARNING
	{CheckIn, EndCheckIn}: Learning,
	// 39: CHECK_IN → SLEEPING_CRIB
	{CheckIn, Settled}: SleepingCrib,
	// 40: CHECK_IN → AWAKE
	{CheckIn, ExitFerber}: Awake,

	// === Cross-boundary chain advances (Awake ↔ DayAwake) ===

	// 41: AWAKE → DAY_AWAKE (chain advance: morning)
	{Awake, StartDay}: DayAwake,
	// 42: DAY_AWAKE → AWAKE (chain advance: bedtime)
	{DayAwake, StartNight}: Awake,
	// 43: NIGHT_OFF → DAY_AWAKE (first-ever-start in day mode or post-migration)
	{NightOff, StartDay}: DayAwake,

	// === Day subgraph ===

	// 44: DAY_AWAKE → DAY_FEEDING
	{DayAwake, StartFeed}: DayFeeding,
	// 45: DAY_AWAKE → DAY_SLEEPING
	{DayAwake, StartSleep}: DaySleeping,
	// 46: DAY_AWAKE → DAY_POOP
	{DayAwake, PoopStart}: DayPoop,

	// 47: DAY_FEEDING → DAY_AWAKE
	{DayFeeding, DislatchAwake}: DayAwake,
	// 48: DAY_FEEDING → DAY_SLEEPING (handler implicitly fills location=on_me)
	{DayFeeding, DislatchAsleep}: DaySleeping,
	// 49: DAY_FEEDING → DAY_FEEDING (switch breast)
	{DayFeeding, SwitchBreast}: DayFeeding,
	// 50: DAY_FEEDING → DAY_POOP
	{DayFeeding, PoopStart}: DayPoop,

	// 51: DAY_SLEEPING → DAY_AWAKE
	{DaySleeping, BabyWoke}: DayAwake,
	// 52: DAY_SLEEPING → DAY_POOP
	{DaySleeping, PoopStart}: DayPoop,

	// 53: DAY_POOP → DAY_AWAKE
	{DayPoop, PoopDone}: DayAwake,
}

// actionsRequiringBreast is the set of actions that need breast metadata.
var actionsRequiringBreast = map[Action]bool{
	StartFeed:    true,
	SwitchBreast: true,
}

// actionsRequiringMood is the set of actions that need mood metadata.
var actionsRequiringMood = map[Action]bool{
	PutDownAwakeFerber: true,
	BabyStirredFerber:  true,
	MoodChange:         true,
	EndCheckIn:         true,
}

// actionsRequiringLocation is the set of actions that need location metadata.
var actionsRequiringLocation = map[Action]bool{
	StartSleep: true,
}

// Transition attempts a state transition and returns the new state.
// Returns an error if the transition is invalid or required metadata is missing.
func Transition(from State, action Action, metadata map[string]string) (State, error) {
	if actionsRequiringBreast[action] {
		if err := validateBreast(metadata); err != nil {
			return "", err
		}
	}
	if actionsRequiringMood[action] {
		if err := validateMood(metadata); err != nil {
			return "", err
		}
	}
	if actionsRequiringLocation[action] {
		if err := validateLocation(metadata); err != nil {
			return "", err
		}
	}

	to, ok := transitions[transitionKey{from, action}]
	if !ok {
		return "", fmt.Errorf("invalid transition: %s -> %s", from, action)
	}

	return to, nil
}

// actionOrder defines the canonical display order for actions.
//
// Chain-advance actions (StartNight, StartDay) sort LAST so they appear at
// the bottom of the grid rather than the top — they terminate the current
// session and aren't the primary daytime/nighttime activities, so visual
// de-emphasis helps. Session-creation flow for new users still works because
// these are the only valid actions from NightOff.
var actionOrder = func() map[Action]int {
	all := []Action{
		// Feeding cluster.
		StartFeed, DislatchAwake, DislatchAsleep, SwitchBreast,
		// Transfer / resettle / crib cluster.
		StartTransfer, TransferSuccess, TransferNeedResettle, TransferFailed,
		PutDownAwake, PutDownAwakeFerber,
		StartResettle, Settled, ResettleFailed, BabyWoke,
		// Stroller cluster.
		StartStrolling, FellAsleep, GiveUp,
		// Day-specific sleep entry (parallel to StartStrolling).
		StartSleep,
		// Crib-stirring cluster.
		BabyStirred, BabyStirredFerber,
		// Ferber cluster.
		MoodChange, CheckInStart, EndCheckIn, ExitFerber,
		// Poop cluster.
		PoopStart, PoopDone,
		// Session-creation actions (chain-advance) sort LAST.
		StartNight, StartDay,
	}
	m := make(map[Action]int, len(all))
	for i, a := range all {
		m[a] = i
	}
	return m
}()

// validActionsMap is pre-computed from the transition table, sorted by canonical order.
var validActionsMap = func() map[State][]Action {
	m := make(map[State][]Action)
	for key := range transitions {
		m[key.From] = append(m[key.From], key.Action)
	}
	for _, actions := range m {
		sort.Slice(actions, func(i, j int) bool {
			return actionOrder[actions[i]] < actionOrder[actions[j]]
		})
	}
	return m
}()

// ValidActions returns all actions available from the given state.
func ValidActions(state State) []Action {
	return validActionsMap[state]
}

// DeriveState returns the current state from an event log.
// Returns NightOff if the event log is empty (brand-new install or
// chain-off after all events undone).
//
// Nuance: under option-B chain semantics a closed session's last event
// lands in Awake or DayAwake, not a terminal *_Off state. DeriveState is
// authoritative only for OPEN sessions. Callers that need to know whether
// a session is closed must consult session.EndedAt, not DeriveState.
func DeriveState(events []Event) State {
	if len(events) == 0 {
		return NightOff
	}
	return events[len(events)-1].ToState
}

func validateBreast(metadata map[string]string) error {
	if metadata == nil {
		return fmt.Errorf("breast metadata required")
	}
	b, ok := metadata["breast"]
	if !ok {
		return fmt.Errorf("breast metadata required")
	}
	if b != string(Left) && b != string(Right) {
		return fmt.Errorf("invalid breast: %s (must be L or R)", b)
	}
	return nil
}

func validateMood(metadata map[string]string) error {
	if metadata == nil {
		return fmt.Errorf("mood metadata required")
	}
	m, ok := metadata["mood"]
	if !ok {
		return fmt.Errorf("mood metadata required")
	}
	if m != "quiet" && m != "fussy" && m != "crying" {
		return fmt.Errorf("invalid mood: %s (must be quiet, fussy, or crying)", m)
	}
	return nil
}

func validateLocation(metadata map[string]string) error {
	if metadata == nil {
		return fmt.Errorf("location metadata required")
	}
	loc, ok := metadata["location"]
	if !ok {
		return fmt.Errorf("location metadata required")
	}
	switch loc {
	case string(LocationCrib), string(LocationStroller), string(LocationOnMe), string(LocationCar):
		return nil
	default:
		return fmt.Errorf("invalid location: %s (must be crib, stroller, on_me, or car)", loc)
	}
}
