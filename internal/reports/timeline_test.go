package reports

import (
	"testing"
	"time"

	"github.com/liviro/boob-o-clock/internal/domain"
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
		mkEvent(13, domain.Awake, domain.Action("end_night"), domain.NightOff, end, nil),
	}

	return events, start, end
}

func TestTimelineFromEvents(t *testing.T) {
	events, _, end := realisticNight()

	tl := BuildTimeline(events, end)
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

	stats, _ := computeBaseStats(events, start, end)

	// Total night duration: 9 hours
	if stats.NightDuration != 9*time.Hour {
		t.Errorf("NightDuration = %v, want 9h", stats.NightDuration)
	}

	// Feed count: 1 (only the feed after first crib sleep; initial feed is excluded)
	if stats.FeedCount != 1 {
		t.Errorf("FeedCount = %d, want 1", stats.FeedCount)
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

	// Feed times: only the 01:00 feed (pre-sleep feed excluded)
	if len(stats.FeedTimes) != 1 {
		t.Fatalf("FeedTimes count = %d, want 1", len(stats.FeedTimes))
	}
	if !stats.FeedTimes[0].Equal(start.Add(4 * time.Hour)) {
		t.Errorf("FeedTimes[0] = %v, want %v", stats.FeedTimes[0], start.Add(4*time.Hour))
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

	// Sleep blocks: first block (on_me 5m + crib 3h35m = 3h40m), second block (on_me 5m + resettling 10m + crib 4h30m = 4h45m)
	if len(stats.SleepBlocks) != 2 {
		t.Fatalf("SleepBlocks count = %d, want 2", len(stats.SleepBlocks))
	}
	if stats.SleepBlocks[0] != 3*time.Hour+40*time.Minute {
		t.Errorf("SleepBlocks[0] = %v, want 3h40m", stats.SleepBlocks[0])
	}
	if stats.SleepBlocks[1] != 4*time.Hour+45*time.Minute {
		t.Errorf("SleepBlocks[1] = %v, want 4h45m", stats.SleepBlocks[1])
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
	stats, _ := computeBaseStats(nil, t0(), t0())
	if stats.FeedCount != 0 {
		t.Error("empty night should have 0 feeds")
	}
	if len(stats.FeedTimes) != 0 {
		t.Errorf("empty night should have 0 feed times, got %d", len(stats.FeedTimes))
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
		mkEvent(4, domain.Awake, domain.Action("end_night"), domain.NightOff, start.Add(10*time.Minute), nil),
	}

	stats, _ := computeBaseStats(events, start, start.Add(10*time.Minute))
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
		mkEvent(9, domain.Awake, domain.Action("end_night"), domain.NightOff, start.Add(4*time.Hour), nil),
	}

	stats, _ := computeBaseStats(events, start, start.Add(4*time.Hour))

	// Feed count: 0 (the only feed happens before first stroller sleep)
	if stats.FeedCount != 0 {
		t.Errorf("FeedCount = %d, want 0", stats.FeedCount)
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
		mkEvent(5, domain.Awake, domain.Action("end_night"), domain.NightOff, start.Add(20*time.Minute), nil),
	}

	stats, _ := computeBaseStats(events, start, start.Add(20*time.Minute))

	// No real sleep in this night, so feed count is 0
	if stats.FeedCount != 0 {
		t.Errorf("FeedCount = %d, want 0 (no real sleep, feeds excluded)", stats.FeedCount)
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

func TestStatsWithRootBack(t *testing.T) {
	start := t0()
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, start, nil),
		// Feed L: 21:00 - 21:15
		mkEvent(2, domain.Awake, domain.StartFeed, domain.Feeding, start, breast("L")),
		mkEvent(3, domain.Feeding, domain.DislatchAsleep, domain.SleepingOnMe, start.Add(15*time.Minute), nil),
		// Baby roots back to breast at 21:20 — same feed session
		mkEvent(4, domain.SleepingOnMe, domain.StartFeed, domain.Feeding, start.Add(20*time.Minute), breast("L")),
		mkEvent(5, domain.Feeding, domain.DislatchAsleep, domain.SleepingOnMe, start.Add(30*time.Minute), nil),
		// Transfer and sleep
		mkEvent(6, domain.SleepingOnMe, domain.StartTransfer, domain.Transferring, start.Add(35*time.Minute), nil),
		mkEvent(7, domain.Transferring, domain.TransferSuccess, domain.SleepingCrib, start.Add(35*time.Minute), nil),
		mkEvent(8, domain.SleepingCrib, domain.BabyWoke, domain.Awake, start.Add(4*time.Hour), nil),
		// New feed after waking — this IS a new feed
		mkEvent(9, domain.Awake, domain.StartFeed, domain.Feeding, start.Add(4*time.Hour), breast("R")),
		mkEvent(10, domain.Feeding, domain.DislatchAsleep, domain.SleepingOnMe, start.Add(4*time.Hour+15*time.Minute), nil),
		mkEvent(11, domain.SleepingOnMe, domain.BabyWoke, domain.Awake, start.Add(8*time.Hour), nil),
		mkEvent(12, domain.Awake, domain.Action("end_night"), domain.NightOff, start.Add(8*time.Hour), nil),
	}

	stats, _ := computeBaseStats(events, start, start.Add(8*time.Hour))

	// Only the feed after first crib sleep counts; pre-sleep feeds excluded
	if stats.FeedCount != 1 {
		t.Errorf("FeedCount = %d, want 1 (pre-sleep feeds excluded)", stats.FeedCount)
	}
	if len(stats.FeedTimes) != 1 {
		t.Fatalf("FeedTimes count = %d, want 1", len(stats.FeedTimes))
	}
	if !stats.FeedTimes[0].Equal(start.Add(4 * time.Hour)) {
		t.Errorf("FeedTimes[0] = %v, want %v", stats.FeedTimes[0], start.Add(4*time.Hour))
	}

	// Total feed time: 15m + 10m + 15m = 40m
	if stats.TotalFeedTime != 40*time.Minute {
		t.Errorf("TotalFeedTime = %v, want 40m", stats.TotalFeedTime)
	}
}

func TestFeedCountNoRealSleep(t *testing.T) {
	// If baby never makes it to crib/stroller, feed count is 0
	start := t0()
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, start, nil),
		mkEvent(2, domain.Awake, domain.StartFeed, domain.Feeding, start, breast("L")),
		mkEvent(3, domain.Feeding, domain.DislatchAsleep, domain.SleepingOnMe, start.Add(20*time.Minute), nil),
		mkEvent(4, domain.SleepingOnMe, domain.BabyWoke, domain.Awake, start.Add(2*time.Hour), nil),
		mkEvent(5, domain.Awake, domain.Action("end_night"), domain.NightOff, start.Add(2*time.Hour), nil),
	}

	stats, _ := computeBaseStats(events, start, start.Add(2*time.Hour))

	if stats.FeedCount != 0 {
		t.Errorf("FeedCount = %d, want 0 (no real sleep, no feeds counted)", stats.FeedCount)
	}
}

