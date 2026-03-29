package reports

import (
	"testing"
	"time"

	"github.com/polina/boob-o-clock/internal/domain"
)

func t0() time.Time {
	return time.Date(2026, 3, 29, 21, 0, 0, 0, time.Local)
}

func mkEvent(seq int, from domain.State, action domain.Action, to domain.State, ts time.Time, meta map[string]string) domain.Event {
	return domain.Event{
		Seq:       seq,
		FromState: from,
		Action:    action,
		ToState:   to,
		Timestamp: ts,
		Metadata:  meta,
	}
}

func breast(side string) map[string]string {
	return map[string]string{"breast": side}
}

// A realistic night: feed, sleep in crib, wake, feed, sleep.
func realisticNight() ([]domain.Event, time.Time, time.Time) {
	start := t0()
	end := start.Add(9 * time.Hour) // 6am

	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, start, nil),
		// Feed L: 21:00 - 21:20
		mkEvent(2, domain.Awake, domain.StartFeed, domain.Feeding, start, breast("L")),
		mkEvent(3, domain.Feeding, domain.DislatchAsleep, domain.SleepingOnMe, start.Add(20*time.Minute), nil),
		// Transfer at 21:25, success
		mkEvent(4, domain.SleepingOnMe, domain.StartTransfer, domain.Transferring, start.Add(25*time.Minute), nil),
		mkEvent(5, domain.Transferring, domain.TransferSuccess, domain.SleepingCrib, start.Add(25*time.Minute), nil),
		// Sleep 21:25 - 01:00 (3h35m)
		mkEvent(6, domain.SleepingCrib, domain.BabyWoke, domain.Awake, start.Add(4*time.Hour), nil),
		// Feed R: 01:00 - 01:15
		mkEvent(7, domain.Awake, domain.StartFeed, domain.Feeding, start.Add(4*time.Hour), breast("R")),
		mkEvent(8, domain.Feeding, domain.DislatchAsleep, domain.SleepingOnMe, start.Add(4*time.Hour+15*time.Minute), nil),
		// Transfer at 01:20, needs resettle
		mkEvent(9, domain.SleepingOnMe, domain.StartTransfer, domain.Transferring, start.Add(4*time.Hour+20*time.Minute), nil),
		mkEvent(10, domain.Transferring, domain.TransferNeedResettle, domain.Resettling, start.Add(4*time.Hour+20*time.Minute), nil),
		// Settled at 01:30
		mkEvent(11, domain.Resettling, domain.Settled, domain.SleepingCrib, start.Add(4*time.Hour+30*time.Minute), nil),
		// Sleep 01:30 - 06:00 (4h30m)
		mkEvent(12, domain.SleepingCrib, domain.BabyWoke, domain.Awake, end, nil),
		mkEvent(13, domain.Awake, domain.EndNight, domain.NightOff, end, nil),
	}

	return events, start, end
}

func TestTimelineFromEvents(t *testing.T) {
	events, _, _ := realisticNight()

	tl := BuildTimeline(events)
	if len(tl) == 0 {
		t.Fatal("expected non-empty timeline")
	}

	// Check that every entry has a non-negative duration
	for i, entry := range tl {
		if entry.Duration < 0 {
			t.Errorf("entry %d (%s) has negative duration: %v", i, entry.State, entry.Duration)
		}
	}
}

