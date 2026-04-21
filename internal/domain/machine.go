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
var transitions = map[transitionKey]State{
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
	// 7: AWAKE → NIGHT_OFF
	{Awake, EndNight}: NightOff,

	// 8: FEEDING → AWAKE
	{Feeding, DislatchAwake}: Awake,
	// 9: FEEDING → SLEEPING_ON_ME
	{Feeding, DislatchAsleep}: SleepingOnMe,
	// 10: FEEDING → FEEDING (switch breast)
	{Feeding, SwitchBreast}: Feeding,

	// 11: SLEEPING_ON_ME → TRANSFERRING
	{SleepingOnMe, StartTransfer}: Transferring,
	// 12: SLEEPING_ON_ME → FEEDING
	{SleepingOnMe, StartFeed}: Feeding,
	// 13: SLEEPING_ON_ME → AWAKE
	{SleepingOnMe, BabyWoke}: Awake,
	// 14: SLEEPING_ON_ME → POOP
	{SleepingOnMe, PoopStart}: Poop,

	// 15: TRANSFERRING → SLEEPING_CRIB
	{Transferring, TransferSuccess}: SleepingCrib,
	// 16: TRANSFERRING → RESETTLING
	{Transferring, TransferNeedResettle}: Resettling,
	// 17: TRANSFERRING → AWAKE
	{Transferring, TransferFailed}: Awake,

	// 18: RESETTLING → SLEEPING_CRIB
	{Resettling, Settled}: SleepingCrib,
	// 19: RESETTLING → AWAKE
	{Resettling, ResettleFailed}: Awake,
	// 20: RESETTLING → POOP
	{Resettling, PoopStart}: Poop,

	// 21: SLEEPING_CRIB → AWAKE
	{SleepingCrib, BabyWoke}: Awake,
	// 22: SLEEPING_CRIB → POOP
	{SleepingCrib, PoopStart}: Poop,
	// 23: SLEEPING_CRIB → SELF_SOOTHING
	{SleepingCrib, BabyStirred}: SelfSoothing,

	// 24: SELF_SOOTHING → SLEEPING_CRIB
	{SelfSoothing, Settled}: SleepingCrib,
	// 25: SELF_SOOTHING → AWAKE
	{SelfSoothing, BabyWoke}: Awake,
	// 26: SELF_SOOTHING → POOP
	{SelfSoothing, PoopStart}: Poop,

	// 27: STROLLING → SLEEPING_STROLLER
	{Strolling, FellAsleep}: SleepingStroller,
	// 28: STROLLING → AWAKE
	{Strolling, GiveUp}: Awake,
	// 29: STROLLING → POOP
	{Strolling, PoopStart}: Poop,

	// 30: SLEEPING_STROLLER → AWAKE
	{SleepingStroller, BabyWoke}: Awake,
	// 31: SLEEPING_STROLLER → POOP
	{SleepingStroller, PoopStart}: Poop,

	// 32: POOP → AWAKE
	{Poop, PoopDone}: Awake,

	// 33: AWAKE → LEARNING
	{Awake, PutDownAwakeFerber}: Learning,
	// 34: SLEEPING_CRIB → LEARNING
	{SleepingCrib, BabyStirredFerber}: Learning,
	// 35: LEARNING → LEARNING (mood change)
	{Learning, MoodChange}: Learning,
	// 36: LEARNING → CHECK_IN
	{Learning, CheckInStart}: CheckIn,
	// 37: LEARNING → SLEEPING_CRIB
	{Learning, Settled}: SleepingCrib,
	// 38: LEARNING → AWAKE
	{Learning, ExitFerber}: Awake,
	// 39: CHECK_IN → LEARNING
	{CheckIn, EndCheckIn}: Learning,
	// 40: CHECK_IN → SLEEPING_CRIB
	{CheckIn, Settled}: SleepingCrib,
	// 41: CHECK_IN → AWAKE
	{CheckIn, ExitFerber}: Awake,
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

	to, ok := transitions[transitionKey{from, action}]
	if !ok {
		return "", fmt.Errorf("invalid transition: %s -> %s", from, action)
	}

	return to, nil
}

// actionOrder defines the canonical display order for actions.
var actionOrder = func() map[Action]int {
	all := []Action{
		StartNight, StartFeed, DislatchAwake, DislatchAsleep, SwitchBreast,
		StartTransfer, TransferSuccess, TransferNeedResettle, TransferFailed,
		PutDownAwake, PutDownAwakeFerber,
		StartResettle, Settled, ResettleFailed, BabyWoke,
		StartStrolling, FellAsleep, GiveUp,
		BabyStirred, BabyStirredFerber,
		MoodChange, CheckInStart, EndCheckIn, ExitFerber,
		PoopStart, PoopDone, EndNight,
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
// Returns NightOff if the event log is empty.
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
