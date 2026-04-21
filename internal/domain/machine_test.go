package domain

import (
	"testing"
)

func TestLearningAndCheckInStatesExist(t *testing.T) {
	wantPresent := []State{Learning, CheckIn}
	present := make(map[State]bool, len(AllStates))
	for _, s := range AllStates {
		present[s] = true
	}
	for _, s := range wantPresent {
		if !present[s] {
			t.Errorf("state %q missing from AllStates", s)
		}
	}
}

// TestAllValidTransitions verifies every row of the 32-transition table.
func TestAllValidTransitions(t *testing.T) {
	tests := []struct {
		name     string
		from     State
		action   Action
		to       State
		metadata map[string]string
	}{
		// Row 1: NIGHT_OFF → AWAKE
		{"start night", NightOff, StartNight, Awake, nil},
		// Row 2: AWAKE → FEEDING
		{"start feed left", Awake, StartFeed, Feeding, map[string]string{"breast": "L"}},
		{"start feed right", Awake, StartFeed, Feeding, map[string]string{"breast": "R"}},
		// Row 3: AWAKE → RESETTLING
		{"start resettle", Awake, StartResettle, Resettling, nil},
		// Row 4: AWAKE → STROLLING
		{"start strolling", Awake, StartStrolling, Strolling, nil},
		// Row 5: AWAKE → POOP
		{"poop from awake", Awake, PoopStart, Poop, nil},
		// Row 6: AWAKE → SELF_SOOTHING
		{"put down awake", Awake, PutDownAwake, SelfSoothing, nil},
		// Row 7: AWAKE → NIGHT_OFF
		{"end night", Awake, EndNight, NightOff, nil},
		// Row 8: FEEDING → AWAKE
		{"dislatch awake", Feeding, DislatchAwake, Awake, nil},
		// Row 9: FEEDING → SLEEPING_ON_ME
		{"dislatch asleep", Feeding, DislatchAsleep, SleepingOnMe, nil},
		// Row 10: FEEDING → FEEDING (switch breast)
		{"switch breast", Feeding, SwitchBreast, Feeding, map[string]string{"breast": "R"}},
		// Row 11: SLEEPING_ON_ME → TRANSFERRING
		{"transfer from sleeping on me", SleepingOnMe, StartTransfer, Transferring, nil},
		// Row 12: SLEEPING_ON_ME → FEEDING
		{"re-feed from sleeping on me", SleepingOnMe, StartFeed, Feeding, map[string]string{"breast": "L"}},
		// Row 13: SLEEPING_ON_ME → AWAKE
		{"baby woke from on me", SleepingOnMe, BabyWoke, Awake, nil},
		// Row 14: SLEEPING_ON_ME → POOP
		{"poop from sleeping on me", SleepingOnMe, PoopStart, Poop, nil},
		// Row 15: TRANSFERRING → SLEEPING_CRIB
		{"transfer success", Transferring, TransferSuccess, SleepingCrib, nil},
		// Row 16: TRANSFERRING → RESETTLING
		{"transfer needs resettle", Transferring, TransferNeedResettle, Resettling, nil},
		// Row 17: TRANSFERRING → AWAKE
		{"transfer failed", Transferring, TransferFailed, Awake, nil},
		// Row 18: RESETTLING → SLEEPING_CRIB
		{"settled", Resettling, Settled, SleepingCrib, nil},
		// Row 19: RESETTLING → AWAKE
		{"resettle failed", Resettling, ResettleFailed, Awake, nil},
		// Row 20: RESETTLING → POOP
		{"poop from resettling", Resettling, PoopStart, Poop, nil},
		// Row 21: SLEEPING_CRIB → AWAKE
		{"baby woke from crib", SleepingCrib, BabyWoke, Awake, nil},
		// Row 22: SLEEPING_CRIB → POOP
		{"poop from crib", SleepingCrib, PoopStart, Poop, nil},
		// Row 23: SLEEPING_CRIB → SELF_SOOTHING
		{"baby stirred in crib", SleepingCrib, BabyStirred, SelfSoothing, nil},
		// Row 24: SELF_SOOTHING → SLEEPING_CRIB
		{"self soothe settled", SelfSoothing, Settled, SleepingCrib, nil},
		// Row 25: SELF_SOOTHING → AWAKE
		{"self soothe failed", SelfSoothing, BabyWoke, Awake, nil},
		// Row 26: SELF_SOOTHING → POOP
		{"poop from self soothing", SelfSoothing, PoopStart, Poop, nil},
		// Row 27: STROLLING → SLEEPING_STROLLER
		{"fell asleep in stroller", Strolling, FellAsleep, SleepingStroller, nil},
		// Row 28: STROLLING → AWAKE
		{"give up strolling", Strolling, GiveUp, Awake, nil},
		// Row 29: STROLLING → POOP
		{"poop from strolling", Strolling, PoopStart, Poop, nil},
		// Row 30: SLEEPING_STROLLER → AWAKE
		{"baby woke from stroller", SleepingStroller, BabyWoke, Awake, nil},
		// Row 31: SLEEPING_STROLLER → POOP
		{"poop from stroller sleep", SleepingStroller, PoopStart, Poop, nil},
		// Row 32: POOP → AWAKE
		{"poop done", Poop, PoopDone, Awake, nil},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := Transition(tt.from, tt.action, tt.metadata)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.to {
				t.Errorf("Transition(%s, %s) = %s, want %s", tt.from, tt.action, got, tt.to)
			}
		})
	}
}