func TestNightStats(t *testing.T) {
	events, start, end := realisticNight()

	stats := ComputeStats(events, start, end)

	// Total night duration: 9 hours
	if stats.NightDuration != 9*time.Hour {
		t.Errorf("NightDuration = %v, want 9h", stats.NightDuration)
	}

	// Feed count: 2
	if stats.FeedCount != 2 {
		t.Errorf("FeedCount = %d, want 2", stats.FeedCount)
	}

	// Total feed time: 20min + 15min = 35min
	if stats.TotalFeedTime != 35*time.Minute {
		t.Errorf("TotalFeedTime = %v, want 35m", stats.TotalFeedTime)
	}

	// Per-breast: L=20min (first feed), R=15min (second feed)
	if stats.FeedTimeLeft != 20*time.Minute {
		t.Errorf("FeedTimeLeft = %v, want 20m", stats.FeedTimeLeft)
	}
	if stats.FeedTimeRight != 15*time.Minute {
		t.Errorf("FeedTimeRight = %v, want 15m", stats.FeedTimeRight)
	}

	// Wake count: 2 (both BabyWoke events: at 01:00 and 06:00)
	if stats.WakeCount != 2 {
		t.Errorf("WakeCount = %d, want 2", stats.WakeCount)
	}

	// Longest sleep block: sleeping_on_me(5m) + transferring(0) + resettling(10m) + sleeping_crib(4h30m) = 4h45m
	// The second sleep block is contiguous from sleeping_on_me through to final crib sleep
	if stats.LongestSleepBlock != 4*time.Hour+45*time.Minute {
		t.Errorf("LongestSleepBlock = %v, want 4h45m", stats.LongestSleepBlock)
	}

	// Total sleep: sleeping_on_me (5min) + sleeping_crib (3h35m) + sleeping_on_me (5min) + resettling (10min) + sleeping_crib (4h30m)
	// sleeping_on_me: 21:00+20m to 21:00+25m = 5min, and 01:15 to 01:20 = 5min
	// sleeping_crib: 21:25 to 01:00 = 3h35m, and 01:30 to 06:00 = 4h30m
	// resettling: 01:20 to 01:30 = 10min
	// Total sleep-ish: 5m + 3h35m + 5m + 10m + 4h30m = 8h25m
	expectedSleep := 5*time.Minute + 3*time.Hour + 35*time.Minute + 5*time.Minute + 10*time.Minute + 4*time.Hour + 30*time.Minute
	if stats.TotalSleepTime != expectedSleep {
		t.Errorf("TotalSleepTime = %v, want %v", stats.TotalSleepTime, expectedSleep)
	}
}

func TestEmptyNightStats(t *testing.T) {
	stats := ComputeStats(nil, t0(), t0())
	if stats.FeedCount != 0 {
		t.Error("empty night should have 0 feeds")
	}
	if stats.WakeCount != 0 {
		t.Error("empty night should have 0 wakes")
	}
}

func TestStatsWithPoop(t *testing.T) {
	start := t0()
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, start, nil),
		mkEvent(2, domain.Awake, domain.PoopStart, domain.Poop, start, nil),
		mkEvent(3, domain.Poop, domain.PoopDone, domain.Awake, start.Add(10*time.Minute), nil),
		mkEvent(4, domain.Awake, domain.EndNight, domain.NightOff, start.Add(10*time.Minute), nil),
	}

	stats := ComputeStats(events, start, start.Add(10*time.Minute))
	if stats.FeedCount != 0 {
		t.Errorf("FeedCount = %d, want 0", stats.FeedCount)
	}
	if stats.WakeCount != 0 {
		t.Errorf("WakeCount = %d, want 0", stats.WakeCount)
	}
}

