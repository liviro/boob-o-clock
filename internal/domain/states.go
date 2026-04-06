package domain

import "time"

// State represents the current state of the baby during a night session.
type State string

const (
	NightOff         State = "night_off"
	Awake            State = "awake"
	Feeding          State = "feeding"
	SleepingOnMe     State = "sleeping_on_me"
	Transferring     State = "transferring"
	Resettling       State = "resettling"
	SleepingCrib     State = "sleeping_crib"
	Strolling        State = "strolling"
	SleepingStroller State = "sleeping_stroller"
	SelfSoothing     State = "self_soothing"
	Poop             State = "poop"
)

// AllStates is the complete set of valid states.
var AllStates = []State{
	NightOff, Awake, Feeding, SleepingOnMe, Transferring,
	Resettling, SleepingCrib, Strolling, SleepingStroller, SelfSoothing, Poop,
}

// Action represents a user action that triggers a state transition.
type Action string

const (
	StartNight           Action = "start_night"
	StartFeed            Action = "start_feed"
	DislatchAwake        Action = "dislatch_awake"
	DislatchAsleep       Action = "dislatch_asleep"
	SwitchBreast         Action = "switch_breast"
	StartTransfer        Action = "start_transfer"
	TransferSuccess      Action = "transfer_success"
	TransferNeedResettle Action = "transfer_need_resettle"
	TransferFailed       Action = "transfer_failed"
	StartResettle        Action = "start_resettle"
	Settled              Action = "settled"
	ResettleFailed       Action = "resettle_failed"
	BabyWoke             Action = "baby_woke"
	StartStrolling       Action = "start_strolling"
	FellAsleep           Action = "fell_asleep"
	GiveUp               Action = "give_up"
	PutDownAwake         Action = "put_down_awake"
	BabyStirred          Action = "baby_stirred"
	PoopStart            Action = "poop_start"
	PoopDone             Action = "poop_done"
	EndNight             Action = "end_night"
)

// Breast side for feeding metadata.
type Breast string

const (
	Left  Breast = "L"
	Right Breast = "R"
)

// Event records a single state transition within a night.
type Event struct {
	ID        int64
	NightID   int64
	FromState State
	Action    Action
	ToState   State
	Timestamp time.Time
	Metadata  map[string]string
	CreatedAt time.Time
	Seq       int
}

// Night represents a tracking session from night start to night end.
type Night struct {
	ID        int64
	StartedAt time.Time
	EndedAt   *time.Time
	CreatedAt time.Time
}

