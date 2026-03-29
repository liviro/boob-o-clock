package domain

import "fmt"

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
	// 3: AWAKE → TRANSFERRING
	{Awake, StartTransfer}: Transferring,
	// 4: AWAKE → RESETTLING
	{Awake, StartResettle}: Resettling,
	// 5: AWAKE → STROLLING
	{Awake, StartStrolling}: Strolling,
	// 6: AWAKE → POOP
	{Awake, PoopStart}: Poop,
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

	// 23: STROLLING → SLEEPING_STROLLER
	{Strolling, FellAsleep}: SleepingStroller,
	// 24: STROLLING → AWAKE
	{Strolling, GiveUp}: Awake,
	// 25: STROLLING → POOP
	{Strolling, PoopStart}: Poop,

	// 26: SLEEPING_STROLLER → AWAKE
	{SleepingStroller, BabyWoke}: Awake,
	// 27: SLEEPING_STROLLER → POOP
	{SleepingStroller, PoopStart}: Poop,

	// 28: POOP → AWAKE
	{Poop, PoopDone}: Awake,
}

// actionsRequiringBreast is the set of actions that need breast metadata.
var actionsRequiringBreast = map[Action]bool{
	StartFeed:    true,
	SwitchBreast: true,
}

// Transition attempts a state transition and returns the new state.
// Returns an error if the transition is invalid or required metadata is missing.
func Transition(from State, action Action, metadata map[string]string) (State, error) {
	if actionsRequiringBreast[action] {
		if err := validateBreast(metadata); err != nil {
			return "", err
		}
	}

	to, ok := transitions[transitionKey{from, action}]
	if !ok {
		return "", fmt.Errorf("invalid transition: %s -> %s", from, action)
	}

	return to, nil
}

// ValidActions returns all actions available from the given state.
func ValidActions(state State) []Action {
	var actions []Action
	for key := range transitions {
		if key.From == state {
			actions = append(actions, key.Action)
		}
	}
	return actions
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
