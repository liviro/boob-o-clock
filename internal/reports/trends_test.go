package reports

import (
	"testing"
	"time"

	"github.com/liviro/boob-o-clock/internal/domain"
)

// --- helpers ---

// dayStart anchors day fixtures to a concrete 7am for readable assertions.
func dayStart() time.Time {
	return time.Date(2026, 4, 23, 7, 0, 0, 0, time.Local)
}

func mkSession(id int64, kind domain.SessionKind, startedAt time.Time, endedAt *time.Time) *domain.Session {
	return &domain.Session{
		ID:        id,
		Kind:      kind,
		StartedAt: startedAt,
		EndedAt:   endedAt,
	}
}

func ptrTime(t time.Time) *time.Time {
	return &t
}

// awakeWindows is a test-local convenience: runs buildDaySegments and
// returns just the awake-kind durations in order. Production code reads
// WakeWindows off DayStats (which derives from buildDaySegments via the
// same filter); this helper keeps the existing test assertions compact.
func awakeWindows(day *domain.Session, dayEvents []domain.Event, night *domain.Session, nightEvents []domain.Event) []time.Duration {
	var out []time.Duration
	for _, s := range buildDaySegments(day, dayEvents, night, nightEvents) {
		if s.Kind == "awake" {
			out = append(out, s.Duration)
		}
	}
	return out
}

// --- awakeWindows (via buildDaySegments) ---

// TestWakeWindows_TwoNaps exercises the canonical scenario from the design
// doc §5.4.1: day has naps at 10am and 2pm; bedtime at 7pm with baby asleep
// in crib at 7:15pm. Expect 3 wake windows.
func TestWakeWindows_TwoNaps(t *testing.T) {
	start := dayStart() // 7am
	nap1Start := start.Add(3 * time.Hour)           // 10am
	nap1End := nap1Start.Add(1 * time.Hour)         // 11am
	nap2Start := start.Add(7 * time.Hour)           // 2pm
	nap2End := nap2Start.Add(90 * time.Minute)      // 3:30pm
	dayEnd := start.Add(12 * time.Hour)             // 7pm
	nightAsleepAt := dayEnd.Add(15 * time.Minute)   // 7:15pm

	day := mkSession(1, domain.SessionKindDay, start, ptrTime(dayEnd))
	night := mkSession(2, domain.SessionKindNight, dayEnd, nil)

	dayEvents := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartDay, domain.DayAwake, start, nil),
		mkEvent(2, domain.DayAwake, domain.StartSleep, domain.DaySleeping, nap1Start, map[string]string{"location": "crib"}),
		mkEvent(3, domain.DaySleeping, domain.BabyWoke, domain.DayAwake, nap1End, nil),
		mkEvent(4, domain.DayAwake, domain.StartSleep, domain.DaySleeping, nap2Start, map[string]string{"location": "crib"}),
		mkEvent(5, domain.DaySleeping, domain.BabyWoke, domain.DayAwake, nap2End, nil),
	}
	nightEvents := []domain.Event{
		mkEvent(1, domain.DayAwake, domain.StartNight, domain.Awake, dayEnd, nil),
		mkEvent(2, domain.Awake, domain.StartResettle, domain.Resettling, dayEnd.Add(5*time.Minute), nil),
		mkEvent(3, domain.Resettling, domain.Settled, domain.SleepingCrib, nightAsleepAt, nil),
	}

	windows := awakeWindows(day, dayEvents, night, nightEvents)
	if len(windows) != 3 {
		t.Fatalf("got %d windows, want 3: %v", len(windows), windows)
	}
	want := []time.Duration{
		3 * time.Hour,                 // 7am → 10am
		3 * time.Hour,                 // 11am → 2pm
		nightAsleepAt.Sub(nap2End),    // 3:30pm → 7:15pm
	}
	for i, w := range windows {
		if w != want[i] {
			t.Errorf("window[%d] = %v, want %v", i, w, want[i])
		}
	}
}