func TestSleepBlocksExcludeOnMeOnly(t *testing.T) {
	// Baby falls asleep on breast, brief on-me sleep, then wakes on transfer
	// and is fed again. The on-me-only interval should NOT count as a sleep block.
	start := t0()
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, start, nil),
		// Feed R
		mkEvent(2, domain.Awake, domain.StartFeed, domain.Feeding, start, breast("R")),
		mkEvent(3, domain.Feeding, domain.DislatchAsleep, domain.SleepingOnMe, start.Add(25*time.Minute), nil),
		// Transfer fails
		mkEvent(4, domain.SleepingOnMe, domain.StartTransfer, domain.Transferring, start.Add(31*time.Minute), nil),
		mkEvent(5, domain.Transferring, domain.TransferFailed, domain.Awake, start.Add(33*time.Minute), nil),
		// Feed L, dislatch asleep, transfer succeeds
		mkEvent(6, domain.Awake, domain.StartFeed, domain.Feeding, start.Add(33*time.Minute), breast("L")),
		mkEvent(7, domain.Feeding, domain.DislatchAsleep, domain.SleepingOnMe, start.Add(55*time.Minute), nil),
		mkEvent(8, domain.SleepingOnMe, domain.StartTransfer, domain.Transferring, start.Add(1*time.Hour+2*time.Minute), nil),
		mkEvent(9, domain.Transferring, domain.TransferSuccess, domain.SleepingCrib, start.Add(1*time.Hour+2*time.Minute), nil),
		// Sleep in crib until wake
		mkEvent(10, domain.SleepingCrib, domain.BabyWoke, domain.Awake, start.Add(3*time.Hour), nil),
		mkEvent(11, domain.Awake, domain.Action("end_night"), domain.NightOff, start.Add(3*time.Hour), nil),
	}

	stats, _ := computeBaseStats(events, start, start.Add(3*time.Hour))

	// Only 1 sleep block: the one containing crib sleep.
	// The first on-me-only interval (6m sleeping_on_me + transfer) should be excluded.
	if len(stats.SleepBlocks) != 1 {
		t.Fatalf("SleepBlocks count = %d, want 1 (on-me-only block excluded)", len(stats.SleepBlocks))
	}
	// Block: on_me(7m) + crib(1h58m) = 2h5m
	if stats.SleepBlocks[0] != 2*time.Hour+5*time.Minute {
		t.Errorf("SleepBlocks[0] = %v, want 2h5m", stats.SleepBlocks[0])
	}
}

func TestSleepBlockOnMeOnlyLong(t *testing.T) {
	// Even a long on-me-only block is excluded from sleep blocks.
	start := t0()
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, start, nil),
		mkEvent(2, domain.Awake, domain.StartFeed, domain.Feeding, start, breast("L")),
		mkEvent(3, domain.Feeding, domain.DislatchAsleep, domain.SleepingOnMe, start.Add(20*time.Minute), nil),
		mkEvent(4, domain.SleepingOnMe, domain.BabyWoke, domain.Awake, start.Add(2*time.Hour), nil),
		mkEvent(5, domain.Awake, domain.Action("end_night"), domain.NightOff, start.Add(2*time.Hour), nil),
	}

	stats, _ := computeBaseStats(events, start, start.Add(2*time.Hour))

	if len(stats.SleepBlocks) != 0 {
		t.Fatalf("SleepBlocks count = %d, want 0 (on-me-only excluded)", len(stats.SleepBlocks))
	}
	if stats.LongestSleepBlock != 0 {
		t.Errorf("LongestSleepBlock = %v, want 0", stats.LongestSleepBlock)
	}
}

