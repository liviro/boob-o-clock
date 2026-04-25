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

func TestDayStatesExist(t *testing.T) {
	wantPresent := []State{DayAwake, DayFeeding, DaySleeping, DayPoop}
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

// TestAllValidTransitions verifies every row of the full transition table
// (night subgraph + cross-boundary + day subgraph).
func TestAllValidTransitions(t *testing.T) {
	tests := []struct {
		name     string
		from     State
		action   Action
		to       State
		metadata map[string]string
	}{
		// --- Night subgraph ---
		{"start night", NightOff, StartNight, Awake, nil},
		{"start feed left", Awake, StartFeed, Feeding, map[string]string{"breast": "L"}},
		{"start feed right", Awake, StartFeed, Feeding, map[string]string{"breast": "R"}},
		{"start resettle", Awake, StartResettle, Resettling, nil},
		{"start strolling", Awake, StartStrolling, Strolling, nil},
		{"poop from awake", Awake, PoopStart, Poop, nil},
		{"put down awake", Awake, PutDownAwake, SelfSoothing, nil},
		{"dislatch awake", Feeding, DislatchAwake, Awake, nil},
		{"dislatch asleep", Feeding, DislatchAsleep, SleepingOnMe, nil},
		{"switch breast", Feeding, SwitchBreast, Feeding, map[string]string{"breast": "R"}},
		{"transfer from sleeping on me", SleepingOnMe, StartTransfer, Transferring, nil},
		{"re-feed from sleeping on me", SleepingOnMe, StartFeed, Feeding, map[string]string{"breast": "L"}},
		{"baby woke from on me", SleepingOnMe, BabyWoke, Awake, nil},
		{"poop from sleeping on me", SleepingOnMe, PoopStart, Poop, nil},
		{"transfer success", Transferring, TransferSuccess, SleepingCrib, nil},
		{"transfer needs resettle", Transferring, TransferNeedResettle, Resettling, nil},
		{"transfer failed", Transferring, TransferFailed, Awake, nil},
		{"settled", Resettling, Settled, SleepingCrib, nil},
		{"resettle failed", Resettling, ResettleFailed, Awake, nil},
		{"poop from resettling", Resettling, PoopStart, Poop, nil},
		{"baby woke from crib", SleepingCrib, BabyWoke, Awake, nil},
		{"poop from crib", SleepingCrib, PoopStart, Poop, nil},
		{"baby stirred in crib", SleepingCrib, BabyStirred, SelfSoothing, nil},
		{"self soothe settled", SelfSoothing, Settled, SleepingCrib, nil},
		{"self soothe failed", SelfSoothing, BabyWoke, Awake, nil},
		{"poop from self soothing", SelfSoothing, PoopStart, Poop, nil},
		{"fell asleep in stroller", Strolling, FellAsleep, SleepingStroller, nil},
		{"give up strolling", Strolling, GiveUp, Awake, nil},
		{"poop from strolling", Strolling, PoopStart, Poop, nil},
		{"baby woke from stroller", SleepingStroller, BabyWoke, Awake, nil},
		{"poop from stroller sleep", SleepingStroller, PoopStart, Poop, nil},
		{"poop done", Poop, PoopDone, Awake, nil},

		// --- Chair sub-machine ---
		{"sit in chair", Awake, SitChair, Chair, nil},
		{"chair settled", Chair, Settled, SleepingCrib, nil},
		{"exit chair", Chair, ExitChair, Awake, nil},

		// --- Cross-boundary chain advances ---
		{"start day from awake (morning)", Awake, StartDay, DayAwake, nil},
		{"start night from day awake (bedtime)", DayAwake, StartNight, Awake, nil},
		{"start day from night_off (first-start in day mode)", NightOff, StartDay, DayAwake, nil},

		// --- Day subgraph ---
		{"day: start feed left", DayAwake, StartFeed, DayFeeding, map[string]string{"breast": "L"}},
		{"day: start feed right", DayAwake, StartFeed, DayFeeding, map[string]string{"breast": "R"}},
		{"day: start sleep in crib", DayAwake, StartSleep, DaySleeping, map[string]string{"location": "crib"}},
		{"day: start sleep on me", DayAwake, StartSleep, DaySleeping, map[string]string{"location": "on_me"}},
		{"day: poop from awake", DayAwake, PoopStart, DayPoop, nil},
		{"day: dislatch awake", DayFeeding, DislatchAwake, DayAwake, nil},
		{"day: dislatch asleep (implicit on_me)", DayFeeding, DislatchAsleep, DaySleeping, nil},
		{"day: switch breast", DayFeeding, SwitchBreast, DayFeeding, map[string]string{"breast": "R"}},
		{"day: poop from feeding", DayFeeding, PoopStart, DayPoop, nil},
		{"day: baby woke from nap", DaySleeping, BabyWoke, DayAwake, nil},
		{"day: poop from nap", DaySleeping, PoopStart, DayPoop, nil},
		{"day: poop done", DayPoop, PoopDone, DayAwake, nil},
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
		{"dislatch from awake", Awake, DislatchAwake},
		{"settle from awake", Awake, Settled},
		{"transfer success from awake", Awake, TransferSuccess},
		{"transfer from awake", Awake, StartTransfer},
		{"start night while awake (night)", Awake, StartNight},
		{"start night while feeding", Feeding, StartNight},
		{"fell asleep from awake", Awake, FellAsleep},
		{"poop from feeding", Feeding, PoopStart},
		{"poop from transferring", Transferring, PoopStart},
		{"poop from night off", NightOff, PoopStart},
		{"switch breast from awake", Awake, SwitchBreast},
		{"give up from crib sleep", SleepingCrib, GiveUp},
		// Chair is reachable only from Awake; baby is awake during chair, so
		// no feed/poop/transfer/etc. directly out of Chair (those go via Awake).
		{"sit_chair from feeding", Feeding, SitChair},
		{"sit_chair from sleeping crib", SleepingCrib, SitChair},
		{"sit_chair from self soothing", SelfSoothing, SitChair},
		{"poop from chair", Chair, PoopStart},
		{"feed from chair", Chair, StartFeed},
		{"transfer from chair", Chair, StartTransfer},
		{"baby woke from chair", Chair, BabyWoke},
		{"exit_chair from awake", Awake, ExitChair},
		// start_day is NOT valid from within a non-AWAKE night state
		{"start day from feeding", Feeding, StartDay},
		{"start day from sleeping crib", SleepingCrib, StartDay},
		// start_night is NOT valid from within a non-DayAwake day state
		{"start night from day feeding", DayFeeding, StartNight},
		{"start night from day sleeping", DaySleeping, StartNight},
		// start_sleep only valid from DayAwake
		{"start sleep from awake (night)", Awake, StartSleep},
		{"start sleep from day feeding", DayFeeding, StartSleep},
		// end_night action is removed post-change; its string "end_night" is
		// invalid in all states.
		{"end_night string is rejected", Awake, Action("end_night")},
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
		// Chain-advance actions sort last in actionOrder, so start_night/start_day
		// appear at the end of validActions (bottom of grid).
		{NightOff, []Action{StartNight, StartDay}},
		{Awake, []Action{StartFeed, PutDownAwake, PutDownAwakeFerber, SitChair, StartResettle, StartStrolling, PoopStart, StartDay}},
		{Feeding, []Action{DislatchAwake, DislatchAsleep, SwitchBreast}},
		{SleepingOnMe, []Action{StartFeed, StartTransfer, BabyWoke, PoopStart}},
		{Transferring, []Action{TransferSuccess, TransferNeedResettle, TransferFailed}},
		{Resettling, []Action{Settled, ResettleFailed, PoopStart}},
		{SleepingCrib, []Action{BabyWoke, BabyStirred, BabyStirredFerber, PoopStart}},
		{SelfSoothing, []Action{Settled, BabyWoke, PoopStart}},
		{Strolling, []Action{FellAsleep, GiveUp, PoopStart}},
		{SleepingStroller, []Action{BabyWoke, PoopStart}},
		{Poop, []Action{PoopDone}},
		{Chair, []Action{Settled, ExitChair}},
		// Day subgraph: start_night is last (chain-advance).
		{DayAwake, []Action{StartFeed, StartSleep, PoopStart, StartNight}},
		{DayFeeding, []Action{DislatchAwake, DislatchAsleep, SwitchBreast, PoopStart}},
		{DaySleeping, []Action{BabyWoke, PoopStart}},
		{DayPoop, []Action{PoopDone}},
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

// TestAwakeNoLongerHasEndNight is a regression test: under contiguous chain,
// end_night is removed entirely. Awake transitions out of the night only via
// start_day (chain advance).
func TestAwakeNoLongerHasEndNight(t *testing.T) {
	for _, a := range ValidActions(Awake) {
		if string(a) == "end_night" {
			t.Errorf("Awake should no longer have end_night; got it in ValidActions")
		}
	}
}

// TestStartFeedRequiresBreast verifies that starting a feed without breast
// metadata fails — applies to both night Feeding and day DayFeeding entries.
func TestStartFeedRequiresBreast(t *testing.T) {
	for _, from := range []State{Awake, DayAwake, SleepingOnMe} {
		t.Run(string(from), func(t *testing.T) {
			if _, err := Transition(from, StartFeed, nil); err == nil {
				t.Errorf("StartFeed from %s with no metadata should require breast", from)
			}
			if _, err := Transition(from, StartFeed, map[string]string{}); err == nil {
				t.Errorf("StartFeed from %s with empty metadata should require breast", from)
			}
			if _, err := Transition(from, StartFeed, map[string]string{"breast": "X"}); err == nil {
				t.Errorf("StartFeed from %s with invalid breast should fail", from)
			}
		})
	}
}

// TestSwitchBreastRequiresBreast verifies switch_breast requires metadata in
// both night and day feeding contexts.
func TestSwitchBreastRequiresBreast(t *testing.T) {
	for _, from := range []State{Feeding, DayFeeding} {
		t.Run(string(from), func(t *testing.T) {
			if _, err := Transition(from, SwitchBreast, nil); err == nil {
				t.Errorf("SwitchBreast from %s with no metadata should require breast", from)
			}
			got, err := Transition(from, SwitchBreast, map[string]string{"breast": "R"})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != from {
				t.Errorf("SwitchBreast should stay in %s, got %s", from, got)
			}
		})
	}
}

// TestStartSleepRequiresLocation covers the new location validator.
func TestStartSleepRequiresLocation(t *testing.T) {
	t.Run("no metadata", func(t *testing.T) {
		if _, err := Transition(DayAwake, StartSleep, nil); err == nil {
			t.Error("StartSleep with no metadata should require location")
		}
	})
	t.Run("missing location key", func(t *testing.T) {
		if _, err := Transition(DayAwake, StartSleep, map[string]string{}); err == nil {
			t.Error("StartSleep with empty metadata should require location")
		}
	})
	t.Run("invalid location value", func(t *testing.T) {
		if _, err := Transition(DayAwake, StartSleep, map[string]string{"location": "couch"}); err == nil {
			t.Error("StartSleep with invalid location should fail")
		}
	})
	t.Run("all four valid locations accepted", func(t *testing.T) {
		for _, loc := range []string{"crib", "stroller", "on_me", "car"} {
			got, err := Transition(DayAwake, StartSleep, map[string]string{"location": loc})
			if err != nil {
				t.Errorf("location %q rejected: %v", loc, err)
			}
			if got != DaySleeping {
				t.Errorf("StartSleep should land in DaySleeping, got %s", got)
			}
		}
	})
}

// TestDeriveState verifies that state is correctly derived from an event log.
func TestDeriveState(t *testing.T) {
	t.Run("empty events returns night off", func(t *testing.T) {
		got := DeriveState(nil)
		if got != NightOff {
			t.Errorf("DeriveState(nil) = %s, want %s", got, NightOff)
		}
	})

	t.Run("returns last event to-state (night)", func(t *testing.T) {
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

	t.Run("returns last event to-state (day)", func(t *testing.T) {
		events := []Event{
			{ToState: DayAwake, Seq: 1},
			{ToState: DayFeeding, Seq: 2},
			{ToState: DaySleeping, Seq: 3},
		}
		got := DeriveState(events)
		if got != DaySleeping {
			t.Errorf("DeriveState = %s, want %s", got, DaySleeping)
		}
	})
}

// TestPoopReachableFromNightStates retains the original "shit happens" design
// for the night subgraph.
func TestPoopReachableFromNightStates(t *testing.T) {
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

// TestPoopReachableFromDayStates covers the day subgraph. Unlike night, day
// FEEDING IS a valid poop source (babies poop during daytime feeds; parent
// can pause and change).
func TestPoopReachableFromDayStates(t *testing.T) {
	poopStates := []State{DayAwake, DayFeeding, DaySleeping}
	noPoopStates := []State{DayPoop}

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

// TestEveryStateCanReachHub verifies no dead ends. Every non-chain-off state
// must reach either Awake or DayAwake (the hubs) within a few hops; chain
// advance from there closes the session.
func TestEveryStateCanReachHub(t *testing.T) {
	hubs := map[State]bool{Awake: true, DayAwake: true}

	for _, state := range AllStates {
		if state == NightOff || hubs[state] {
			continue
		}

		visited := map[State]bool{state: true}
		queue := []State{state}
		found := false

		for depth := 0; depth < 5 && len(queue) > 0 && !found; depth++ {
			var next []State
			for _, s := range queue {
				for _, a := range ValidActions(s) {
					meta := map[string]string{}
					if a == StartFeed || a == SwitchBreast {
						meta["breast"] = "L"
					}
					if a == StartSleep {
						meta["location"] = "crib"
					}
					if a == PutDownAwakeFerber || a == BabyStirredFerber || a == MoodChange || a == EndCheckIn {
						meta["mood"] = "quiet"
					}
					ns, err := Transition(s, a, meta)
					if err != nil {
						continue
					}
					if hubs[ns] {
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
			t.Errorf("state %s cannot reach a hub (Awake or DayAwake) within 5 hops", state)
		}
	}
}

// TestDayStatesCanReachAwakeViaChainAdvance verifies that the day subgraph is
// connected to the night subgraph through DayAwake → Awake (chain advance),
// so end-of-cycle reporting works across the boundary.
func TestDayStatesCanReachAwakeViaChainAdvance(t *testing.T) {
	// From DayAwake: one hop.
	got, err := Transition(DayAwake, StartNight, nil)
	if err != nil || got != Awake {
		t.Fatalf("DayAwake → Awake via start_night failed: got=%s err=%v", got, err)
	}

	// Every other day state reaches DayAwake within a few hops, then Awake.
	for _, s := range []State{DayFeeding, DaySleeping, DayPoop} {
		visited := map[State]bool{s: true}
		queue := []State{s}
		reachedDayAwake := false

		for depth := 0; depth < 3 && len(queue) > 0 && !reachedDayAwake; depth++ {
			var next []State
			for _, st := range queue {
				for _, a := range ValidActions(st) {
					meta := map[string]string{}
					if a == StartFeed || a == SwitchBreast {
						meta["breast"] = "L"
					}
					ns, err := Transition(st, a, meta)
					if err != nil {
						continue
					}
					if ns == DayAwake {
						reachedDayAwake = true
						break
					}
					if !visited[ns] {
						visited[ns] = true
						next = append(next, ns)
					}
				}
				if reachedDayAwake {
					break
				}
			}
			queue = next
		}

		if !reachedDayAwake {
			t.Errorf("day state %s cannot reach DayAwake within 3 hops", s)
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
