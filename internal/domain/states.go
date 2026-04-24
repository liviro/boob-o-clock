package domain

import "time"

// State represents the current state of the baby during a tracked session
// (either a night or a day).
type State string

const (
	// Chain-off: the one pseudo-state where no session is active. Reached on
	// first-ever-start and post-migration from historical end_night rows.
	NightOff State = "night_off"

	// Night subgraph.
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
	Learning         State = "learning"
	CheckIn          State = "check_in"

	// Day subgraph.
	DayAwake    State = "day_awake"
	DayFeeding  State = "day_feeding"
	DaySleeping State = "day_sleeping"
	DayPoop     State = "day_poop"
)

// AllStates is the complete set of valid states.
var AllStates = []State{
	NightOff,
	Awake, Feeding, SleepingOnMe, Transferring,
	Resettling, SleepingCrib, Strolling, SleepingStroller, SelfSoothing, Poop,
	Learning, CheckIn,
	DayAwake, DayFeeding, DaySleeping, DayPoop,
}

// SleepingStates: states where the baby is asleep (night subgraph + day nap).
// Source of truth for cross-layer sleep checks.
var SleepingStates = []State{SleepingCrib, SleepingOnMe, SleepingStroller, DaySleeping}

// Action represents a user action that triggers a state transition.
type Action string

const (
	// Session-creation actions (routed through POST /api/session/start).
	StartNight Action = "start_night"
	StartDay   Action = "start_day"

	// Feeding actions (shared between night Feeding and day DayFeeding).
	StartFeed      Action = "start_feed"
	DislatchAwake  Action = "dislatch_awake"
	DislatchAsleep Action = "dislatch_asleep"
	SwitchBreast   Action = "switch_breast"

	// Night-specific actions.
	StartTransfer        Action = "start_transfer"
	TransferSuccess      Action = "transfer_success"
	TransferNeedResettle Action = "transfer_need_resettle"
	TransferFailed       Action = "transfer_failed"
	StartResettle        Action = "start_resettle"
	Settled              Action = "settled"
	ResettleFailed       Action = "resettle_failed"
	StartStrolling       Action = "start_strolling"
	FellAsleep           Action = "fell_asleep"
	GiveUp               Action = "give_up"
	PutDownAwake         Action = "put_down_awake"
	BabyStirred          Action = "baby_stirred"

	// Day-specific action.
	StartSleep Action = "start_sleep"

	// Shared wake / poop actions.
	BabyWoke  Action = "baby_woke"
	PoopStart Action = "poop_start"
	PoopDone  Action = "poop_done"

	// Ferber actions.
	PutDownAwakeFerber Action = "put_down_awake_ferber"
	BabyStirredFerber  Action = "baby_stirred_ferber"
	MoodChange         Action = "mood_change"
	CheckInStart       Action = "check_in" // identifier differs from value to avoid clash with State CheckIn
	EndCheckIn         Action = "end_check_in"
	ExitFerber         Action = "exit_ferber"
)

// Breast side for feeding metadata.
type Breast string

const (
	Left  Breast = "L"
	Right Breast = "R"
)

// Location for nap metadata (day-session start_sleep action).
type Location string

const (
	LocationCrib     Location = "crib"
	LocationStroller Location = "stroller"
	LocationOnMe     Location = "on_me"
	LocationCar      Location = "car"
)

// SessionKind distinguishes night sessions from day sessions.
type SessionKind string

const (
	SessionKindNight SessionKind = "night"
	SessionKindDay   SessionKind = "day"
)

// Event records a single state transition within a session.
type Event struct {
	ID        int64
	SessionID int64
	FromState State
	Action    Action
	ToState   State
	Timestamp time.Time
	Metadata  map[string]string
	CreatedAt time.Time
	Seq       int
}

// Session represents a tracking period — either a night or a day — from its
// first event (start_night or start_day) to closure via ended_at.
type Session struct {
	ID                int64
	Kind              SessionKind
	StartedAt         time.Time
	EndedAt           *time.Time
	CreatedAt         time.Time
	FerberEnabled     bool
	FerberNightNumber *int // nil when Kind != Night or Ferber not enabled
}

// IsNight reports whether this is a night session.
func (s *Session) IsNight() bool { return s.Kind == SessionKindNight }

// IsDay reports whether this is a day session.
func (s *Session) IsDay() bool { return s.Kind == SessionKindDay }