func TestFeedCountAfterStrollerSleep(t *testing.T) {
	// Stroller sleep also counts as "real sleep"
	start := t0()
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, start, nil),
		// Pre-sleep feed
		mkEvent(2, domain.Awake, domain.StartFeed, domain.Feeding, start, breast("L")),
		mkEvent(3, domain.Feeding, domain.DislatchAwake, domain.Awake, start.Add(15*time.Minute), nil),
		// Stroller sleep — first real sleep
		mkEvent(4, domain.Awake, domain.StartStrolling, domain.Strolling, start.Add(20*time.Minute), nil),
		mkEvent(5, domain.Strolling, domain.FellAsleep, domain.SleepingStroller, start.Add(30*time.Minute), nil),
		// Wake and feed
		mkEvent(6, domain.SleepingStroller, domain.BabyWoke, domain.Awake, start.Add(4*time.Hour), nil),
		mkEvent(7, domain.Awake, domain.StartFeed, domain.Feeding, start.Add(4*time.Hour), breast("R")),
		mkEvent(8, domain.Feeding, domain.DislatchAwake, domain.Awake, start.Add(4*time.Hour+15*time.Minute), nil),
		mkEvent(9, domain.Awake, domain.Action("end_night"), domain.NightOff, start.Add(4*time.Hour+15*time.Minute), nil),
	}

	stats, _ := computeBaseStats(events, start, start.Add(4*time.Hour+15*time.Minute))

	if stats.FeedCount != 1 {
		t.Errorf("FeedCount = %d, want 1 (only post-stroller feed counts)", stats.FeedCount)
	}
	if len(stats.FeedTimes) != 1 {
		t.Fatalf("FeedTimes count = %d, want 1", len(stats.FeedTimes))
	}
	if !stats.FeedTimes[0].Equal(start.Add(4 * time.Hour)) {
		t.Errorf("FeedTimes[0] = %v, want %v", stats.FeedTimes[0], start.Add(4*time.Hour))
	}
}

func TestTimelineEntries(t *testing.T) {
	start := t0()
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, start, nil),
		mkEvent(2, domain.Awake, domain.StartFeed, domain.Feeding, start.Add(2*time.Minute), breast("L")),
		mkEvent(3, domain.Feeding, domain.DislatchAwake, domain.Awake, start.Add(12*time.Minute), nil),
		mkEvent(4, domain.Awake, domain.Action("end_night"), domain.NightOff, start.Add(15*time.Minute), nil),
	}

	tl := BuildTimeline(events, start.Add(15*time.Minute))

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

func TestTimelineClosesOpenState(t *testing.T) {
	// In-progress night: baby is sleeping in crib, no closing event yet.
	// nightEnd (time.Now) should close the final state.
	start := t0()
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, start, nil),
		mkEvent(2, domain.Awake, domain.StartFeed, domain.Feeding, start, breast("L")),
		mkEvent(3, domain.Feeding, domain.DislatchAsleep, domain.SleepingOnMe, start.Add(15*time.Minute), nil),
		mkEvent(4, domain.SleepingOnMe, domain.StartTransfer, domain.Transferring, start.Add(20*time.Minute), nil),
		mkEvent(5, domain.Transferring, domain.TransferSuccess, domain.SleepingCrib, start.Add(20*time.Minute), nil),
	}

	nightEnd := start.Add(2 * time.Hour)
	tl := BuildTimeline(events, nightEnd)

	// Last entry should be sleeping_crib closed at nightEnd: 20m to 2h = 1h40m
	last := tl[len(tl)-1]
	if last.State != domain.SleepingCrib {
		t.Errorf("last state = %s, want sleeping_crib", last.State)
	}
	if last.Duration != 1*time.Hour+40*time.Minute {
		t.Errorf("last duration = %v, want 1h40m", last.Duration)
	}
}

