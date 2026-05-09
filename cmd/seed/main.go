// Command seed inserts plausible test data into a boob-o-clock database.
// Usage: go run ./cmd/seed -db ./dev.db -days 40
//
// Structure:
//   - One orphan night (no paired day) at -(days+2) → -(days+1) — exercises
//     the historical pre-feature case (the "day=null" branch of the cycle
//     view).
//   - `days` full cycles spanning -days .. -1, each a day session paired
//     with the following night. Wake times and night-start times rotate
//     through a fixed pattern (some 6:30 / 7:30 wakes for variety around
//     the 7am cycle-bar epoch). Day and night activity fixtures rotate
//     independently so longer windows stay visually varied.
//   - Today: an in-progress day session that starts when the last cycle's
//     night ends, with a few events already logged.
//
// No Ferber / Chair: kept simple per owner preference.
package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/liviro/boob-o-clock/internal/domain"
	"github.com/liviro/boob-o-clock/internal/store"
)

func main() {
	dbPath := flag.String("db", "./dev.db", "path to SQLite database")
	days := flag.Int("days", 40, "number of complete day+night cycles to seed")
	flag.Parse()

	os.Remove(*dbPath) // start fresh

	s, err := store.New(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	if err := seedAll(s, *days); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Seeded %d day+night cycles into %s\n", *days, *dbPath)
	fmt.Printf("To point the dev server at it: go run ./cmd/server -db %s\n", *dbPath)
}

// --- time helpers ---

// midnightLocal returns 00:00 local time for today.
func midnightLocal() time.Time {
	now := time.Now()
	return time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, time.Local)
}

// atHourMin returns t with hours+minutes set.
func atHourMin(t time.Time, h, m int) time.Time {
	return t.Add(time.Duration(h)*time.Hour + time.Duration(m)*time.Minute)
}

// --- specs ---

type dayActivity struct {
	// offset from day start at which this activity begins
	offset time.Duration
	kind   string // "feed", "nap", "poop"
	// feed:
	breast   string
	durMins  int
	// nap:
	location string
}

type nightBlock struct {
	feedBreast    string
	feedMins      int
	sleepOnMeMins int
	resettleMins  int // 0 = transfer succeeds directly
	cribMins      int // minutes in crib before waking (0 = still sleeping — in-progress)
	stroller      bool
	strollMins    int
	strollerMins  int
	poopMins      int // if > 0, poop from awake before the feed/stroller
}

// --- night fixtures ---

// nightOneWakeup is the typical "slept pretty well" night: feed down, sleep,
// one wakeup in the middle for a feed, back to crib till morning.
func nightOneWakeup() []nightBlock {
	return []nightBlock{
		{feedBreast: "L", feedMins: 18, sleepOnMeMins: 5, cribMins: 240},
		{feedBreast: "R", feedMins: 14, sleepOnMeMins: 4, resettleMins: 6, cribMins: 200},
	}
}

// nightTwoWakeups has two mid-night feeds.
func nightTwoWakeups() []nightBlock {
	return []nightBlock{
		{feedBreast: "R", feedMins: 16, sleepOnMeMins: 4, cribMins: 180},
		{feedBreast: "L", feedMins: 15, sleepOnMeMins: 5, resettleMins: 8, cribMins: 150},
		{feedBreast: "R", feedMins: 10, sleepOnMeMins: 3, cribMins: 120},
	}
}

// --- day fixtures ---

// dayTwoNaps: crib morning nap, stroller afternoon nap, 3 feeds.
func dayTwoNaps() []dayActivity {
	return []dayActivity{
		{offset: 30 * time.Minute, kind: "feed", breast: "L", durMins: 20},
		{offset: 2*time.Hour + 30*time.Minute, kind: "nap", location: "crib", durMins: 75},
		{offset: 4 * time.Hour, kind: "feed", breast: "R", durMins: 15},
		{offset: 6 * time.Hour, kind: "nap", location: "stroller", durMins: 60},
		{offset: 7 * time.Hour, kind: "poop", durMins: 5},
		{offset: 9 * time.Hour, kind: "feed", breast: "L", durMins: 18},
	}
}

