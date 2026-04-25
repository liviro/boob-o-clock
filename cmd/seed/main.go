// Command seed inserts plausible test data into a boob-o-clock database.
// Usage: go run ./cmd/seed -db ./dev.db
//
// Structure:
//   - Day -8: one orphan night (no paired day) — exercises the historical
//     pre-feature case. This tests the "day=null" branch of the cycle view.
//   - Days -7..-1: six full cycles, each a day session paired with the
//     following night. Day-start times vary around 7am (±30 min) so the
//     cycle-bar's 7am epoch is visible in the stacked timelines.
//   - Today: an in-progress day session with a few events already logged.
//
// No Ferber: removed per owner preference.
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
	flag.Parse()

	os.Remove(*dbPath) // start fresh

	s, err := store.New(*dbPath)
	if err != nil {
		log.Fatal(err)
	}
	defer s.Close()

	if err := seedAll(s); err != nil {
		log.Fatal(err)
	}
	fmt.Printf("Seeded plausible day+night cycles into %s\n", *dbPath)
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

func seedAll(s *store.Store) error {
	mid := midnightLocal()

	// Orphan night: 8 days ago, 8:00 PM → 7 days ago, 6:45 AM.
	orphanStart := atHourMin(mid.AddDate(0, 0, -8), 20, 0)
	orphanEnd := atHourMin(mid.AddDate(0, 0, -7), 6, 45)
	if err := seedNight(s, orphanStart, orphanEnd, nightOneWakeup()); err != nil {
		return fmt.Errorf("orphan night: %w", err)
	}

	// Six complete cycles over days -6..-1. Under contiguous-chain semantics,
	// each cycle's nightEnd must equal the next cycle's dayStart (no gaps).
	// Day start times vary to exercise the 7am cycle boundary — some start
	// before 7am (early wake), some after (late wake = late-sleeping baby,
	// which visualizes as a sleep sliver at the left of the current cycle).
	cycles := []cycleSpec{
		// Day -6: typical 7am start. Next day wakes at 6:45 (early).
		{
			dayStart:   atHourMin(mid.AddDate(0, 0, -6), 7, 0),
			nightStart: atHourMin(mid.AddDate(0, 0, -6), 19, 30),
			nightEnd:   atHourMin(mid.AddDate(0, 0, -5), 6, 45),
			day:        dayTwoNaps(),
			night:      nightOneWakeup(),
		},
		// Day -5: early morning wake (6:45am, just before the 7am epoch).
		// Night runs long (until 7:15am) → visualizes as a 15-min sleep
		// sliver at the left of day -4's cycle bar.
		{
			dayStart:   atHourMin(mid.AddDate(0, 0, -5), 6, 45),
			nightStart: atHourMin(mid.AddDate(0, 0, -5), 19, 45),
			nightEnd:   atHourMin(mid.AddDate(0, 0, -4), 7, 15),
			day:        dayThreeNaps(),
			night:      nightTwoWakeups(),
		},
		// Day -4: late wake from previous night (7:15am — sleep tail visible
		// at the left of this bar).
		{
			dayStart:   atHourMin(mid.AddDate(0, 0, -4), 7, 15),
			nightStart: atHourMin(mid.AddDate(0, 0, -4), 19, 30),
			nightEnd:   atHourMin(mid.AddDate(0, 0, -3), 6, 30),
			day:        dayCarNap(),
			night:      nightOneWakeup(),
		},
		// Day -3: very early wake (6:30am). Night runs until 7:30am next day
		// — produces a noticeable 30-min sleep sliver at the left of day -2's
		// cycle bar.
		{
			dayStart:   atHourMin(mid.AddDate(0, 0, -3), 6, 30),
			nightStart: atHourMin(mid.AddDate(0, 0, -3), 19, 30),
			nightEnd:   atHourMin(mid.AddDate(0, 0, -2), 7, 30),
			day:        dayThreeNaps(),
			night:      nightTwoWakeups(),
		},
		// Day -2: late wake (7:30am — inherits the 30-min sleep sliver).
		{
			dayStart:   atHourMin(mid.AddDate(0, 0, -2), 7, 30),
			nightStart: atHourMin(mid.AddDate(0, 0, -2), 20, 0),
			nightEnd:   atHourMin(mid.AddDate(0, 0, -1), 7, 0),
			day:        dayTwoNaps(),
			night:      nightOneWakeup(),
		},
		// Day -1: typical 7am wake.
		{
			dayStart:   atHourMin(mid.AddDate(0, 0, -1), 7, 0),
			nightStart: atHourMin(mid.AddDate(0, 0, -1), 19, 30),
			nightEnd:   atHourMin(mid, 6, 50),
			day:        dayThreeNaps(),
			night:      nightOneWakeup(),
		},
	}

	for i, c := range cycles {
		if err := seedDay(s, c.dayStart, c.nightStart, c.day); err != nil {
			return fmt.Errorf("cycle %d day: %w", i+1, err)
		}
		if err := seedNight(s, c.nightStart, c.nightEnd, c.night); err != nil {
			return fmt.Errorf("cycle %d night: %w", i+1, err)
		}
	}

	// Today's in-progress day. Started at 6:50am; we've logged a morning
	// feed and one nap so far. The session stays open (no EndSession call).
	todayStart := atHourMin(mid, 6, 50)
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