func TestStatsInProgressNight(t *testing.T) {
	// Baby sleeping in crib with no EndNight — sleep blocks should include the open block.
	start := t0()
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, start, nil),
		mkEvent(2, domain.Awake, domain.StartFeed, domain.Feeding, start, breast("L")),
		mkEvent(3, domain.Feeding, domain.DislatchAsleep, domain.SleepingOnMe, start.Add(15*time.Minute), nil),
		mkEvent(4, domain.SleepingOnMe, domain.StartTransfer, domain.Transferring, start.Add(20*time.Minute), nil),
		mkEvent(5, domain.Transferring, domain.TransferSuccess, domain.SleepingCrib, start.Add(20*time.Minute), nil),
		// Wake, feed, back to crib
		mkEvent(6, domain.SleepingCrib, domain.BabyWoke, domain.Awake, start.Add(3*time.Hour), nil),
		mkEvent(7, domain.Awake, domain.StartFeed, domain.Feeding, start.Add(3*time.Hour), breast("R")),
		mkEvent(8, domain.Feeding, domain.DislatchAsleep, domain.SleepingOnMe, start.Add(3*time.Hour+10*time.Minute), nil),
		mkEvent(9, domain.SleepingOnMe, domain.StartTransfer, domain.Transferring, start.Add(3*time.Hour+15*time.Minute), nil),
		mkEvent(10, domain.Transferring, domain.TransferSuccess, domain.SleepingCrib, start.Add(3*time.Hour+15*time.Minute), nil),
		// Still sleeping — no EndNight
	}

	nightEnd := start.Add(5 * time.Hour) // "now"
	stats, _ := computeBaseStats(events, start, nightEnd)

	// Block 1: on_me(5m) + crib(2h40m) = 2h45m
	// Block 2: on_me(5m) + crib(1h45m) = 1h50m (open, closed by nightEnd)
	if len(stats.SleepBlocks) != 2 {
		t.Fatalf("SleepBlocks count = %d, want 2", len(stats.SleepBlocks))
	}
	if stats.SleepBlocks[0] != 2*time.Hour+45*time.Minute {
		t.Errorf("SleepBlocks[0] = %v, want 2h45m", stats.SleepBlocks[0])
	}
	if stats.SleepBlocks[1] != 1*time.Hour+50*time.Minute {
		t.Errorf("SleepBlocks[1] = %v, want 1h50m", stats.SleepBlocks[1])
	}
	if stats.LongestSleepBlock != 2*time.Hour+45*time.Minute {
		t.Errorf("LongestSleepBlock = %v, want 2h45m", stats.LongestSleepBlock)
	}
}

// TestEveryStateClassified ensures that every state in AllStates is explicitly
// accounted for in the sleep classification maps. Adding a new state without
// classifying it here will fail this test.
func TestEveryStateClassified(t *testing.T) {
	// States that are explicitly not sleep-related in the NIGHT timeline.
	// When adding a new state, put it here OR in the appropriate sleep map.
	// Day states are vacuously non-night-sleep: they never appear in a night
	// session's event stream (chain boundary is also a session boundary), so
	// their classification under night logic is never exercised. Listing them
	// here keeps TestEveryStateClassified honest without polluting the
	// night-only sleep maps in timeline.go.
	nonSleepStates := map[domain.State]bool{
		domain.NightOff:    true,
		domain.Awake:       true,
		domain.Feeding:     true,
		domain.Poop:        true,
		domain.Learning:    true, // Ferber: awake in crib, not sleep (spec §3.8)
		domain.CheckIn:     true, // Ferber: parent in the room, not sleep (spec §3.8)
		domain.DayAwake:    true, // day subgraph — never appears in night timelines
		domain.DayFeeding:  true,
		domain.DaySleeping: true,
		domain.DayPoop:     true,
	}

	for _, state := range domain.AllStates {
		inSleep := sleepStates[state]
		inContiguous := contiguousSleepStates[state]
		inNonSleep := nonSleepStates[state]

		if !inSleep && !inContiguous && !inNonSleep {
			t.Errorf("state %s is unclassified — add it to the sleep or non-sleep group in timeline.go and this test", state)
		}
		if (inSleep || inContiguous) && inNonSleep {
			t.Errorf("state %s is in both a sleep map and nonSleepStates — pick one", state)
		}
	}

	// independentSleepStates must be a subset of sleepStates
	for state := range independentSleepStates {
		if !sleepStates[state] {
			t.Errorf("state %s is in independentSleepStates but not sleepStates", state)
		}
	}

}

func TestTransferSuccessCountsAsSleep(t *testing.T) {
	start := t0()
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, start, nil),
		mkEvent(2, domain.Awake, domain.StartFeed, domain.Feeding, start, breast("L")),
		mkEvent(3, domain.Feeding, domain.DislatchAsleep, domain.SleepingOnMe, start.Add(15*time.Minute), nil),
		// Transfer takes 3 minutes
		mkEvent(4, domain.SleepingOnMe, domain.StartTransfer, domain.Transferring, start.Add(20*time.Minute), nil),
		mkEvent(5, domain.Transferring, domain.TransferSuccess, domain.SleepingCrib, start.Add(23*time.Minute), nil),
		// Sleep in crib
		mkEvent(6, domain.SleepingCrib, domain.BabyWoke, domain.Awake, start.Add(4*time.Hour), nil),
		mkEvent(7, domain.Awake, domain.Action("end_night"), domain.NightOff, start.Add(4*time.Hour), nil),
	}

	stats, tl := computeBaseStats(events, start, start.Add(4*time.Hour))

	// Timeline should include the transferring entry
	hasTransfer := false
	for _, entry := range tl {
		if entry.State == domain.Transferring {
			hasTransfer = true
			if entry.Duration != 3*time.Minute {
				t.Errorf("transferring duration = %v, want 3m", entry.Duration)
			}
		}
	}
	if !hasTransfer {
		t.Error("timeline should include a transferring entry")
	}

	// Successful transfer: 3m should count as sleep
	// Total sleep: on_me(5m) + transferring(3m) + crib(3h37m) = 3h45m
	expectedSleep := 5*time.Minute + 3*time.Minute + 3*time.Hour + 37*time.Minute
	if stats.TotalSleepTime != expectedSleep {
		t.Errorf("TotalSleepTime = %v, want %v", stats.TotalSleepTime, expectedSleep)
	}

	// Sleep block: on_me(5m) + transferring(3m) + crib(3h37m) = 3h45m
	if len(stats.SleepBlocks) != 1 {
		t.Fatalf("SleepBlocks count = %d, want 1", len(stats.SleepBlocks))
	}
	if stats.SleepBlocks[0] != 3*time.Hour+45*time.Minute {
		t.Errorf("SleepBlocks[0] = %v, want 3h45m", stats.SleepBlocks[0])
	}
}