func TestStatsWithStroller(t *testing.T) {
	start := t0()
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, start, nil),
		mkEvent(2, domain.Awake, domain.StartFeed, domain.Feeding, start, breast("L")),
		mkEvent(3, domain.Feeding, domain.DislatchAwake, domain.Awake, start.Add(15*time.Minute), nil),
		// Crib attempt fails
		mkEvent(4, domain.Awake, domain.StartTransfer, domain.Transferring, start.Add(20*time.Minute), nil),
		mkEvent(5, domain.Transferring, domain.TransferFailed, domain.Awake, start.Add(20*time.Minute), nil),
		// Nuclear option: stroller
		mkEvent(6, domain.Awake, domain.StartStrolling, domain.Strolling, start.Add(25*time.Minute), nil),
		mkEvent(7, domain.Strolling, domain.FellAsleep, domain.SleepingStroller, start.Add(35*time.Minute), nil),
		// Sleep in stroller: 35min to 4h
		mkEvent(8, domain.SleepingStroller, domain.BabyWoke, domain.Awake, start.Add(4*time.Hour), nil),
		mkEvent(9, domain.Awake, domain.EndNight, domain.NightOff, start.Add(4*time.Hour), nil),
	}

	stats := ComputeStats(events, start, start.Add(4*time.Hour))

	if stats.FeedCount != 1 {
		t.Errorf("FeedCount = %d, want 1", stats.FeedCount)
	}
	// Longest sleep block: strolling(10m) + sleeping_stroller(3h25m) = 3h35m
	// Strolling is part of the settling effort, so it's contiguous with the sleep
	if stats.LongestSleepBlock != 3*time.Hour+35*time.Minute {
		t.Errorf("LongestSleepBlock = %v, want 3h35m", stats.LongestSleepBlock)
	}
}

func TestStatsWithSwitchBreast(t *testing.T) {
	start := t0()
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, start, nil),
		mkEvent(2, domain.Awake, domain.StartFeed, domain.Feeding, start, breast("L")),
		mkEvent(3, domain.Feeding, domain.SwitchBreast, domain.Feeding, start.Add(10*time.Minute), breast("R")),
		mkEvent(4, domain.Feeding, domain.DislatchAwake, domain.Awake, start.Add(20*time.Minute), nil),
		mkEvent(5, domain.Awake, domain.EndNight, domain.NightOff, start.Add(20*time.Minute), nil),
	}

	stats := ComputeStats(events, start, start.Add(20*time.Minute))

	// Switch breast is one feed session, not two
	if stats.FeedCount != 1 {
		t.Errorf("FeedCount = %d, want 1 (switch breast is same feed)", stats.FeedCount)
	}
	// Total feed time: full 20 minutes
	if stats.TotalFeedTime != 20*time.Minute {
		t.Errorf("TotalFeedTime = %v, want 20m", stats.TotalFeedTime)
	}
	// Per-breast: L=10min (first half), R=10min (after switch)
	if stats.FeedTimeLeft != 10*time.Minute {
		t.Errorf("FeedTimeLeft = %v, want 10m", stats.FeedTimeLeft)
	}
	if stats.FeedTimeRight != 10*time.Minute {
		t.Errorf("FeedTimeRight = %v, want 10m", stats.FeedTimeRight)
	}
}

func TestTimelineEntries(t *testing.T) {
	start := t0()
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, start, nil),
		mkEvent(2, domain.Awake, domain.StartFeed, domain.Feeding, start.Add(2*time.Minute), breast("L")),
		mkEvent(3, domain.Feeding, domain.DislatchAwake, domain.Awake, start.Add(12*time.Minute), nil),
		mkEvent(4, domain.Awake, domain.EndNight, domain.NightOff, start.Add(15*time.Minute), nil),
	}

	tl := BuildTimeline(events)

	// Should have 3 entries: awake(2m), feeding(10m), awake(3m)
	if len(tl) != 3 {
		t.Fatalf("got %d timeline entries, want 3", len(tl))
	}

	if tl[0].State != domain.Awake || tl[0].Duration != 2*time.Minute {
		t.Errorf("entry 0: state=%s dur=%v, want awake 2m", tl[0].State, tl[0].Duration)
	}
	if tl[1].State != domain.Feeding || tl[1].Duration != 10*time.Minute {
		t.Errorf("entry 1: state=%s dur=%v, want feeding 10m", tl[1].State, tl[1].Duration)
	}
	if tl[2].State != domain.Awake || tl[2].Duration != 3*time.Minute {
		t.Errorf("entry 2: state=%s dur=%v, want awake 3m", tl[2].State, tl[2].Duration)
	}
}