// TestInvalidTransitions verifies that impossible transitions are rejected.
func TestInvalidTransitions(t *testing.T) {
	tests := []struct {
		name   string
		from   State
		action Action
	}{
		{"feed from night off", NightOff, StartFeed},
		{"end night from night off", NightOff, EndNight},
		{"dislatch from awake", Awake, DislatchAwake},
		{"settle from awake", Awake, Settled},
		{"transfer success from awake", Awake, TransferSuccess},
		{"transfer from awake", Awake, StartTransfer},
		{"start night while awake", Awake, StartNight},
		{"start night while feeding", Feeding, StartNight},
		{"end night from feeding", Feeding, EndNight},
		{"end night from sleeping crib", SleepingCrib, EndNight},
		{"end night from sleeping stroller", SleepingStroller, EndNight},
		{"fell asleep from awake", Awake, FellAsleep},
		{"poop from feeding", Feeding, PoopStart},
		{"poop from transferring", Transferring, PoopStart},
		{"poop from night off", NightOff, PoopStart},
		{"switch breast from awake", Awake, SwitchBreast},
		{"give up from crib sleep", SleepingCrib, GiveUp},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := Transition(tt.from, tt.action, nil)
			if err == nil {
				t.Errorf("Transition(%s, %s) should have returned error", tt.from, tt.action)
			}
		})
	}
}

// TestValidActions verifies that each state reports the correct set of valid actions.
func TestValidActions(t *testing.T) {
	tests := []struct {
		state   State
		actions []Action
	}{
		{NightOff, []Action{StartNight}},
		{Awake, []Action{StartFeed, PutDownAwake, PutDownAwakeFerber, StartResettle, StartStrolling, PoopStart, EndNight}},
		{Feeding, []Action{DislatchAwake, DislatchAsleep, SwitchBreast}},
		{SleepingOnMe, []Action{StartFeed, StartTransfer, BabyWoke, PoopStart}},
		{Transferring, []Action{TransferSuccess, TransferNeedResettle, TransferFailed}},
		{Resettling, []Action{Settled, ResettleFailed, PoopStart}},
		{SleepingCrib, []Action{BabyWoke, BabyStirred, BabyStirredFerber, PoopStart}},
		{SelfSoothing, []Action{Settled, BabyWoke, PoopStart}},
		{Strolling, []Action{FellAsleep, GiveUp, PoopStart}},
		{SleepingStroller, []Action{BabyWoke, PoopStart}},
		{Poop, []Action{PoopDone}},
	}

	for _, tt := range tests {
		t.Run(string(tt.state), func(t *testing.T) {
			got := ValidActions(tt.state)
			if len(got) != len(tt.actions) {
				t.Fatalf("ValidActions(%s) = %v, want %v", tt.state, got, tt.actions)
			}
			for i := range got {
				if got[i] != tt.actions[i] {
					t.Fatalf("ValidActions(%s) = %v, want %v", tt.state, got, tt.actions)
				}
			}
		})
	}
}