func TestTransferFailedCountsAsAwake(t *testing.T) {
	start := t0()
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, start, nil),
		mkEvent(2, domain.Awake, domain.StartFeed, domain.Feeding, start, breast("L")),
		mkEvent(3, domain.Feeding, domain.DislatchAsleep, domain.SleepingOnMe, start.Add(15*time.Minute), nil),
		// Transfer takes 5 minutes and fails
		mkEvent(4, domain.SleepingOnMe, domain.StartTransfer, domain.Transferring, start.Add(20*time.Minute), nil),
		mkEvent(5, domain.Transferring, domain.TransferFailed, domain.Awake, start.Add(25*time.Minute), nil),
		mkEvent(6, domain.Awake, domain.Action("end_night"), domain.NightOff, start.Add(25*time.Minute), nil),
	}

	stats, _ := computeBaseStats(events, start, start.Add(25*time.Minute))

	// Failed transfer: 5m should count as awake, not sleep
	// Total sleep: on_me(5m) only
	if stats.TotalSleepTime != 5*time.Minute {
		t.Errorf("TotalSleepTime = %v, want 5m", stats.TotalSleepTime)
	}
	// Total awake: transferring(5m)
	if stats.TotalAwakeTime != 5*time.Minute {
		t.Errorf("TotalAwakeTime = %v, want 5m", stats.TotalAwakeTime)
	}
}

func TestTransferNeedResettleCountsAsSleep(t *testing.T) {
	start := t0()
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, start, nil),
		mkEvent(2, domain.Awake, domain.StartFeed, domain.Feeding, start, breast("R")),
		mkEvent(3, domain.Feeding, domain.DislatchAsleep, domain.SleepingOnMe, start.Add(15*time.Minute), nil),
		// Transfer takes 4 minutes, baby stirs → needs resettle
		mkEvent(4, domain.SleepingOnMe, domain.StartTransfer, domain.Transferring, start.Add(20*time.Minute), nil),
		mkEvent(5, domain.Transferring, domain.TransferNeedResettle, domain.Resettling, start.Add(24*time.Minute), nil),
		mkEvent(6, domain.Resettling, domain.Settled, domain.SleepingCrib, start.Add(30*time.Minute), nil),
		mkEvent(7, domain.SleepingCrib, domain.BabyWoke, domain.Awake, start.Add(4*time.Hour), nil),
		mkEvent(8, domain.Awake, domain.Action("end_night"), domain.NightOff, start.Add(4*time.Hour), nil),
	}

	stats, _ := computeBaseStats(events, start, start.Add(4*time.Hour))

	// Transfer to resettle: 4m counts as sleep
	// Total sleep: on_me(5m) + transferring(4m) + resettling(6m) + crib(3h30m) = 3h45m
	expectedSleep := 5*time.Minute + 4*time.Minute + 6*time.Minute + 3*time.Hour + 30*time.Minute
	if stats.TotalSleepTime != expectedSleep {
		t.Errorf("TotalSleepTime = %v, want %v", stats.TotalSleepTime, expectedSleep)
	}
}

func TestTransferInProgressNotCounted(t *testing.T) {
	start := t0()
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, start, nil),
		mkEvent(2, domain.Awake, domain.StartFeed, domain.Feeding, start, breast("L")),
		mkEvent(3, domain.Feeding, domain.DislatchAsleep, domain.SleepingOnMe, start.Add(15*time.Minute), nil),
		// Transfer started, no outcome yet
		mkEvent(4, domain.SleepingOnMe, domain.StartTransfer, domain.Transferring, start.Add(20*time.Minute), nil),
	}

	nightEnd := start.Add(25 * time.Minute) // "now", 5 minutes into transfer
	stats, tl := computeBaseStats(events, start, nightEnd)

	// Timeline should show transferring with 5m duration
	last := tl[len(tl)-1]
	if last.State != domain.Transferring {
		t.Errorf("last timeline state = %s, want transferring", last.State)
	}
	if last.Duration != 5*time.Minute {
		t.Errorf("last timeline duration = %v, want 5m", last.Duration)
	}

	// In-progress transfer: not counted as sleep or awake
	// Total sleep: on_me(5m) only
	if stats.TotalSleepTime != 5*time.Minute {
		t.Errorf("TotalSleepTime = %v, want 5m (in-progress transfer not counted)", stats.TotalSleepTime)
	}
	if stats.TotalAwakeTime != 0 {
		t.Errorf("TotalAwakeTime = %v, want 0 (in-progress transfer not counted)", stats.TotalAwakeTime)
	}
}