// TestWakeWindows_NeverNapsCrossesBoundary: baby never naps during the day;
// the single wake window runs 7am to first-night-sleep at 7:15pm.
func TestWakeWindows_NeverNapsCrossesBoundary(t *testing.T) {
	start := dayStart()
	dayEnd := start.Add(12 * time.Hour)
	nightAsleepAt := dayEnd.Add(15 * time.Minute)

	day := mkSession(1, domain.SessionKindDay, start, ptrTime(dayEnd))
	night := mkSession(2, domain.SessionKindNight, dayEnd, nil)

	dayEvents := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartDay, domain.DayAwake, start, nil),
	}
	nightEvents := []domain.Event{
		mkEvent(1, domain.DayAwake, domain.StartNight, domain.Awake, dayEnd, nil),
		mkEvent(2, domain.Awake, domain.StartResettle, domain.Resettling, dayEnd.Add(5*time.Minute), nil),
		mkEvent(3, domain.Resettling, domain.Settled, domain.SleepingCrib, nightAsleepAt, nil),
	}

	windows := awakeWindows(day, dayEvents, night, nightEvents)
	if len(windows) != 1 {
		t.Fatalf("got %d windows, want 1", len(windows))
	}
	if windows[0] != nightAsleepAt.Sub(start) {
		t.Errorf("window = %v, want %v", windows[0], nightAsleepAt.Sub(start))
	}
}

// TestWakeWindows_InProgressNoNaps exercises the "today, open day, no naps"
// branch: a single running open window from start to now (approximated with a
// tolerance since ComputeDayStats calls time.Now() internally).
func TestWakeWindows_InProgressNoNaps(t *testing.T) {
	start := time.Now().Add(-3 * time.Hour)
	day := mkSession(1, domain.SessionKindDay, start, nil) // still open

	dayEvents := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartDay, domain.DayAwake, start, nil),
	}

	windows := awakeWindows(day, dayEvents, nil, nil)
	if len(windows) != 1 {
		t.Fatalf("got %d windows, want 1", len(windows))
	}
	// Running window ≈ 3 hours. Allow ±5s slop.
	diff := windows[0] - 3*time.Hour
	if diff < -5*time.Second || diff > 5*time.Second {
		t.Errorf("window = %v, want ~3h", windows[0])
	}
}

// TestWakeWindows_InProgressBetweenNaps: day is open, one nap completed, now awake.
// Expect 2 windows (pre-nap closed, post-nap running).
func TestWakeWindows_InProgressBetweenNaps(t *testing.T) {
	now := time.Now()
	start := now.Add(-5 * time.Hour)
	napStart := now.Add(-3 * time.Hour)
	napEnd := now.Add(-2 * time.Hour)

	day := mkSession(1, domain.SessionKindDay, start, nil)
	dayEvents := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartDay, domain.DayAwake, start, nil),
		mkEvent(2, domain.DayAwake, domain.StartSleep, domain.DaySleeping, napStart, map[string]string{"location": "crib"}),
		mkEvent(3, domain.DaySleeping, domain.BabyWoke, domain.DayAwake, napEnd, nil),
	}

	windows := awakeWindows(day, dayEvents, nil, nil)
	if len(windows) != 2 {
		t.Fatalf("got %d windows, want 2: %v", len(windows), windows)
	}
	if windows[0] != napStart.Sub(start) {
		t.Errorf("closed window = %v, want %v", windows[0], napStart.Sub(start))
	}
	// Running window ≈ 2h. Tolerance for time.Now() slop.
	diff := windows[1] - 2*time.Hour
	if diff < -5*time.Second || diff > 5*time.Second {
		t.Errorf("running window = %v, want ~2h", windows[1])
	}
}

// TestWakeWindows_OrphanDayNil returns empty windows when there's no day.
func TestWakeWindows_OrphanDayNil(t *testing.T) {
	windows := awakeWindows(nil, nil, nil, nil)
	if windows != nil {
		t.Errorf("orphan (day=nil): got %v, want nil", windows)
	}
}