// dayThreeNaps: three naps, four feeds, light day.
func dayThreeNaps() []dayActivity {
	return []dayActivity{
		{offset: 20 * time.Minute, kind: "feed", breast: "R", durMins: 18},
		{offset: 2 * time.Hour, kind: "nap", location: "crib", durMins: 60},
		{offset: 3*time.Hour + 30*time.Minute, kind: "feed", breast: "L", durMins: 16},
		{offset: 5*time.Hour + 30*time.Minute, kind: "nap", location: "crib", durMins: 90},
		{offset: 7*time.Hour + 30*time.Minute, kind: "feed", breast: "R", durMins: 14},
		{offset: 9 * time.Hour, kind: "nap", location: "on_me", durMins: 30},
		{offset: 10*time.Hour + 30*time.Minute, kind: "feed", breast: "L", durMins: 15},
	}
}

// dayCarNap: one crib nap + one car nap (varied venue).
func dayCarNap() []dayActivity {
	return []dayActivity{
		{offset: 45 * time.Minute, kind: "feed", breast: "R", durMins: 16},
		{offset: 2*time.Hour + 30*time.Minute, kind: "nap", location: "crib", durMins: 90},
		{offset: 4*time.Hour + 30*time.Minute, kind: "feed", breast: "L", durMins: 18},
		{offset: 6 * time.Hour, kind: "poop", durMins: 5},
		{offset: 7 * time.Hour, kind: "nap", location: "car", durMins: 45},
		{offset: 9 * time.Hour, kind: "feed", breast: "R", durMins: 15},
	}
}

// --- cycle fixture ---

type cycleSpec struct {
	dayStart   time.Time // when the day session starts
	nightStart time.Time // when the night starts (also when the day ends — chain advance)
	nightEnd   time.Time
	day        []dayActivity
	night      []nightBlock
}

func seedAll(s *store.Store, days int) error {
	mid := midnightLocal()

	// Orphan night sits two days before the earliest cycle so the inter-cycle
	// gap (no paired day) is unambiguous regardless of how many cycles follow.
	orphanStart := atHourMin(mid.AddDate(0, 0, -(days+2)), 20, 0)
	orphanEnd := atHourMin(mid.AddDate(0, 0, -(days+1)), 6, 45)
	if err := seedNight(s, orphanStart, orphanEnd, nightOneWakeup()); err != nil {
		return fmt.Errorf("orphan night: %w", err)
	}

	cycles := generateCycles(days, mid)
	for i, c := range cycles {
		if err := seedDay(s, c.dayStart, c.nightStart, c.day); err != nil {
			return fmt.Errorf("cycle %d day: %w", i+1, err)
		}
		if err := seedNight(s, c.nightStart, c.nightEnd, c.night); err != nil {
			return fmt.Errorf("cycle %d night: %w", i+1, err)
		}
	}

	// In-progress day starts where the last cycle's night ended so the chain
	// stays unbroken; falls back to a sensible default when days == 0.
	var todayStart time.Time
	if len(cycles) > 0 {
		todayStart = cycles[len(cycles)-1].nightEnd
	} else {
		todayStart = atHourMin(mid, 6, 50)
	}
	now := time.Now()
	// Only include activities that can fit before "now".
	todayActivities := []dayActivity{
		{offset: 25 * time.Minute, kind: "feed", breast: "L", durMins: 20},
	}
	// Add a nap if it's already afternoon.
	if now.Sub(todayStart) > 3*time.Hour {
		todayActivities = append(todayActivities, dayActivity{
			offset: 2 * time.Hour, kind: "nap", location: "crib", durMins: 75,
		})
	}
	// Add a second feed if it's past midday.
	if now.Sub(todayStart) > 5*time.Hour {
		todayActivities = append(todayActivities, dayActivity{
			offset: 4 * time.Hour, kind: "feed", breast: "R", durMins: 15,
		})
	}
	if err := seedInProgressDay(s, todayStart, todayActivities); err != nil {
		return fmt.Errorf("in-progress day: %w", err)
	}

	return nil
}