func TestSelfSoothingCountsAsAwake(t *testing.T) {
	// SleepingCrib(20m) → SelfSoothing(5m) → SleepingCrib(30m) → BabyWoke.
	// Expect: the 5m of self-soothing counts as awake, not sleep.
	// Expect: the block remains contiguous — single 55m block (20+5+30).
	start := t0()
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, start, nil),
		mkEvent(2, domain.Awake, domain.StartFeed, domain.Feeding, start, breast("L")),
		mkEvent(3, domain.Feeding, domain.DislatchAsleep, domain.SleepingOnMe, start.Add(5*time.Minute), nil),
		mkEvent(4, domain.SleepingOnMe, domain.StartTransfer, domain.Transferring, start.Add(10*time.Minute), nil),
		mkEvent(5, domain.Transferring, domain.TransferSuccess, domain.SleepingCrib, start.Add(10*time.Minute), nil),
		// Sleeping in crib 00:10 → 00:30 (20m)
		mkEvent(6, domain.SleepingCrib, domain.BabyStirred, domain.SelfSoothing, start.Add(30*time.Minute), nil),
		// Self-soothing 00:30 → 00:35 (5m)
		mkEvent(7, domain.SelfSoothing, domain.Settled, domain.SleepingCrib, start.Add(35*time.Minute), nil),
		// Sleeping in crib 00:35 → 01:05 (30m)
		mkEvent(8, domain.SleepingCrib, domain.BabyWoke, domain.Awake, start.Add(65*time.Minute), nil),
		mkEvent(9, domain.Awake, domain.Action("end_night"), domain.NightOff, start.Add(65*time.Minute), nil),
	}

	stats, _ := computeBaseStats(events, start, start.Add(65*time.Minute))

	// Single contiguous block: on_me(5m) + transferring(0) + crib(20m) + self_soothing(5m) + crib(30m) = 60m
	if len(stats.SleepBlocks) != 1 {
		t.Fatalf("SleepBlocks count = %d, want 1", len(stats.SleepBlocks))
	}
	if stats.SleepBlocks[0] != 60*time.Minute {
		t.Errorf("SleepBlocks[0] = %v, want 60m", stats.SleepBlocks[0])
	}

	// Total sleep: on_me(5m) + crib(20m) + crib(30m) = 55m (self-soothing excluded)
	if stats.TotalSleepTime != 55*time.Minute {
		t.Errorf("TotalSleepTime = %v, want 55m (self-soothing excluded)", stats.TotalSleepTime)
	}
	// Total awake: self_soothing(5m)
	if stats.TotalAwakeTime != 5*time.Minute {
		t.Errorf("TotalAwakeTime = %v, want 5m (self-soothing counted as awake)", stats.TotalAwakeTime)
	}
}

// mustTime is a helper used by bedtime tests to assert a non-nil timestamp equals an expected value.
func mustTime(t *testing.T, ts *time.Time, want time.Time, context string) {
	t.Helper()
	if ts == nil {
		t.Fatalf("%s: RealBedtime = nil, want %v", context, want)
	}
	if !ts.Equal(want) {
		t.Errorf("%s: RealBedtime = %v, want %v", context, *ts, want)
	}
}

func TestRealBedtimeFromDislatchAsleep(t *testing.T) {
	// Feeding → SleepingOnMe → Transferring → SleepingCrib.
	// Bedtime should be the dislatch_asleep timestamp (start of SleepingOnMe).
	start := t0()
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, start, nil),
		mkEvent(2, domain.Awake, domain.StartFeed, domain.Feeding, start, breast("L")),
		mkEvent(3, domain.Feeding, domain.DislatchAsleep, domain.SleepingOnMe, start.Add(20*time.Minute), nil),
		mkEvent(4, domain.SleepingOnMe, domain.StartTransfer, domain.Transferring, start.Add(25*time.Minute), nil),
		mkEvent(5, domain.Transferring, domain.TransferSuccess, domain.SleepingCrib, start.Add(25*time.Minute), nil),
		mkEvent(6, domain.SleepingCrib, domain.BabyWoke, domain.Awake, start.Add(4*time.Hour), nil),
		mkEvent(7, domain.Awake, domain.Action("end_night"), domain.NightOff, start.Add(4*time.Hour), nil),
	}
	stats, _ := computeBaseStats(events, start, start.Add(4*time.Hour))
	mustTime(t, stats.RealBedtime, start.Add(20*time.Minute), "dislatch_asleep")
}