// TestWakeWindows_FeedsAndPoopDoNotBreak: mid-day feed and poop inside a
// wake window should NOT split the window — only DaySleeping breaks it.
func TestWakeWindows_FeedsAndPoopDoNotBreak(t *testing.T) {
	start := dayStart()
	feedStart := start.Add(1 * time.Hour)
	feedEnd := feedStart.Add(30 * time.Minute)
	poopStart := start.Add(2 * time.Hour)
	poopEnd := poopStart.Add(10 * time.Minute)
	napStart := start.Add(3 * time.Hour)
	dayEnd := start.Add(12 * time.Hour)
	nightAsleep := dayEnd.Add(10 * time.Minute)

	day := mkSession(1, domain.SessionKindDay, start, ptrTime(dayEnd))
	night := mkSession(2, domain.SessionKindNight, dayEnd, nil)
	dayEvents := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartDay, domain.DayAwake, start, nil),
		mkEvent(2, domain.DayAwake, domain.StartFeed, domain.DayFeeding, feedStart, map[string]string{"breast": "L"}),
		mkEvent(3, domain.DayFeeding, domain.DislatchAwake, domain.DayAwake, feedEnd, nil),
		mkEvent(4, domain.DayAwake, domain.PoopStart, domain.DayPoop, poopStart, nil),
		mkEvent(5, domain.DayPoop, domain.PoopDone, domain.DayAwake, poopEnd, nil),
		mkEvent(6, domain.DayAwake, domain.StartSleep, domain.DaySleeping, napStart, map[string]string{"location": "crib"}),
	}
	nightEvents := []domain.Event{
		mkEvent(1, domain.DayAwake, domain.StartNight, domain.Awake, dayEnd, nil),
		mkEvent(2, domain.Awake, domain.StartResettle, domain.Resettling, dayEnd.Add(5*time.Minute), nil),
		mkEvent(3, domain.Resettling, domain.Settled, domain.SleepingCrib, nightAsleep, nil),
	}

	windows := awakeWindows(day, dayEvents, night, nightEvents)
	if len(windows) != 1 {
		// With nap in the middle, we'd actually expect 2: pre-nap and
		// post-nap-into-night. The day events end with StartSleep (napStart),
		// so only one nap starts and never ends in the day events; but wait,
		// the day session is closed, so the nap would actually still be
		// running at chain advance. Let's check: in my events, the nap starts
		// but there's no BabyWoke — so windowStart is cleared when nap starts,
		// and the post-nap window starts only if there's a BabyWoke event.
		// With only one nap event and no wake, I expect ONE window: 7am→napStart.
		// Actually scratch that, read more carefully.
		t.Logf("windows: %v", windows)
	}
	// Re-reading the algo: windowStart set to start. At napStart, window closes
	// (7am→10am). After nap, no BabyWoke event exists in dayEvents, so
	// windowStart stays nil through end of dayEvents. In night events, no
	// from_state==DaySleeping exit would happen (nap persists across chain
	// boundary). So final terminal step would NOT open a new window.
	// Expectation: ONE window — pre-nap 3h.
	if len(windows) != 1 {
		t.Fatalf("got %d windows, want 1 (pre-nap only): %v", len(windows), windows)
	}
	if windows[0] != napStart.Sub(start) {
		t.Errorf("window = %v, want %v", windows[0], napStart.Sub(start))
	}
}

// --- ComputeDayStats ---

func TestComputeDayStats_DayDuration_Closed(t *testing.T) {
	start := dayStart()                // 7am
	end := start.Add(11 * time.Hour)   // 6pm
	day := mkSession(1, domain.SessionKindDay, start, ptrTime(end))
	dayEvents := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartDay, domain.DayAwake, start, nil),
	}

	stats := ComputeDayStats(day, dayEvents, nil, nil)

	if stats.DayDuration != 11*time.Hour {
		t.Errorf("DayDuration = %v, want 11h (end - start for closed day)", stats.DayDuration)
	}
}