// TestStartFeedRequiresBreast verifies that starting a feed without breast metadata fails.
func TestStartFeedRequiresBreast(t *testing.T) {
	_, err := Transition(Awake, StartFeed, nil)
	if err == nil {
		t.Error("StartFeed with no metadata should require breast")
	}

	_, err = Transition(Awake, StartFeed, map[string]string{})
	if err == nil {
		t.Error("StartFeed with empty metadata should require breast")
	}

	_, err = Transition(Awake, StartFeed, map[string]string{"breast": "X"})
	if err == nil {
		t.Error("StartFeed with invalid breast should fail")
	}
}

// TestSwitchBreastRequiresBreast verifies that switching breast requires the new side.
func TestSwitchBreastRequiresBreast(t *testing.T) {
	_, err := Transition(Feeding, SwitchBreast, nil)
	if err == nil {
		t.Error("SwitchBreast with no metadata should require breast")
	}

	got, err := Transition(Feeding, SwitchBreast, map[string]string{"breast": "R"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got != Feeding {
		t.Errorf("SwitchBreast should stay in Feeding, got %s", got)
	}
}

// TestReFeedFromSleepingOnMeRequiresBreast verifies breast metadata on re-feed.
func TestReFeedFromSleepingOnMeRequiresBreast(t *testing.T) {
	_, err := Transition(SleepingOnMe, StartFeed, nil)
	if err == nil {
		t.Error("StartFeed from SleepingOnMe with no metadata should require breast")
	}
}

// TestDeriveState verifies that state is correctly derived from an event log.
func TestDeriveState(t *testing.T) {
	t.Run("empty events returns night off", func(t *testing.T) {
		got := DeriveState(nil)
		if got != NightOff {
			t.Errorf("DeriveState(nil) = %s, want %s", got, NightOff)
		}
	})

	t.Run("returns last event to-state", func(t *testing.T) {
		events := []Event{
			{ToState: Awake, Seq: 1},
			{ToState: Feeding, Seq: 2},
			{ToState: SleepingOnMe, Seq: 3},
		}
		got := DeriveState(events)
		if got != SleepingOnMe {
			t.Errorf("DeriveState = %s, want %s", got, SleepingOnMe)
		}
	})
}

// TestPoopReachableFromExactlySevenStates confirms the "shit happens" design.
func TestPoopReachableFromExactlySevenStates(t *testing.T) {
	poopStates := []State{Awake, SleepingOnMe, Resettling, SleepingCrib, Strolling, SleepingStroller, SelfSoothing}
	noPoopStates := []State{NightOff, Feeding, Transferring, Poop}

	for _, s := range poopStates {
		_, err := Transition(s, PoopStart, nil)
		if err != nil {
			t.Errorf("PoopStart should be valid from %s, got error: %v", s, err)
		}
	}

	for _, s := range noPoopStates {
		_, err := Transition(s, PoopStart, nil)
		if err == nil {
			t.Errorf("PoopStart should NOT be valid from %s", s)
		}
	}
}

// TestActionOrderCoversAllActions verifies that every action in the transition
// table has an entry in actionOrder, so new actions can't silently mis-sort.
func TestActionOrderCoversAllActions(t *testing.T) {
	seen := make(map[Action]bool)
	for key := range transitions {
		seen[key.Action] = true
	}
	for action := range seen {
		if _, ok := actionOrder[action]; !ok {
			t.Errorf("action %s is in the transition table but missing from actionOrder", action)
		}
	}
}

// TestEveryStateCanReachNightOff verifies no dead ends in the state machine.
// Every state should reach AWAKE within 2 hops (then AWAKE → NIGHT_OFF).
func TestEveryStateCanReachNightOff(t *testing.T) {
	for _, state := range AllStates {
		if state == NightOff {
			continue
		}

		// BFS to find path to NightOff within reasonable depth
		visited := map[State]bool{state: true}
		queue := []State{state}
		found := false

		for depth := 0; depth < 5 && len(queue) > 0 && !found; depth++ {
			var next []State
			for _, s := range queue {
				for _, a := range ValidActions(s) {
					// Use dummy metadata for actions that need it
					meta := map[string]string{}
					if a == StartFeed || a == SwitchBreast {
						meta["breast"] = "L"
					}
					ns, err := Transition(s, a, meta)
					if err != nil {
						continue
					}
					if ns == NightOff {
						found = true
						break
					}
					if !visited[ns] {
						visited[ns] = true
						next = append(next, ns)
					}
				}
				if found {
					break
				}
			}
			queue = next
		}

		if !found {
			t.Errorf("state %s cannot reach NightOff within 5 hops", state)
		}
	}
}

func TestFerberActionsExist(t *testing.T) {
	// Referencing the constants ensures they exist (any typo = compile error).
	_ = PutDownAwakeFerber
	_ = BabyStirredFerber
	_ = MoodChange
	_ = CheckInStart
	_ = EndCheckIn
	_ = ExitFerber
}

func TestFerberTransitions(t *testing.T) {
	mood := map[string]string{"mood": "quiet"}
	cases := []struct {
		name     string
		from     State
		action   Action
		meta     map[string]string
		expected State
	}{
		{"put down awake (ferber)", Awake, PutDownAwakeFerber, mood, Learning},
		{"baby stirred (ferber)", SleepingCrib, BabyStirredFerber, mood, Learning},
		{"mood change", Learning, MoodChange, map[string]string{"mood": "crying"}, Learning},
		{"check in", Learning, CheckInStart, nil, CheckIn},
		{"settled from learning", Learning, Settled, nil, SleepingCrib},
		{"exit ferber from learning", Learning, ExitFerber, nil, Awake},
		{"end check in", CheckIn, EndCheckIn, mood, Learning},
		{"settled from check in", CheckIn, Settled, nil, SleepingCrib},
		{"exit ferber from check in", CheckIn, ExitFerber, nil, Awake},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, err := Transition(c.from, c.action, c.meta)
			if err != nil {
				t.Fatalf("Transition(%s, %s): %v", c.from, c.action, err)
			}
			if got != c.expected {
				t.Errorf("Transition(%s, %s) = %s, want %s", c.from, c.action, got, c.expected)
			}
		})
	}
}