func TestRealBedtimeFromSettledAfterSelfSoothing(t *testing.T) {
	// Awake → SelfSoothing (put down awake) → SleepingCrib (settled).
	// Bedtime should be the `settled` timestamp.
	start := t0()
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, start, nil),
		mkEvent(2, domain.Awake, domain.PutDownAwake, domain.SelfSoothing, start.Add(10*time.Minute), nil),
		mkEvent(3, domain.SelfSoothing, domain.Settled, domain.SleepingCrib, start.Add(25*time.Minute), nil),
		mkEvent(4, domain.SleepingCrib, domain.BabyWoke, domain.Awake, start.Add(3*time.Hour), nil),
		mkEvent(5, domain.Awake, domain.Action("end_night"), domain.NightOff, start.Add(3*time.Hour), nil),
	}
	stats, _ := computeBaseStats(events, start, start.Add(3*time.Hour))
	mustTime(t, stats.RealBedtime, start.Add(25*time.Minute), "settled from self-soothing")
}

func TestRealBedtimeFromSettledAfterResettling(t *testing.T) {
	// Awake → Resettling → SleepingCrib.
	// Bedtime should be the `settled` timestamp (Resettling → SleepingCrib).
	start := t0()
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, start, nil),
		mkEvent(2, domain.Awake, domain.StartResettle, domain.Resettling, start, nil),
		mkEvent(3, domain.Resettling, domain.Settled, domain.SleepingCrib, start.Add(12*time.Minute), nil),
		mkEvent(4, domain.SleepingCrib, domain.BabyWoke, domain.Awake, start.Add(3*time.Hour), nil),
		mkEvent(5, domain.Awake, domain.Action("end_night"), domain.NightOff, start.Add(3*time.Hour), nil),
	}
	stats, _ := computeBaseStats(events, start, start.Add(3*time.Hour))
	mustTime(t, stats.RealBedtime, start.Add(12*time.Minute), "settled from resettling")
}

func TestRealBedtimeFromStrollerFellAsleep(t *testing.T) {
	// Strolling → SleepingStroller.
	// Bedtime should be the `fell_asleep` timestamp.
	start := t0()
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, start, nil),
		mkEvent(2, domain.Awake, domain.StartStrolling, domain.Strolling, start, nil),
		mkEvent(3, domain.Strolling, domain.FellAsleep, domain.SleepingStroller, start.Add(18*time.Minute), nil),
		mkEvent(4, domain.SleepingStroller, domain.BabyWoke, domain.Awake, start.Add(3*time.Hour), nil),
		mkEvent(5, domain.Awake, domain.Action("end_night"), domain.NightOff, start.Add(3*time.Hour), nil),
	}
	stats, _ := computeBaseStats(events, start, start.Add(3*time.Hour))
	mustTime(t, stats.RealBedtime, start.Add(18*time.Minute), "fell_asleep in stroller")
}

func TestRealBedtimeNilForContactOnlyNight(t *testing.T) {
	// Baby fell asleep at breast, woke before any transfer. No independent sleep.
	start := t0()
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, start, nil),
		mkEvent(2, domain.Awake, domain.StartFeed, domain.Feeding, start, breast("L")),
		mkEvent(3, domain.Feeding, domain.DislatchAsleep, domain.SleepingOnMe, start.Add(20*time.Minute), nil),
		mkEvent(4, domain.SleepingOnMe, domain.BabyWoke, domain.Awake, start.Add(2*time.Hour), nil),
		mkEvent(5, domain.Awake, domain.Action("end_night"), domain.NightOff, start.Add(2*time.Hour), nil),
	}
	stats, _ := computeBaseStats(events, start, start.Add(2*time.Hour))
	if stats.RealBedtime != nil {
		t.Errorf("RealBedtime = %v, want nil (no independent sleep)", *stats.RealBedtime)
	}
}

func TestRealBedtimeUsesSecondBlockWhenFirstFails(t *testing.T) {
	// First block ends with transfer_failed — no independent sleep.
	// Second block succeeds via dislatch_asleep → transfer_success.
	// Bedtime = second block's dislatch_asleep.
	start := t0()
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, start, nil),
		// Block 1: fail
		mkEvent(2, domain.Awake, domain.StartFeed, domain.Feeding, start, breast("L")),
		mkEvent(3, domain.Feeding, domain.DislatchAsleep, domain.SleepingOnMe, start.Add(10*time.Minute), nil),
		mkEvent(4, domain.SleepingOnMe, domain.StartTransfer, domain.Transferring, start.Add(15*time.Minute), nil),
		mkEvent(5, domain.Transferring, domain.TransferFailed, domain.Awake, start.Add(17*time.Minute), nil),
		// Block 2: success
		mkEvent(6, domain.Awake, domain.StartFeed, domain.Feeding, start.Add(20*time.Minute), breast("R")),
		mkEvent(7, domain.Feeding, domain.DislatchAsleep, domain.SleepingOnMe, start.Add(35*time.Minute), nil),
		mkEvent(8, domain.SleepingOnMe, domain.StartTransfer, domain.Transferring, start.Add(40*time.Minute), nil),
		mkEvent(9, domain.Transferring, domain.TransferSuccess, domain.SleepingCrib, start.Add(40*time.Minute), nil),
		mkEvent(10, domain.SleepingCrib, domain.BabyWoke, domain.Awake, start.Add(3*time.Hour), nil),
		mkEvent(11, domain.Awake, domain.Action("end_night"), domain.NightOff, start.Add(3*time.Hour), nil),
	}
	stats, _ := computeBaseStats(events, start, start.Add(3*time.Hour))
	mustTime(t, stats.RealBedtime, start.Add(35*time.Minute), "second block dislatch_asleep")
}