// In-progress day clamps end to time.Now(). Tolerance covers test-execution slack.
func TestComputeDayStats_DayDuration_InProgress(t *testing.T) {
	start := time.Now().Add(-3 * time.Hour)
	day := mkSession(1, domain.SessionKindDay, start, nil)
	dayEvents := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartDay, domain.DayAwake, start, nil),
	}

	stats := ComputeDayStats(day, dayEvents, nil, nil)

	if stats.DayDuration < 3*time.Hour || stats.DayDuration > 3*time.Hour+time.Second {
		t.Errorf("DayDuration = %v, want ~3h (in-progress, end clamps to now)", stats.DayDuration)
	}
}

func TestComputeDayStats_NapDurations(t *testing.T) {
	start := dayStart()
	nap1Start := start.Add(2 * time.Hour)
	nap1End := nap1Start.Add(45 * time.Minute)
	nap2Start := start.Add(6 * time.Hour)
	nap2End := nap2Start.Add(90 * time.Minute)
	dayEnd := start.Add(12 * time.Hour)

	day := mkSession(1, domain.SessionKindDay, start, ptrTime(dayEnd))
	dayEvents := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartDay, domain.DayAwake, start, nil),
		mkEvent(2, domain.DayAwake, domain.StartSleep, domain.DaySleeping, nap1Start, map[string]string{"location": "crib"}),
		mkEvent(3, domain.DaySleeping, domain.BabyWoke, domain.DayAwake, nap1End, nil),
		mkEvent(4, domain.DayAwake, domain.StartSleep, domain.DaySleeping, nap2Start, map[string]string{"location": "crib"}),
		mkEvent(5, domain.DaySleeping, domain.BabyWoke, domain.DayAwake, nap2End, nil),
	}

	stats := ComputeDayStats(day, dayEvents, nil, nil)

	if stats.NapCount != 2 {
		t.Errorf("NapCount = %d, want 2", stats.NapCount)
	}
	if stats.TotalNapTime != 45*time.Minute+90*time.Minute {
		t.Errorf("TotalNapTime = %v, want 2h15m", stats.TotalNapTime)
	}
}

// TestComputeDayStats_DayFeedCountIgnoresSwitchBreast verifies that only
// start_feed events bump DayFeedCount — switch_breast (which also produces
// a feed event) must not be counted.
func TestComputeDayStats_DayFeedCountIgnoresSwitchBreast(t *testing.T) {
	start := dayStart()
	feed1 := start.Add(1 * time.Hour)
	feed2 := start.Add(5 * time.Hour)
	feed3 := start.Add(9 * time.Hour)
	dayEnd := start.Add(12 * time.Hour)

	day := mkSession(1, domain.SessionKindDay, start, ptrTime(dayEnd))
	dayEvents := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartDay, domain.DayAwake, start, nil),
		mkEvent(2, domain.DayAwake, domain.StartFeed, domain.DayFeeding, feed1, map[string]string{"breast": "L"}),
		mkEvent(3, domain.DayFeeding, domain.DislatchAwake, domain.DayAwake, feed1.Add(20*time.Minute), nil),
		mkEvent(4, domain.DayAwake, domain.StartFeed, domain.DayFeeding, feed2, map[string]string{"breast": "R"}),
		mkEvent(5, domain.DayFeeding, domain.SwitchBreast, domain.DayFeeding, feed2.Add(10*time.Minute), map[string]string{"breast": "L"}),
		mkEvent(6, domain.DayFeeding, domain.DislatchAwake, domain.DayAwake, feed2.Add(25*time.Minute), nil),
		mkEvent(7, domain.DayAwake, domain.StartFeed, domain.DayFeeding, feed3, map[string]string{"breast": "L"}),
		mkEvent(8, domain.DayFeeding, domain.DislatchAwake, domain.DayAwake, feed3.Add(15*time.Minute), nil),
	}

	stats := ComputeDayStats(day, dayEvents, nil, nil)

	if stats.DayFeedCount != 3 {
		t.Errorf("DayFeedCount = %d, want 3 (three start_feed events; switch_breast does NOT count)", stats.DayFeedCount)
	}
	// Feed1 L 20m + Feed2 first half R 10m + Feed2 second half L 15m + Feed3 L 15m
	// = L: 20+15+15 = 50m, R: 10m. Total 60m.
	if stats.FeedTimeLeft != 50*time.Minute {
		t.Errorf("FeedTimeLeft = %v, want 50m (20m feed1 + 15m feed2-after-switch + 15m feed3)", stats.FeedTimeLeft)
	}
	if stats.FeedTimeRight != 10*time.Minute {
		t.Errorf("FeedTimeRight = %v, want 10m (feed2 first half before switch)", stats.FeedTimeRight)
	}
	if stats.DayTotalFeedTime != 60*time.Minute {
		t.Errorf("DayTotalFeedTime = %v, want 60m (20+25+15)", stats.DayTotalFeedTime)
	}
}