// generateCycles produces n contiguous day+night cycles spanning days -n..-1,
// rotating through wake times, night-start times, and activity fixtures so
// longer windows stay visually varied. Each cycle's nightEnd matches the next
// cycle's dayStart so the chain is unbroken.
func generateCycles(n int, today time.Time) []cycleSpec {
	if n <= 0 {
		return nil
	}

	nightFixtures := [][]nightBlock{nightOneWakeup(), nightTwoWakeups()}
	dayFixtures := [][]dayActivity{dayTwoNaps(), dayThreeNaps(), dayCarNap()}

	type clock struct{ h, m int }
	wakeTimes := []clock{
		{7, 0}, {6, 45}, {7, 15}, {6, 30}, {7, 30}, {7, 0},
	}
	nightStartTimes := []clock{
		{19, 30}, {19, 45}, {19, 30}, {19, 30}, {20, 0}, {19, 30},
	}

	cycles := make([]cycleSpec, n)
	for i := 0; i < n; i++ {
		dayOff := -(n - i)
		ws := wakeTimes[i%len(wakeTimes)]
		ns := nightStartTimes[i%len(nightStartTimes)]
		nextWake := wakeTimes[(i+1)%len(wakeTimes)]
		cycles[i] = cycleSpec{
			dayStart:   atHourMin(today.AddDate(0, 0, dayOff), ws.h, ws.m),
			nightStart: atHourMin(today.AddDate(0, 0, dayOff), ns.h, ns.m),
			nightEnd:   atHourMin(today.AddDate(0, 0, dayOff+1), nextWake.h, nextWake.m),
			day:        dayFixtures[i%len(dayFixtures)],
			night:      nightFixtures[i%len(nightFixtures)],
		}
	}
	return cycles
}

// --- seed helpers ---

// eventAppender returns a closure that builds and inserts events, advancing
// a cursor. The closure reports the first error via a captured variable.
type eventAppender struct {
	store     *store.Store
	sessionID int64
	cursor    time.Time
	err       error
}

func newAppender(s *store.Store, sessionID int64, start time.Time) *eventAppender {
	return &eventAppender{store: s, sessionID: sessionID, cursor: start}
}

func (ea *eventAppender) add(from domain.State, action domain.Action, to domain.State, meta map[string]string) {
	if ea.err != nil {
		return
	}
	evt := &domain.Event{
		SessionID: ea.sessionID,
		FromState: from,
		Action:    action,
		ToState:   to,
		Timestamp: ea.cursor,
		Metadata:  meta,
	}
	if err := ea.store.AddEvent(evt); err != nil {
		ea.err = err
	}
}

func (ea *eventAppender) tick(d time.Duration) {
	ea.cursor = ea.cursor.Add(d)
}

func (ea *eventAppender) advanceTo(t time.Time) {
	ea.cursor = t
}

func seedNight(s *store.Store, start, end time.Time, blocks []nightBlock) error {
	night, err := s.CreateSession(domain.SessionKindNight, start, false, 0, false)
	if err != nil {
		return err
	}

	ea := newAppender(s, night.ID, start)
	ea.add(domain.NightOff, domain.StartNight, domain.Awake, nil)

	for i, b := range blocks {
		isLast := i == len(blocks)-1

		if b.poopMins > 0 {
			ea.add(domain.Awake, domain.PoopStart, domain.Poop, nil)
			ea.tick(time.Duration(b.poopMins) * time.Minute)
			ea.add(domain.Poop, domain.PoopDone, domain.Awake, nil)
		}

		if b.stroller {
			ea.add(domain.Awake, domain.StartStrolling, domain.Strolling, nil)
			ea.tick(time.Duration(b.strollMins) * time.Minute)
			ea.add(domain.Strolling, domain.FellAsleep, domain.SleepingStroller, nil)
			if b.cribMins == 0 && isLast {
				break // in-progress, end mid-stroller-sleep
			}
			ea.tick(time.Duration(b.strollerMins) * time.Minute)
			ea.add(domain.SleepingStroller, domain.BabyWoke, domain.Awake, nil)
			continue
		}

		// Feed → on-me → transfer → (resettle) → crib
		ea.add(domain.Awake, domain.StartFeed, domain.Feeding, map[string]string{"breast": b.feedBreast})
		ea.tick(time.Duration(b.feedMins) * time.Minute)
		ea.add(domain.Feeding, domain.DislatchAsleep, domain.SleepingOnMe, nil)
		ea.tick(time.Duration(b.sleepOnMeMins) * time.Minute)
		ea.add(domain.SleepingOnMe, domain.StartTransfer, domain.Transferring, nil)

		if b.resettleMins > 0 {
			ea.add(domain.Transferring, domain.TransferNeedResettle, domain.Resettling, nil)
			ea.tick(time.Duration(b.resettleMins) * time.Minute)
			ea.add(domain.Resettling, domain.Settled, domain.SleepingCrib, nil)
		} else {
			ea.add(domain.Transferring, domain.TransferSuccess, domain.SleepingCrib, nil)
		}

		if b.cribMins == 0 && isLast {
			break // in-progress, baby still sleeping
		}
		if isLast {
			// Last block: crib sleep runs all the way to the night's end
			// time, then baby wakes. baby_woke fires AT nightEnd so the
			// session's last event matches ended_at, and to_state=Awake
			// aligns with the next day's start_day (Awake → DayAwake).
			ea.advanceTo(end)
		} else {
			ea.tick(time.Duration(b.cribMins) * time.Minute)
		}
		ea.add(domain.SleepingCrib, domain.BabyWoke, domain.Awake, nil)
	}
	if ea.err != nil {
		return ea.err
	}

	return s.EndSession(night.ID, end)
}