func TestRealBedtimeInProgressReached(t *testing.T) {
	// In-progress night where baby has already reached SleepingCrib.
	start := t0()
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, start, nil),
		mkEvent(2, domain.Awake, domain.StartFeed, domain.Feeding, start, breast("L")),
		mkEvent(3, domain.Feeding, domain.DislatchAsleep, domain.SleepingOnMe, start.Add(15*time.Minute), nil),
		mkEvent(4, domain.SleepingOnMe, domain.StartTransfer, domain.Transferring, start.Add(20*time.Minute), nil),
		mkEvent(5, domain.Transferring, domain.TransferSuccess, domain.SleepingCrib, start.Add(20*time.Minute), nil),
		// No EndNight; still sleeping.
	}
	nightEnd := start.Add(45 * time.Minute) // "now"
	stats, _ := computeBaseStats(events, start, nightEnd)
	mustTime(t, stats.RealBedtime, start.Add(15*time.Minute), "in-progress reached crib")
}

func TestRealBedtimeInProgressNotReached(t *testing.T) {
	// In-progress night where baby hasn't reached independent sleep yet.
	start := t0()
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, start, nil),
		mkEvent(2, domain.Awake, domain.StartFeed, domain.Feeding, start, breast("L")),
		mkEvent(3, domain.Feeding, domain.DislatchAsleep, domain.SleepingOnMe, start.Add(15*time.Minute), nil),
		// Still on me; no transfer yet.
	}
	nightEnd := start.Add(20 * time.Minute) // "now"
	stats, _ := computeBaseStats(events, start, nightEnd)
	if stats.RealBedtime != nil {
		t.Errorf("RealBedtime = %v, want nil (in-progress, no independent sleep yet)", *stats.RealBedtime)
	}
}

func TestRealBedtimeEmpty(t *testing.T) {
	stats, _ := computeBaseStats(nil, t0(), t0())
	if stats.RealBedtime != nil {
		t.Errorf("RealBedtime = %v, want nil for empty events", *stats.RealBedtime)
	}
}

func TestRealBedtimeWithSwitchBreast(t *testing.T) {
	// Block has switch_breast before dislatch_asleep. Bedtime = dislatch_asleep timestamp,
	// not switch_breast (switch_breast is not an asleep-signaling action).
	start := t0()
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, start, nil),
		mkEvent(2, domain.Awake, domain.StartFeed, domain.Feeding, start, breast("L")),
		mkEvent(3, domain.Feeding, domain.SwitchBreast, domain.Feeding, start.Add(10*time.Minute), breast("R")),
		mkEvent(4, domain.Feeding, domain.DislatchAsleep, domain.SleepingOnMe, start.Add(25*time.Minute), nil),
		mkEvent(5, domain.SleepingOnMe, domain.StartTransfer, domain.Transferring, start.Add(30*time.Minute), nil),
		mkEvent(6, domain.Transferring, domain.TransferSuccess, domain.SleepingCrib, start.Add(30*time.Minute), nil),
		mkEvent(7, domain.SleepingCrib, domain.BabyWoke, domain.Awake, start.Add(3*time.Hour), nil),
		mkEvent(8, domain.Awake, domain.Action("end_night"), domain.NightOff, start.Add(3*time.Hour), nil),
	}
	stats, _ := computeBaseStats(events, start, start.Add(3*time.Hour))
	mustTime(t, stats.RealBedtime, start.Add(25*time.Minute), "dislatch_asleep after switch_breast")
}

func TestRealBedtimeViaTransferNeedResettle(t *testing.T) {
	// Feeding → SleepingOnMe → Transferring → Resettling → SleepingCrib.
	// Bedtime = dislatch_asleep (the first asleep-signaling action in the block).
	start := t0()
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, start, nil),
		mkEvent(2, domain.Awake, domain.StartFeed, domain.Feeding, start, breast("L")),
		mkEvent(3, domain.Feeding, domain.DislatchAsleep, domain.SleepingOnMe, start.Add(15*time.Minute), nil),
		mkEvent(4, domain.SleepingOnMe, domain.StartTransfer, domain.Transferring, start.Add(20*time.Minute), nil),
		mkEvent(5, domain.Transferring, domain.TransferNeedResettle, domain.Resettling, start.Add(24*time.Minute), nil),
		mkEvent(6, domain.Resettling, domain.Settled, domain.SleepingCrib, start.Add(30*time.Minute), nil),
		mkEvent(7, domain.SleepingCrib, domain.BabyWoke, domain.Awake, start.Add(4*time.Hour), nil),
		mkEvent(8, domain.Awake, domain.Action("end_night"), domain.NightOff, start.Add(4*time.Hour), nil),
	}
	stats, _ := computeBaseStats(events, start, start.Add(4*time.Hour))
	mustTime(t, stats.RealBedtime, start.Add(15*time.Minute), "dislatch_asleep (block that reaches crib via resettle)")
}