// In-progress day feed: end clamps to time.Now(), so the side accumulator
// captures the elapsed time on the active side.
func TestComputeDayStats_FeedSidesInProgress(t *testing.T) {
	start := time.Now().Add(-2 * time.Hour)
	feedStart := start.Add(30 * time.Minute) // feed started 1h30m ago
	day := mkSession(1, domain.SessionKindDay, start, nil)
	dayEvents := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartDay, domain.DayAwake, start, nil),
		mkEvent(2, domain.DayAwake, domain.StartFeed, domain.DayFeeding, feedStart, map[string]string{"breast": "L"}),
	}

	stats := ComputeDayStats(day, dayEvents, nil, nil)

	// FeedTimeLeft ≈ 1h30m (allow 1s slack).
	want := 90 * time.Minute
	if stats.FeedTimeLeft < want || stats.FeedTimeLeft > want+time.Second {
		t.Errorf("FeedTimeLeft = %v, want ~90m (open feed, clamps to now)", stats.FeedTimeLeft)
	}
	if stats.FeedTimeRight != 0 {
		t.Errorf("FeedTimeRight = %v, want 0", stats.FeedTimeRight)
	}
}

func TestComputeDayStats_FeedTimes(t *testing.T) {
	start := dayStart()
	feed1Start := start.Add(1 * time.Hour)
	feed1End := feed1Start.Add(20 * time.Minute)
	feed2Start := start.Add(5 * time.Hour)
	feed2End := feed2Start.Add(15 * time.Minute)
	dayEnd := start.Add(12 * time.Hour)

	day := mkSession(1, domain.SessionKindDay, start, ptrTime(dayEnd))
	dayEvents := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartDay, domain.DayAwake, start, nil),
		mkEvent(2, domain.DayAwake, domain.StartFeed, domain.DayFeeding, feed1Start, map[string]string{"breast": "L"}),
		mkEvent(3, domain.DayFeeding, domain.DislatchAwake, domain.DayAwake, feed1End, nil),
		mkEvent(4, domain.DayAwake, domain.StartFeed, domain.DayFeeding, feed2Start, map[string]string{"breast": "R"}),
		mkEvent(5, domain.DayFeeding, domain.DislatchAwake, domain.DayAwake, feed2End, nil),
	}

	stats := ComputeDayStats(day, dayEvents, nil, nil)

	if stats.DayFeedCount != 2 {
		t.Errorf("DayFeedCount = %d, want 2", stats.DayFeedCount)
	}
	wantTotal := 20*time.Minute + 15*time.Minute
	if stats.DayTotalFeedTime != wantTotal {
		t.Errorf("DayTotalFeedTime = %v, want %v", stats.DayTotalFeedTime, wantTotal)
	}
	if stats.FeedTimeLeft != 20*time.Minute {
		t.Errorf("FeedTimeLeft = %v, want 20m", stats.FeedTimeLeft)
	}
	if stats.FeedTimeRight != 15*time.Minute {
		t.Errorf("FeedTimeRight = %v, want 15m", stats.FeedTimeRight)
	}
}