func TestFerberForbiddenTransitions(t *testing.T) {
	// Explicitly-excluded transitions (spec §3.5).
	cases := []struct {
		name   string
		from   State
		action Action
	}{
		{"no poop from learning", Learning, PoopStart},
		{"no poop from check-in", CheckIn, PoopStart},
		{"no baby_woke from learning", Learning, BabyWoke},
		{"no feed from learning", Learning, StartFeed},
		{"no feed from check-in", CheckIn, StartFeed},
		{"no check-in from self-soothing", SelfSoothing, CheckInStart},
		{"no mood change from self-soothing", SelfSoothing, MoodChange},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			_, err := Transition(c.from, c.action, map[string]string{"mood": "quiet", "breast": "L"})
			if err == nil {
				t.Errorf("expected error for %s -> %s, got none", c.from, c.action)
			}
		})
	}
}

func TestMoodValidation(t *testing.T) {
	t.Run("missing mood on put_down_awake_ferber", func(t *testing.T) {
		_, err := Transition(Awake, PutDownAwakeFerber, nil)
		if err == nil {
			t.Error("expected mood-required error, got none")
		}
	})
	t.Run("missing mood on mood_change", func(t *testing.T) {
		_, err := Transition(Learning, MoodChange, map[string]string{})
		if err == nil {
			t.Error("expected mood-required error, got none")
		}
	})
	t.Run("invalid mood value", func(t *testing.T) {
		_, err := Transition(Awake, PutDownAwakeFerber, map[string]string{"mood": "angry"})
		if err == nil {
			t.Error("expected invalid-mood error, got none")
		}
	})
	t.Run("all three valid moods accepted", func(t *testing.T) {
		for _, m := range []string{"quiet", "fussy", "crying"} {
			if _, err := Transition(Awake, PutDownAwakeFerber, map[string]string{"mood": m}); err != nil {
				t.Errorf("mood %q rejected: %v", m, err)
			}
		}
	})
	t.Run("check_in does not require mood", func(t *testing.T) {
		if _, err := Transition(Learning, CheckInStart, nil); err != nil {
			t.Errorf("check_in should not require mood: %v", err)
		}
	})
	t.Run("end_check_in requires mood", func(t *testing.T) {
		if _, err := Transition(CheckIn, EndCheckIn, nil); err == nil {
			t.Error("expected mood-required error on end_check_in, got none")
		}
	})
}