func seedDay(s *store.Store, start, end time.Time, activities []dayActivity) error {
	day, err := s.CreateSession(domain.SessionKindDay, start, false, 0, false)
	if err != nil {
		return err
	}

	ea := newAppender(s, day.ID, start)
	ea.add(domain.NightOff, domain.StartDay, domain.DayAwake, nil)

	for _, a := range activities {
		ea.advanceTo(start.Add(a.offset))
		switch a.kind {
		case "feed":
			ea.add(domain.DayAwake, domain.StartFeed, domain.DayFeeding, map[string]string{"breast": a.breast})
			ea.tick(time.Duration(a.durMins) * time.Minute)
			ea.add(domain.DayFeeding, domain.DislatchAwake, domain.DayAwake, nil)
		case "nap":
			ea.add(domain.DayAwake, domain.StartSleep, domain.DaySleeping, map[string]string{"location": a.location})
			ea.tick(time.Duration(a.durMins) * time.Minute)
			ea.add(domain.DaySleeping, domain.BabyWoke, domain.DayAwake, nil)
		case "poop":
			ea.add(domain.DayAwake, domain.PoopStart, domain.DayPoop, nil)
			ea.tick(time.Duration(a.durMins) * time.Minute)
			ea.add(domain.DayPoop, domain.PoopDone, domain.DayAwake, nil)
		default:
			return fmt.Errorf("unknown day activity kind %q", a.kind)
		}
	}
	if ea.err != nil {
		return ea.err
	}

	return s.EndSession(day.ID, end)
}

// seedInProgressDay is seedDay minus the EndSession call — the day remains
// open so the Tracker renders a live in-progress state.
func seedInProgressDay(s *store.Store, start time.Time, activities []dayActivity) error {
	day, err := s.CreateSession(domain.SessionKindDay, start, false, 0, false)
	if err != nil {
		return err
	}

	ea := newAppender(s, day.ID, start)
	ea.add(domain.NightOff, domain.StartDay, domain.DayAwake, nil)

	for _, a := range activities {
		ea.advanceTo(start.Add(a.offset))
		switch a.kind {
		case "feed":
			ea.add(domain.DayAwake, domain.StartFeed, domain.DayFeeding, map[string]string{"breast": a.breast})
			ea.tick(time.Duration(a.durMins) * time.Minute)
			ea.add(domain.DayFeeding, domain.DislatchAwake, domain.DayAwake, nil)
		case "nap":
			ea.add(domain.DayAwake, domain.StartSleep, domain.DaySleeping, map[string]string{"location": a.location})
			ea.tick(time.Duration(a.durMins) * time.Minute)
			ea.add(domain.DaySleeping, domain.BabyWoke, domain.DayAwake, nil)
		case "poop":
			ea.add(domain.DayAwake, domain.PoopStart, domain.DayPoop, nil)
			ea.tick(time.Duration(a.durMins) * time.Minute)
			ea.add(domain.DayPoop, domain.PoopDone, domain.DayAwake, nil)
		}
	}
	return ea.err
}