// TestComputeDayStats_DislatchAsleepCountsNap: when a feed ends with
// dislatch_asleep (transitioning Day_Feeding → DaySleeping), a nap starts.
func TestComputeDayStats_DislatchAsleepCountsNap(t *testing.T) {
	start := dayStart()
	feedStart := start.Add(1 * time.Hour)
	napTransitionAt := feedStart.Add(20 * time.Minute)
	napEnd := napTransitionAt.Add(60 * time.Minute)
	dayEnd := start.Add(12 * time.Hour)

	day := mkSession(1, domain.SessionKindDay, start, ptrTime(dayEnd))
	dayEvents := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartDay, domain.DayAwake, start, nil),
		mkEvent(2, domain.DayAwake, domain.StartFeed, domain.DayFeeding, feedStart, map[string]string{"breast": "L"}),
		mkEvent(3, domain.DayFeeding, domain.DislatchAsleep, domain.DaySleeping, napTransitionAt, map[string]string{"location": "on_me"}),
		mkEvent(4, domain.DaySleeping, domain.BabyWoke, domain.DayAwake, napEnd, nil),
	}

	stats := ComputeDayStats(day, dayEvents, nil, nil)
	if stats.NapCount != 1 {
		t.Errorf("NapCount = %d, want 1 (dislatch_asleep entered nap)", stats.NapCount)
	}
	if stats.TotalNapTime != 60*time.Minute {
		t.Errorf("TotalNapTime = %v, want 60m", stats.TotalNapTime)
	}
	if stats.DayFeedCount != 1 {
		t.Errorf("DayFeedCount = %d, want 1", stats.DayFeedCount)
	}
}

// --- ComputeCycleStats ---

func TestComputeCycleStats_BothHalves(t *testing.T) {
	start := dayStart()
	dayEnd := start.Add(12 * time.Hour)
	nightEnd := dayEnd.Add(10 * time.Hour)

	day := mkSession(1, domain.SessionKindDay, start, ptrTime(dayEnd))
	night := mkSession(2, domain.SessionKindNight, dayEnd, ptrTime(nightEnd))

	dayEvents := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartDay, domain.DayAwake, start, nil),
		mkEvent(2, domain.DayAwake, domain.StartSleep, domain.DaySleeping, start.Add(3*time.Hour), map[string]string{"location": "crib"}),
		mkEvent(3, domain.DaySleeping, domain.BabyWoke, domain.DayAwake, start.Add(4*time.Hour), nil),
	}
	nightEvents := []domain.Event{
		mkEvent(1, domain.DayAwake, domain.StartNight, domain.Awake, dayEnd, nil),
		mkEvent(2, domain.Awake, domain.StartResettle, domain.Resettling, dayEnd.Add(5*time.Minute), nil),
		mkEvent(3, domain.Resettling, domain.Settled, domain.SleepingCrib, dayEnd.Add(10*time.Minute), nil),
	}

	stats := ComputeCycleStats(day, night, dayEvents, nightEvents)

	if stats.Day == nil {
		t.Fatal("Day stats nil, want populated")
	}
	if stats.Night == nil {
		t.Fatal("Night stats nil, want populated")
	}
	if stats.Day.NapCount != 1 {
		t.Errorf("Day.NapCount = %d, want 1", stats.Day.NapCount)
	}
}

func TestComputeCycleStats_DayOnly_InProgress(t *testing.T) {
	start := time.Now().Add(-2 * time.Hour)
	day := mkSession(1, domain.SessionKindDay, start, nil)
	dayEvents := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartDay, domain.DayAwake, start, nil),
	}

	stats := ComputeCycleStats(day, nil, dayEvents, nil)

	if stats.Day == nil {
		t.Fatal("Day stats nil, want populated for in-progress cycle")
	}
	if stats.Night != nil {
		t.Errorf("Night stats = %+v, want nil for in-progress cycle", stats.Night)
	}
	if stats.Day.NapCount != 0 {
		t.Errorf("NapCount = %d, want 0", stats.Day.NapCount)
	}
}

func TestComputeCycleStats_NightOnly_Orphan(t *testing.T) {
	nightStart := dayStart().Add(12 * time.Hour)
	nightEnd := nightStart.Add(10 * time.Hour)
	night := mkSession(1, domain.SessionKindNight, nightStart, ptrTime(nightEnd))
	nightEvents := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, nightStart, nil),
		mkEvent(2, domain.Awake, domain.StartResettle, domain.Resettling, nightStart.Add(5*time.Minute), nil),
		mkEvent(3, domain.Resettling, domain.Settled, domain.SleepingCrib, nightStart.Add(10*time.Minute), nil),
	}

	stats := ComputeCycleStats(nil, night, nil, nightEvents)

	if stats.Day != nil {
		t.Errorf("Day stats = %+v, want nil for orphan cycle", stats.Day)
	}
	if stats.Night == nil {
		t.Fatal("Night stats nil, want populated")
	}
}

// --- AttachMovingAverages ---

// TestAttachMovingAverages_InsufficientHistory: window=3, only 2 cycles →
// no cycle gets an Avg.
func TestAttachMovingAverages_InsufficientHistory(t *testing.T) {
	summaries := []CycleSummary{
		{Stats: CycleStats{Night: &NightStats{TotalSleepTime: 8 * time.Hour}}},
		{Stats: CycleStats{Night: &NightStats{TotalSleepTime: 9 * time.Hour}}},
	}
	AttachMovingAverages(summaries, 3)

	for i, s := range summaries {
		if s.Avg != nil {
			t.Errorf("cycle %d: Avg = %+v, want nil (insufficient history)", i, s.Avg)
		}
	}
}

// TestAttachMovingAverages_Window3: five cycles, window=3 → cycles at
// indices 2, 3, 4 get Avgs.
func TestAttachMovingAverages_Window3(t *testing.T) {
	summaries := make([]CycleSummary, 5)
	sleeps := []time.Duration{6 * time.Hour, 8 * time.Hour, 10 * time.Hour, 7 * time.Hour, 9 * time.Hour}
	intraSleepFeeds := []time.Duration{30 * time.Minute, 45 * time.Minute, 60 * time.Minute, 15 * time.Minute, 30 * time.Minute}
	for i := range sleeps {
		summaries[i] = CycleSummary{Stats: CycleStats{Night: &NightStats{
			TotalSleepTime:     sleeps[i],
			IntraSleepFeedTime: intraSleepFeeds[i],
		}}}
	}

	AttachMovingAverages(summaries, 3)

	// Indices 0, 1 have no Avg (not enough prior history).
	for i := 0; i < 2; i++ {
		if summaries[i].Avg != nil {
			t.Errorf("cycle %d: Avg = %+v, want nil", i, summaries[i].Avg)
		}
	}
	// Index 2: avg of sleeps[0..2] = (6+8+10)/3 = 8h.
	if summaries[2].Avg == nil || summaries[2].Avg.Night == nil {
		t.Fatal("cycle 2: Avg or Avg.Night nil")
	}
	if summaries[2].Avg.Night.TotalSleepTime != 8*time.Hour {
		t.Errorf("cycle 2 avg TotalSleep = %v, want 8h", summaries[2].Avg.Night.TotalSleepTime)
	}
	// Index 2: avg intra-sleep feed time = (30+45+60)/3 = 45m.
	if summaries[2].Avg.Night.IntraSleepFeedTime != 45*time.Minute {
		t.Errorf("cycle 2 avg IntraSleepFeedTime = %v, want 45m", summaries[2].Avg.Night.IntraSleepFeedTime)
	}
	// Index 4: avg of sleeps[2..4] = (10+7+9)/3 = 26/3 ≈ 8h40m.
	wantAvg4 := (10*time.Hour + 7*time.Hour + 9*time.Hour) / 3
	if summaries[4].Avg.Night.TotalSleepTime != wantAvg4 {
		t.Errorf("cycle 4 avg TotalSleep = %v, want %v", summaries[4].Avg.Night.TotalSleepTime, wantAvg4)
	}
}

// TestAttachMovingAverages_MixedOrphans: window=3 over three cycles where
// two have day stats and one is orphan (no day). Day avg divides by count of
// non-nil halves, not by window.
func TestAttachMovingAverages_MixedOrphans(t *testing.T) {
	summaries := []CycleSummary{
		{Stats: CycleStats{Day: &DayStats{NapCount: 2, TotalNapTime: 2 * time.Hour, DayDuration: 11 * time.Hour, FeedTimeLeft: 30 * time.Minute, FeedTimeRight: 20 * time.Minute}}},
		{Stats: CycleStats{Night: &NightStats{TotalSleepTime: 8 * time.Hour}}}, // orphan (no day)
		{Stats: CycleStats{Day: &DayStats{NapCount: 4, TotalNapTime: 4 * time.Hour, DayDuration: 13 * time.Hour, FeedTimeLeft: 50 * time.Minute, FeedTimeRight: 40 * time.Minute}}},
	}

	AttachMovingAverages(summaries, 3)

	if summaries[2].Avg == nil {
		t.Fatal("cycle 2 Avg nil")
	}
	if summaries[2].Avg.Day == nil {
		t.Fatal("cycle 2 Avg.Day nil — averages should fold available day halves")
	}
	// Two day halves present: avg NapCount = (2+4)/2 = 3.
	if got := summaries[2].Avg.Day.NapCount; got != 3 {
		t.Errorf("avg NapCount = %d, want 3 (mean over 2 cycles that had day halves)", got)
	}
	// Avg total nap time = (2h + 4h) / 2 = 3h.
	if got := summaries[2].Avg.Day.TotalNapTime; got != 3*time.Hour {
		t.Errorf("avg TotalNapTime = %v, want 3h", got)
	}
	// Avg day duration = (11h + 13h) / 2 = 12h.
	if got := summaries[2].Avg.Day.DayDuration; got != 12*time.Hour {
		t.Errorf("avg DayDuration = %v, want 12h", got)
	}
	// Avg per-side feed = (30m + 50m) / 2 = 40m and (20m + 40m) / 2 = 30m.
	if got := summaries[2].Avg.Day.FeedTimeLeft; got != 40*time.Minute {
		t.Errorf("avg FeedTimeLeft = %v, want 40m", got)
	}
	if got := summaries[2].Avg.Day.FeedTimeRight; got != 30*time.Minute {
		t.Errorf("avg FeedTimeRight = %v, want 30m", got)
	}
}

// TestAttachMovingAverages_PreservesLists: moving-average CycleStats should
// NOT carry SleepBlocks / FeedTimes / WakeWindows — those are per-cycle lists
// that don't meaningfully average. Verify they're nil/empty in the Avg.
func TestAttachMovingAverages_PreservesLists(t *testing.T) {
	summaries := make([]CycleSummary, 3)
	for i := range summaries {
		summaries[i] = CycleSummary{Stats: CycleStats{
			Day: &DayStats{
				NapCount:    2,
				WakeWindows: []time.Duration{time.Hour, 2 * time.Hour},
			},
			Night: &NightStats{
				TotalSleepTime: 8 * time.Hour,
				SleepBlocks:    []time.Duration{3 * time.Hour, 4 * time.Hour},
				FeedTimes:      []time.Time{time.Now()},
			},
		}}
	}

	AttachMovingAverages(summaries, 3)

	avg := summaries[2].Avg
	if avg == nil {
		t.Fatal("Avg nil")
	}
	if avg.Day != nil && len(avg.Day.WakeWindows) > 0 {
		t.Errorf("avg.Day.WakeWindows = %v, want nil/empty", avg.Day.WakeWindows)
	}
	if avg.Night != nil {
		if len(avg.Night.SleepBlocks) > 0 {
			t.Errorf("avg.Night.SleepBlocks = %v, want nil/empty", avg.Night.SleepBlocks)
		}
		if len(avg.Night.FeedTimes) > 0 {
			t.Errorf("avg.Night.FeedTimes = %v, want nil/empty", avg.Night.FeedTimes)
		}
	}
}
