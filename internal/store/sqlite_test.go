package store

import (
	"database/sql"
	"encoding/json"
	"maps"
	"path/filepath"
	"testing"
	"time"

	"github.com/liviro/boob-o-clock/internal/domain"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create store: %v", err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

// openLegacyDB creates a fresh SQLite file at dbPath with the pre-day-mode
// schema (nights table + events with night_id FK). Returns the open handle
// so the caller can seed rows; closing is the caller's responsibility.
// Shared by the three migration tests so the legacy shape is defined once.
func openLegacyDB(t *testing.T, dbPath string) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("open raw: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE nights (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			started_at TEXT NOT NULL,
			ended_at TEXT,
			created_at TEXT NOT NULL,
			ferber_enabled INTEGER NOT NULL DEFAULT 0,
			ferber_night_number INTEGER
		)`); err != nil {
		t.Fatalf("create nights: %v", err)
	}
	if _, err := db.Exec(`
		CREATE TABLE events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			night_id INTEGER NOT NULL REFERENCES nights(id) ON DELETE CASCADE,
			from_state TEXT NOT NULL,
			action TEXT NOT NULL,
			to_state TEXT NOT NULL,
			timestamp TEXT NOT NULL,
			metadata TEXT,
			created_at TEXT NOT NULL,
			seq INTEGER NOT NULL
		)`); err != nil {
		t.Fatalf("create events: %v", err)
	}
	return db
}

func TestSchemaCreation(t *testing.T) {
	s := newTestStore(t)

	// Opening again should be idempotent (migrations run twice without error).
	s2, err := New(s.dbPath)
	if err != nil {
		t.Fatalf("second open failed: %v", err)
	}
	s2.Close()
}

func TestCreateSessionNight(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().Truncate(time.Millisecond)

	sess, err := s.CreateSession(domain.SessionKindNight, now, false, 0, false)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if sess.ID == 0 {
		t.Error("expected non-zero session ID")
	}
	if sess.Kind != domain.SessionKindNight {
		t.Errorf("Kind = %s, want night", sess.Kind)
	}
	if !sess.StartedAt.Equal(now) {
		t.Errorf("StartedAt = %v, want %v", sess.StartedAt, now)
	}
	if sess.EndedAt != nil {
		t.Error("EndedAt should be nil for new session")
	}
}

func TestCreateSessionDay(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().Truncate(time.Millisecond)

	sess, err := s.CreateSession(domain.SessionKindDay, now, false, 0, false)
	if err != nil {
		t.Fatalf("CreateSession day: %v", err)
	}
	if sess.Kind != domain.SessionKindDay {
		t.Errorf("Kind = %s, want day", sess.Kind)
	}
	if sess.FerberEnabled {
		t.Error("day session should never have FerberEnabled")
	}
}

func TestCreateNightWithFerber(t *testing.T) {
	s := newTestStore(t)
	started := time.Now()
	sess, err := s.CreateSession(domain.SessionKindNight, started, true, 3, false)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if !sess.FerberEnabled {
		t.Error("FerberEnabled = false, want true")
	}
	if sess.FerberNightNumber == nil || *sess.FerberNightNumber != 3 {
		t.Errorf("FerberNightNumber = %v, want 3", sess.FerberNightNumber)
	}

	roundTrip, _, err := s.CurrentSession()
	if err != nil {
		t.Fatalf("CurrentSession: %v", err)
	}
	if roundTrip == nil || !roundTrip.FerberEnabled || *roundTrip.FerberNightNumber != 3 {
		t.Errorf("round-trip lost Ferber fields: %+v", roundTrip)
	}
}

func TestCreateNightWithChair(t *testing.T) {
	s := newTestStore(t)
	started := time.Now()
	sess, err := s.CreateSession(domain.SessionKindNight, started, false, 0, true)
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if !sess.ChairEnabled {
		t.Error("ChairEnabled = false, want true")
	}
	if sess.FerberEnabled {
		t.Error("FerberEnabled should be false on a chair session")
	}

	roundTrip, _, err := s.CurrentSession()
	if err != nil {
		t.Fatalf("CurrentSession: %v", err)
	}
	if roundTrip == nil || !roundTrip.ChairEnabled {
		t.Errorf("round-trip lost ChairEnabled: %+v", roundTrip)
	}
}

func TestEndSession(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().Truncate(time.Millisecond)

	sess, _ := s.CreateSession(domain.SessionKindNight, now, false, 0, false)
	endTime := now.Add(8 * time.Hour)

	if err := s.EndSession(sess.ID, endTime); err != nil {
		t.Fatalf("EndSession: %v", err)
	}

	got, _, err := s.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got.EndedAt == nil || !got.EndedAt.Equal(endTime) {
		t.Errorf("EndedAt = %v, want %v", got.EndedAt, endTime)
	}
}

func TestOneOpenSessionInvariant(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().Truncate(time.Millisecond)

	// First open session is fine.
	if _, err := s.CreateSession(domain.SessionKindNight, now, false, 0, false); err != nil {
		t.Fatalf("first CreateSession: %v", err)
	}

	// Second open session (without closing the first) must fail via the
	// unique partial index on ended_at.
	if _, err := s.CreateSession(domain.SessionKindDay, now.Add(time.Hour), false, 0, false); err == nil {
		t.Error("expected unique constraint error on second open session, got none")
	}
}

func TestLastFeedStart(t *testing.T) {
	s := newTestStore(t)

	// No events → nil.
	if got, err := s.LastFeedStart(); err != nil || got != nil {
		t.Fatalf("empty: got=%v err=%v, want nil/nil", got, err)
	}

	now := time.Now().Truncate(time.Millisecond)
	// Two sessions: a historical night with feeds, a current day with a later feed.
	night, _ := s.CreateSession(domain.SessionKindNight, now.Add(-10*time.Hour), false, 0, false)
	s.EndSession(night.ID, now.Add(-4*time.Hour))
	day, _ := s.CreateSession(domain.SessionKindDay, now.Add(-4*time.Hour), false, 0, false)

	nightFeed := now.Add(-8 * time.Hour)
	if err := s.AddEvent(&domain.Event{
		SessionID: night.ID, FromState: domain.Awake, Action: domain.StartFeed, ToState: domain.Feeding,
		Timestamp: nightFeed, Metadata: map[string]string{"breast": "L"},
	}); err != nil {
		t.Fatalf("night feed: %v", err)
	}
	dayFeed := now.Add(-2 * time.Hour)
	if err := s.AddEvent(&domain.Event{
		SessionID: day.ID, FromState: domain.DayAwake, Action: domain.StartFeed, ToState: domain.DayFeeding,
		Timestamp: dayFeed, Metadata: map[string]string{"breast": "R"},
	}); err != nil {
		t.Fatalf("day feed: %v", err)
	}
	// A non-feed event later — must not be picked up.
	if err := s.AddEvent(&domain.Event{
		SessionID: day.ID, FromState: domain.DayFeeding, Action: domain.DislatchAwake, ToState: domain.DayAwake,
		Timestamp: now.Add(-1 * time.Hour),
	}); err != nil {
		t.Fatalf("dislatch: %v", err)
	}

	got, err := s.LastFeedStart()
	if err != nil {
		t.Fatalf("LastFeedStart: %v", err)
	}
	if got == nil || !got.Equal(dayFeed) {
		t.Errorf("LastFeedStart = %v, want %v (most recent across sessions)", got, dayFeed)
	}
}

func TestLastWakeFromSleep(t *testing.T) {
	s := newTestStore(t)

	// No events → nil.
	if got, err := s.LastWakeFromSleep(); err != nil || got != nil {
		t.Fatalf("empty: got=%v err=%v, want nil/nil", got, err)
	}

	now := time.Now().Truncate(time.Millisecond)
	night, _ := s.CreateSession(domain.SessionKindNight, now.Add(-12*time.Hour), false, 0, false)
	s.EndSession(night.ID, now.Add(-6*time.Hour))
	day, _ := s.CreateSession(domain.SessionKindDay, now.Add(-6*time.Hour), false, 0, false)

	// Night sleep (transfer_success → sleeping_crib). Not a wake; must be ignored.
	if err := s.AddEvent(&domain.Event{
		SessionID: night.ID, FromState: domain.Transferring, Action: domain.TransferSuccess, ToState: domain.SleepingCrib,
		Timestamp: now.Add(-10 * time.Hour),
	}); err != nil {
		t.Fatalf("night sleep: %v", err)
	}
	// Night wake (baby_woke → awake), the previous wake.
	if err := s.AddEvent(&domain.Event{
		SessionID: night.ID, FromState: domain.SleepingCrib, Action: domain.BabyWoke, ToState: domain.Awake,
		Timestamp: now.Add(-7 * time.Hour),
	}); err != nil {
		t.Fatalf("night wake: %v", err)
	}
	// Day nap start (start_sleep → day_sleeping). Not a wake; must be ignored.
	if err := s.AddEvent(&domain.Event{
		SessionID: day.ID, FromState: domain.DayAwake, Action: domain.StartSleep, ToState: domain.DaySleeping,
		Timestamp: now.Add(-3 * time.Hour), Metadata: map[string]string{"location": "crib"},
	}); err != nil {
		t.Fatalf("day nap: %v", err)
	}
	// Day nap end — baby_woke → day_awake. Most recent wake transition.
	dayWake := now.Add(-1 * time.Hour)
	if err := s.AddEvent(&domain.Event{
		SessionID: day.ID, FromState: domain.DaySleeping, Action: domain.BabyWoke, ToState: domain.DayAwake,
		Timestamp: dayWake,
	}); err != nil {
		t.Fatalf("wake: %v", err)
	}

	got, err := s.LastWakeFromSleep()
	if err != nil {
		t.Fatalf("LastWakeFromSleep: %v", err)
	}
	if got == nil || !got.Equal(dayWake) {
		t.Errorf("LastWakeFromSleep = %v, want %v (most recent transition out of sleep)", got, dayWake)
	}
}

func TestAddEventAndGetSession(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().Truncate(time.Millisecond)

	sess, _ := s.CreateSession(domain.SessionKindNight, now, false, 0, false)

	evt := &domain.Event{
		SessionID: sess.ID,
		FromState: domain.NightOff,
		Action:    domain.StartNight,
		ToState:   domain.Awake,
		Timestamp: now,
	}
	if err := s.AddEvent(evt); err != nil {
		t.Fatalf("AddEvent: %v", err)
	}
	if evt.ID == 0 {
		t.Error("expected non-zero event ID")
	}
	if evt.Seq != 1 {
		t.Errorf("first event Seq = %d, want 1", evt.Seq)
	}

	evt2 := &domain.Event{
		SessionID: sess.ID,
		FromState: domain.Awake,
		Action:    domain.StartFeed,
		ToState:   domain.Feeding,
		Timestamp: now.Add(5 * time.Minute),
		Metadata:  map[string]string{"breast": "L"},
	}
	if err := s.AddEvent(evt2); err != nil {
		t.Fatalf("AddEvent 2: %v", err)
	}
	if evt2.Seq != 2 {
		t.Errorf("second event Seq = %d, want 2", evt2.Seq)
	}

	_, events, err := s.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}
	if events[0].Action != domain.StartNight {
		t.Errorf("first event action = %s, want start_night", events[0].Action)
	}
	if events[1].Metadata["breast"] != "L" {
		t.Errorf("second event breast = %s, want L", events[1].Metadata["breast"])
	}
}

func TestPopEvent(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().Truncate(time.Millisecond)

	sess, _ := s.CreateSession(domain.SessionKindNight, now, false, 0, false)

	s.AddEvent(&domain.Event{
		SessionID: sess.ID, FromState: domain.NightOff,
		Action: domain.StartNight, ToState: domain.Awake, Timestamp: now,
	})
	s.AddEvent(&domain.Event{
		SessionID: sess.ID, FromState: domain.Awake,
		Action: domain.StartFeed, ToState: domain.Feeding,
		Timestamp: now.Add(5 * time.Minute),
		Metadata:  map[string]string{"breast": "L"},
	})

	popped, err := s.PopEvent(sess.ID)
	if err != nil {
		t.Fatalf("PopEvent: %v", err)
	}
	if popped.Action != domain.StartFeed {
		t.Errorf("popped action = %s, want start_feed", popped.Action)
	}

	_, events, _ := s.GetSession(sess.ID)
	if len(events) != 1 {
		t.Fatalf("got %d events after pop, want 1", len(events))
	}
}

func TestPopEventEmpty(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().Truncate(time.Millisecond)

	sess, _ := s.CreateSession(domain.SessionKindNight, now, false, 0, false)

	if _, err := s.PopEvent(sess.ID); err == nil {
		t.Error("PopEvent on empty session should return error")
	}
}

func TestCurrentSession(t *testing.T) {
	s := newTestStore(t)

	// No active session.
	sess, _, err := s.CurrentSession()
	if err != nil {
		t.Fatalf("CurrentSession: %v", err)
	}
	if sess != nil {
		t.Error("expected nil session when none active")
	}

	now := time.Now().Truncate(time.Millisecond)
	opened, _ := s.CreateSession(domain.SessionKindNight, now, false, 0, false)
	s.AddEvent(&domain.Event{
		SessionID: opened.ID, FromState: domain.NightOff,
		Action: domain.StartNight, ToState: domain.Awake, Timestamp: now,
	})

	sess, events, err := s.CurrentSession()
	if err != nil {
		t.Fatalf("CurrentSession: %v", err)
	}
	if sess == nil || sess.ID != opened.ID {
		t.Fatalf("CurrentSession ID = %v, want %d", sess, opened.ID)
	}
	if len(events) != 1 {
		t.Errorf("got %d events, want 1", len(events))
	}

	s.EndSession(opened.ID, now.Add(8*time.Hour))
	sess, _, err = s.CurrentSession()
	if err != nil {
		t.Fatalf("CurrentSession after end: %v", err)
	}
	if sess != nil {
		t.Error("expected nil session after ending")
	}
}

func TestListSessions(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().Truncate(time.Millisecond)

	n1, _ := s.CreateSession(domain.SessionKindNight, now, false, 0, false)
	s.EndSession(n1.ID, now.Add(8*time.Hour))

	d1, _ := s.CreateSession(domain.SessionKindDay, now.Add(8*time.Hour), false, 0, false)
	s.EndSession(d1.ID, now.Add(22*time.Hour))

	n2, _ := s.CreateSession(domain.SessionKindNight, now.Add(22*time.Hour), false, 0, false)
	s.EndSession(n2.ID, now.Add(32*time.Hour))

	// Unfiltered: all three.
	all, err := s.ListSessions(now.Add(-time.Hour), now.Add(48*time.Hour), "")
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(all) != 3 {
		t.Errorf("got %d sessions, want 3", len(all))
	}

	// Nights only.
	nights, err := s.ListSessions(now.Add(-time.Hour), now.Add(48*time.Hour), domain.SessionKindNight)
	if err != nil {
		t.Fatalf("ListSessions night: %v", err)
	}
	if len(nights) != 2 {
		t.Errorf("got %d nights, want 2", len(nights))
	}

	// Days only.
	days, err := s.ListSessions(now.Add(-time.Hour), now.Add(48*time.Hour), domain.SessionKindDay)
	if err != nil {
		t.Fatalf("ListSessions day: %v", err)
	}
	if len(days) != 1 {
		t.Errorf("got %d days, want 1", len(days))
	}
}

func TestGetEventsForSessions(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().Truncate(time.Millisecond)

	n1, _ := s.CreateSession(domain.SessionKindNight, now, false, 0, false)
	s.AddEvent(&domain.Event{
		SessionID: n1.ID, FromState: domain.NightOff,
		Action: domain.StartNight, ToState: domain.Awake, Timestamp: now,
	})
	s.AddEvent(&domain.Event{
		SessionID: n1.ID, FromState: domain.Awake,
		Action: domain.StartDay, ToState: domain.DayAwake, Timestamp: now.Add(8 * time.Hour),
	})
	s.EndSession(n1.ID, now.Add(8*time.Hour))

	n2, _ := s.CreateSession(domain.SessionKindDay, now.Add(8*time.Hour), false, 0, false)
	s.AddEvent(&domain.Event{
		SessionID: n2.ID, FromState: domain.Awake,
		Action: domain.StartDay, ToState: domain.DayAwake, Timestamp: now.Add(8 * time.Hour),
	})

	eventsMap, err := s.GetEventsForSessions([]int64{n1.ID, n2.ID})
	if err != nil {
		t.Fatalf("GetEventsForSessions: %v", err)
	}
	if len(eventsMap[n1.ID]) != 2 {
		t.Errorf("session 1: got %d events, want 2", len(eventsMap[n1.ID]))
	}
	if len(eventsMap[n2.ID]) != 1 {
		t.Errorf("session 2: got %d events, want 1", len(eventsMap[n2.ID]))
	}

	emptyMap, err := s.GetEventsForSessions(nil)
	if err != nil {
		t.Fatalf("GetEventsForSessions(nil): %v", err)
	}
	if len(emptyMap) != 0 {
		t.Errorf("empty input: got %d entries, want 0", len(emptyMap))
	}
}

func TestDeleteSession(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().Truncate(time.Millisecond)

	sess, _ := s.CreateSession(domain.SessionKindNight, now, false, 0, false)
	s.AddEvent(&domain.Event{
		SessionID: sess.ID, FromState: domain.NightOff,
		Action: domain.StartNight, ToState: domain.Awake, Timestamp: now,
	})

	if err := s.DeleteSession(sess.ID); err != nil {
		t.Fatalf("DeleteSession: %v", err)
	}

	got, _, err := s.GetSession(sess.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if got != nil {
		t.Error("expected nil session after delete")
	}
}

func TestLastSession(t *testing.T) {
	s := newTestStore(t)

	// Empty DB.
	got, err := s.LastSession("")
	if err != nil {
		t.Fatalf("LastSession empty: %v", err)
	}
	if got != nil {
		t.Errorf("expected nil, got %+v", got)
	}

	// Create two nights.
	_, _ = s.CreateSession(domain.SessionKindNight, time.Now().Add(-48*time.Hour), false, 0, false)
	s.EndSession(1, time.Now().Add(-40*time.Hour))
	n2, _ := s.CreateSession(domain.SessionKindNight, time.Now().Add(-24*time.Hour), true, 2, false)
	_ = s.EndSession(n2.ID, time.Now().Add(-23*time.Hour))

	got, err = s.LastSession(domain.SessionKindNight)
	if err != nil {
		t.Fatalf("LastSession: %v", err)
	}
	if got == nil || got.ID != n2.ID {
		t.Fatalf("LastSession = %+v, want id %d", got, n2.ID)
	}
	if !got.FerberEnabled || *got.FerberNightNumber != 2 {
		t.Errorf("LastSession = %+v, want FerberEnabled=true NightNumber=2", got)
	}
}

func TestPrevSessionBeforeAndNextSessionAfter(t *testing.T) {
	s := newTestStore(t)
	t0 := time.Now().Truncate(time.Millisecond)

	n1, _ := s.CreateSession(domain.SessionKindNight, t0, false, 0, false)
	s.EndSession(n1.ID, t0.Add(8*time.Hour))
	d1, _ := s.CreateSession(domain.SessionKindDay, t0.Add(8*time.Hour), false, 0, false)
	s.EndSession(d1.ID, t0.Add(22*time.Hour))
	n2, _ := s.CreateSession(domain.SessionKindNight, t0.Add(22*time.Hour), false, 0, false)
	s.EndSession(n2.ID, t0.Add(30*time.Hour))

	// Prev of n1 is nil (first session).
	prev, err := s.PrevSessionBefore(n1.ID)
	if err != nil {
		t.Fatalf("PrevSessionBefore: %v", err)
	}
	if prev != nil {
		t.Errorf("prev of first session should be nil, got %+v", prev)
	}

	// Prev of d1 is n1.
	prev, err = s.PrevSessionBefore(d1.ID)
	if err != nil || prev == nil || prev.ID != n1.ID {
		t.Errorf("prev of d1 should be n1, got %+v err=%v", prev, err)
	}

	// Next of n1 is d1.
	next, err := s.NextSessionAfter(n1.ID)
	if err != nil || next == nil || next.ID != d1.ID {
		t.Errorf("next of n1 should be d1, got %+v err=%v", next, err)
	}

	// Next of n2 is nil (last session).
	next, err = s.NextSessionAfter(n2.ID)
	if err != nil {
		t.Fatalf("NextSessionAfter: %v", err)
	}
	if next != nil {
		t.Errorf("next of last session should be nil, got %+v", next)
	}
}

// TestLegacyMigration simulates upgrading a pre-day-mode DB: seed a legacy
// schema with nights + events(night_id), then open via New() and verify the
// migration produced the new schema with data intact.
func TestLegacyMigration(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy.db")
	db := openLegacyDB(t, dbPath)
	// Seed two historical nights, one with Ferber, both ending with end_night.
	now := time.Now().Truncate(time.Second)
	_, err := db.Exec(`INSERT INTO nights (id, started_at, ended_at, created_at, ferber_enabled, ferber_night_number) VALUES
		(1, ?, ?, ?, 0, NULL),
		(2, ?, ?, ?, 1, 3)`,
		now.Add(-48*time.Hour).Format(time.RFC3339Nano), now.Add(-40*time.Hour).Format(time.RFC3339Nano), now.Add(-48*time.Hour).Format(time.RFC3339Nano),
		now.Add(-24*time.Hour).Format(time.RFC3339Nano), now.Add(-16*time.Hour).Format(time.RFC3339Nano), now.Add(-24*time.Hour).Format(time.RFC3339Nano),
	)
	if err != nil {
		t.Fatalf("seed nights: %v", err)
	}
	_, err = db.Exec(`INSERT INTO events (night_id, from_state, action, to_state, timestamp, metadata, created_at, seq) VALUES
		(1, 'night_off', 'start_night', 'awake', ?, NULL, ?, 1),
		(1, 'awake', 'end_night', 'night_off', ?, NULL, ?, 2),
		(2, 'night_off', 'start_night', 'awake', ?, NULL, ?, 1),
		(2, 'awake', 'end_night', 'night_off', ?, NULL, ?, 2)`,
		now.Add(-48*time.Hour).Format(time.RFC3339Nano), now.Add(-48*time.Hour).Format(time.RFC3339Nano),
		now.Add(-40*time.Hour).Format(time.RFC3339Nano), now.Add(-40*time.Hour).Format(time.RFC3339Nano),
		now.Add(-24*time.Hour).Format(time.RFC3339Nano), now.Add(-24*time.Hour).Format(time.RFC3339Nano),
		now.Add(-16*time.Hour).Format(time.RFC3339Nano), now.Add(-16*time.Hour).Format(time.RFC3339Nano),
	)
	if err != nil {
		t.Fatalf("seed events: %v", err)
	}
	db.Close()

	// Open via New() — triggers legacy migration.
	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("New on legacy DB: %v", err)
	}
	defer s.Close()

	// Verify sessions table exists and has two night rows with preserved IDs.
	sessions, err := s.ListSessions(now.Add(-72*time.Hour), now.Add(time.Hour), "")
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != 2 {
		t.Fatalf("got %d sessions, want 2", len(sessions))
	}
	if sessions[0].ID != 1 || sessions[1].ID != 2 {
		t.Errorf("session IDs = %d, %d; want 1, 2 (preserved)", sessions[0].ID, sessions[1].ID)
	}
	if sessions[0].Kind != domain.SessionKindNight || sessions[1].Kind != domain.SessionKindNight {
		t.Error("migrated sessions should be kind=night")
	}
	if !sessions[1].FerberEnabled || *sessions[1].FerberNightNumber != 3 {
		t.Error("ferber fields lost in migration")
	}

	// Verify events were preserved and now reference session_id.
	_, events, err := s.GetSession(1)
	if err != nil || len(events) != 2 {
		t.Fatalf("GetSession(1) events = %d, want 2 (err=%v)", len(events), err)
	}
	if events[0].SessionID != 1 {
		t.Errorf("event 0 SessionID = %d, want 1", events[0].SessionID)
	}
	if events[1].Action != "end_night" {
		t.Errorf("historical end_night event preserved as string, got action=%s", events[1].Action)
	}

	// Verify the nights table was dropped.
	exists, err := tableExists(s.db, "nights")
	if err != nil {
		t.Fatalf("tableExists: %v", err)
	}
	if exists {
		t.Error("legacy nights table should be dropped after migration")
	}

	// Verify re-opening is idempotent.
	s.Close()
	s2, err := New(dbPath)
	if err != nil {
		t.Fatalf("re-open after migration: %v", err)
	}
	s2.Close()
}

// TestLegacyMigrationFidelity models a realistic production DB (~20 nights
// with Ferber, mixed metadata, one still-open night) and asserts byte-for-byte
// preservation of every column the app reads back — event IDs, seq, metadata
// JSON, timestamps (with RFC3339Nano precision), created_at (distinct from
// timestamp), and Ferber night-level fields. A regression here would corrupt
// or lose real user data, so the assertions compare the full round-trip.
func TestLegacyMigrationFidelity(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy-fidelity.db")
	db := openLegacyDB(t, dbPath)

	// Event state/action strings are hard-coded (not domain.* constants) because
	// a migration must preserve whatever bytes were in the legacy DB — even if a
	// domain constant is later renamed, the old rows should still round-trip.
	// Seed 20 historical nights + 1 still-open "tonight". Stretch back 3 weeks.
	// Varied: some Ferber, some not; metadata mixing breast + mood.
	type evtSeed struct {
		fromState string
		action    string
		toState   string
		tsOffset  time.Duration
		metadata  string // raw JSON as it would appear in the legacy DB
	}
	type nightSeed struct {
		startOffset   time.Duration
		endOffset     time.Duration // 0 = open
		ferber        bool
		ferberNight   int
		createdOffset time.Duration // distinct from startOffset to exercise created_at
		events        []evtSeed
	}

	// Anchor — use Truncate(time.Microsecond) so RFC3339Nano round-trips exactly.
	anchor := time.Now().Truncate(time.Microsecond)

	nights := make([]nightSeed, 0, 21)
	// 20 closed nights, ~1 per day going back 3 weeks. Mix in Ferber from night 10 on.
	for i := 0; i < 20; i++ {
		daysAgo := time.Duration(20-i) * 24 * time.Hour
		n := nightSeed{
			startOffset:   -daysAgo - 3*time.Hour, // bedtime 3h before midnight-ish
			endOffset:     -daysAgo + 4*time.Hour, // wake ~4h after midnight-ish
			createdOffset: -daysAgo - 3*time.Hour + 50*time.Millisecond,
		}
		if i >= 10 {
			n.ferber = true
			n.ferberNight = i - 9 // ferber night 1, 2, 3, ...
		}
		// A few distinct event shapes so metadata/seq variety is real.
		switch i % 4 {
		case 0:
			n.events = []evtSeed{
				{"night_off", "start_night", "awake", 0, ""},
				{"awake", "start_feed", "feeding", 10 * time.Minute, `{"breast":"L"}`},
				{"feeding", "dislatch_asleep", "sleeping_on_me", 25 * time.Minute, ""},
				{"sleeping_on_me", "start_transfer", "transferring", 40 * time.Minute, ""},
				{"transferring", "transfer_success", "sleeping_crib", 42 * time.Minute, ""},
				{"sleeping_crib", "end_night", "night_off", n.endOffset - n.startOffset, ""},
			}
		case 1:
			n.events = []evtSeed{
				{"night_off", "start_night", "awake", 0, ""},
				{"awake", "start_feed", "feeding", 5 * time.Minute, `{"breast":"R"}`},
				{"feeding", "switch_breast", "feeding", 12 * time.Minute, `{"breast":"L"}`},
				{"feeding", "dislatch_awake", "awake", 20 * time.Minute, ""},
				{"awake", "start_resettle", "resettling", 22 * time.Minute, ""},
				{"resettling", "settled", "sleeping_crib", 40 * time.Minute, ""},
				{"sleeping_crib", "end_night", "night_off", n.endOffset - n.startOffset, ""},
			}
		case 2:
			n.events = []evtSeed{
				{"night_off", "start_night", "awake", 0, ""},
				{"awake", "put_down_awake_ferber", "learning", 5 * time.Minute, `{"mood":"quiet"}`},
				{"learning", "mood_change", "learning", 15 * time.Minute, `{"mood":"fussy"}`},
				{"learning", "check_in", "check_in", 20 * time.Minute, ""},
				{"check_in", "end_check_in", "learning", 21 * time.Minute, ""},
				{"learning", "settled", "sleeping_crib", 40 * time.Minute, ""},
				{"sleeping_crib", "end_night", "night_off", n.endOffset - n.startOffset, ""},
			}
		case 3:
			n.events = []evtSeed{
				{"night_off", "start_night", "awake", 0, ""},
				{"awake", "poop_start", "poop", 3 * time.Minute, ""},
				{"poop", "poop_done", "awake", 7 * time.Minute, ""},
				{"awake", "start_feed", "feeding", 10 * time.Minute, `{"breast":"L"}`},
				{"feeding", "dislatch_asleep", "sleeping_on_me", 30 * time.Minute, ""},
				{"sleeping_on_me", "end_night", "night_off", n.endOffset - n.startOffset, ""},
			}
		}
		nights = append(nights, n)
	}
	// One still-open "tonight": started ~2h ago, no ended_at, a couple of events.
	nights = append(nights, nightSeed{
		startOffset:   -2 * time.Hour,
		endOffset:     0,
		ferber:        true,
		ferberNight:   12,
		createdOffset: -2*time.Hour + 30*time.Millisecond,
		events: []evtSeed{
			{"night_off", "start_night", "awake", 0, ""},
			{"awake", "start_feed", "feeding", 20 * time.Minute, `{"breast":"R"}`},
		},
	})

	// Keep seeded-ID → expected (startedAt, endedAt, ferber*) snapshots so we
	// can compare verbatim after migration.
	type nightSnapshot struct {
		startedAt, createdAt time.Time
		endedAt              *time.Time
		ferber               bool
		ferberNight          *int
	}
	type eventSnapshot struct {
		sessionID int64
		fromState string
		action    string
		toState   string
		timestamp time.Time
		createdAt time.Time
		metadata  string // raw JSON string stored in column, "" if NULL
		seq       int
	}
	nightSnap := map[int64]nightSnapshot{}
	eventSnap := map[int64]eventSnapshot{} // keyed by event ID

	for i, n := range nights {
		nightID := int64(i + 1)
		startedAt := anchor.Add(n.startOffset)
		createdAt := anchor.Add(n.createdOffset)
		var endedAtPtr *time.Time
		var endedArg any
		if n.endOffset != 0 {
			e := anchor.Add(n.endOffset)
			endedAtPtr = &e
			endedArg = e.Format(time.RFC3339Nano)
		}
		var ferberNumArg any
		var ferberNumPtr *int
		if n.ferber {
			v := n.ferberNight
			ferberNumPtr = &v
			ferberNumArg = v
		}
		ferberEnabledInt := 0
		if n.ferber {
			ferberEnabledInt = 1
		}
		if _, err := db.Exec(`INSERT INTO nights (id, started_at, ended_at, created_at, ferber_enabled, ferber_night_number) VALUES (?, ?, ?, ?, ?, ?)`,
			nightID, startedAt.Format(time.RFC3339Nano), endedArg, createdAt.Format(time.RFC3339Nano), ferberEnabledInt, ferberNumArg,
		); err != nil {
			t.Fatalf("seed night %d: %v", nightID, err)
		}
		nightSnap[nightID] = nightSnapshot{
			startedAt:   startedAt,
			endedAt:     endedAtPtr,
			createdAt:   createdAt,
			ferber:      n.ferber,
			ferberNight: ferberNumPtr,
		}

		for seq, evt := range n.events {
			ts := startedAt.Add(evt.tsOffset)
			// Simulate created_at lagging timestamp by a few ms (real-world I/O delay).
			evtCreated := ts.Add(17 * time.Millisecond)
			var metaArg any
			if evt.metadata != "" {
				metaArg = evt.metadata
			}
			res, err := db.Exec(`INSERT INTO events (night_id, from_state, action, to_state, timestamp, metadata, created_at, seq) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
				nightID, evt.fromState, evt.action, evt.toState, ts.Format(time.RFC3339Nano), metaArg, evtCreated.Format(time.RFC3339Nano), seq+1,
			)
			if err != nil {
				t.Fatalf("seed event night=%d seq=%d: %v", nightID, seq+1, err)
			}
			evtID, err := res.LastInsertId()
			if err != nil {
				t.Fatalf("event last id: %v", err)
			}
			eventSnap[evtID] = eventSnapshot{
				sessionID: nightID,
				fromState: evt.fromState,
				action:    evt.action,
				toState:   evt.toState,
				timestamp: ts,
				createdAt: evtCreated,
				metadata:  evt.metadata,
				seq:       seq + 1,
			}
		}
	}

	// Count seeded rows so we can confirm zero drift.
	var seededNights, seededEvents int
	if err := db.QueryRow(`SELECT COUNT(*) FROM nights`).Scan(&seededNights); err != nil {
		t.Fatalf("count nights: %v", err)
	}
	if err := db.QueryRow(`SELECT COUNT(*) FROM events`).Scan(&seededEvents); err != nil {
		t.Fatalf("count events: %v", err)
	}
	db.Close()

	// --- Migrate ---
	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("New on legacy DB: %v", err)
	}
	defer s.Close()

	// Every seeded night should surface as a session with matching id/kind/times.
	sessions, err := s.ListSessions(anchor.Add(-30*24*time.Hour), anchor.Add(24*time.Hour), "")
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	if len(sessions) != seededNights {
		t.Fatalf("got %d sessions, want %d (all seeded nights)", len(sessions), seededNights)
	}

	var openCount int
	for _, sess := range sessions {
		want, ok := nightSnap[sess.ID]
		if !ok {
			t.Errorf("unexpected session id=%d (no seed)", sess.ID)
			continue
		}
		if sess.Kind != domain.SessionKindNight {
			t.Errorf("session %d kind=%q, want night", sess.ID, sess.Kind)
		}
		if !sess.StartedAt.Equal(want.startedAt) {
			t.Errorf("session %d startedAt=%s, want %s", sess.ID, sess.StartedAt, want.startedAt)
		}
		if !sess.CreatedAt.Equal(want.createdAt) {
			t.Errorf("session %d createdAt=%s, want %s", sess.ID, sess.CreatedAt, want.createdAt)
		}
		if (sess.EndedAt == nil) != (want.endedAt == nil) {
			t.Errorf("session %d endedAt presence mismatch: got=%v want=%v", sess.ID, sess.EndedAt, want.endedAt)
		} else if sess.EndedAt != nil && !sess.EndedAt.Equal(*want.endedAt) {
			t.Errorf("session %d endedAt=%s, want %s", sess.ID, sess.EndedAt, want.endedAt)
		}
		if sess.EndedAt == nil {
			openCount++
		}
		if sess.FerberEnabled != want.ferber {
			t.Errorf("session %d FerberEnabled=%v, want %v", sess.ID, sess.FerberEnabled, want.ferber)
		}
		if (sess.FerberNightNumber == nil) != (want.ferberNight == nil) {
			t.Errorf("session %d ferberNightNumber presence mismatch", sess.ID)
		} else if sess.FerberNightNumber != nil && *sess.FerberNightNumber != *want.ferberNight {
			t.Errorf("session %d ferberNightNumber=%d, want %d", sess.ID, *sess.FerberNightNumber, *want.ferberNight)
		}
	}
	if openCount != 1 {
		t.Errorf("open session count = %d, want 1", openCount)
	}

	// Every seeded event should surface with preserved id/seq/timestamps/metadata.
	allEvents, err := s.GetAllEvents()
	if err != nil {
		t.Fatalf("GetAllEvents: %v", err)
	}
	if len(allEvents) != seededEvents {
		t.Fatalf("got %d events, want %d (all seeded events)", len(allEvents), seededEvents)
	}
	for _, got := range allEvents {
		want, ok := eventSnap[got.ID]
		if !ok {
			t.Errorf("unexpected event id=%d (no seed)", got.ID)
			continue
		}
		if got.SessionID != want.sessionID {
			t.Errorf("event %d SessionID=%d, want %d (night_id → session_id)", got.ID, got.SessionID, want.sessionID)
		}
		if string(got.FromState) != want.fromState {
			t.Errorf("event %d FromState=%q, want %q", got.ID, got.FromState, want.fromState)
		}
		if string(got.Action) != want.action {
			t.Errorf("event %d Action=%q, want %q", got.ID, got.Action, want.action)
		}
		if string(got.ToState) != want.toState {
			t.Errorf("event %d ToState=%q, want %q", got.ID, got.ToState, want.toState)
		}
		if !got.Timestamp.Equal(want.timestamp) {
			t.Errorf("event %d Timestamp=%s, want %s", got.ID, got.Timestamp, want.timestamp)
		}
		if !got.CreatedAt.Equal(want.createdAt) {
			t.Errorf("event %d CreatedAt=%s, want %s", got.ID, got.CreatedAt, want.createdAt)
		}
		if got.Seq != want.seq {
			t.Errorf("event %d Seq=%d, want %d", got.ID, got.Seq, want.seq)
		}
		// Migration copies metadata as raw text; scanner re-parses. Compare
		// as maps so key order in the JSON doesn't matter.
		if want.metadata == "" {
			if got.Metadata != nil {
				t.Errorf("event %d Metadata=%v, want nil", got.ID, got.Metadata)
			}
		} else {
			var wantMap map[string]string
			if err := json.Unmarshal([]byte(want.metadata), &wantMap); err != nil {
				t.Fatalf("parse seeded metadata: %v", err)
			}
			if !maps.Equal(got.Metadata, wantMap) {
				t.Errorf("event %d Metadata=%v, want %v", got.ID, got.Metadata, wantMap)
			}
		}
	}

	// The still-open session (id 21) should be the one CurrentSession returns.
	cur, curEvents, err := s.CurrentSession()
	if err != nil {
		t.Fatalf("CurrentSession: %v", err)
	}
	if cur == nil {
		t.Fatal("CurrentSession = nil after migration, want the open session")
	}
	if cur.ID != int64(len(nights)) {
		t.Errorf("CurrentSession id=%d, want %d", cur.ID, len(nights))
	}
	if len(curEvents) != len(nights[len(nights)-1].events) {
		t.Errorf("CurrentSession events=%d, want %d", len(curEvents), len(nights[len(nights)-1].events))
	}

	// nights table must be gone so a subsequent run takes the idempotent path.
	exists, err := tableExists(s.db, "nights")
	if err != nil {
		t.Fatalf("tableExists: %v", err)
	}
	if exists {
		t.Error("legacy nights table should be dropped after migration")
	}

	// Idempotent re-open: simulate a restart, confirm data is still intact.
	s.Close()
	s2, err := New(dbPath)
	if err != nil {
		t.Fatalf("re-open after migration: %v", err)
	}
	sess2, err := s2.ListSessions(anchor.Add(-30*24*time.Hour), anchor.Add(24*time.Hour), "")
	if err != nil {
		t.Fatalf("ListSessions after re-open: %v", err)
	}
	if len(sess2) != seededNights {
		t.Errorf("sessions after re-open = %d, want %d", len(sess2), seededNights)
	}
	s2.Close()
}

// TestLegacyMigrationAtomic verifies the migration either fully commits or
// leaves the legacy schema untouched. Injects a mid-transaction failure by
// pre-creating the `events_new` temp table so migrateLegacy's phase-C
// `CREATE TABLE events_new` fails, forcing the transaction to roll back
// every prior phase (sessions DDL, sessions backfill). If rollback is
// broken, the legacy tables would be lost or corrupted.
func TestLegacyMigrationAtomic(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "legacy-atomic.db")
	db := openLegacyDB(t, dbPath)
	now := time.Now().Truncate(time.Microsecond)
	if _, err := db.Exec(`INSERT INTO nights (id, started_at, ended_at, created_at, ferber_enabled, ferber_night_number) VALUES (1, ?, ?, ?, 0, NULL)`,
		now.Add(-10*time.Hour).Format(time.RFC3339Nano), now.Add(-2*time.Hour).Format(time.RFC3339Nano), now.Add(-10*time.Hour).Format(time.RFC3339Nano),
	); err != nil {
		t.Fatalf("seed night: %v", err)
	}
	if _, err := db.Exec(`INSERT INTO events (night_id, from_state, action, to_state, timestamp, metadata, created_at, seq) VALUES (1, 'night_off', 'start_night', 'awake', ?, NULL, ?, 1)`,
		now.Add(-10*time.Hour).Format(time.RFC3339Nano), now.Add(-10*time.Hour).Format(time.RFC3339Nano),
	); err != nil {
		t.Fatalf("seed event: %v", err)
	}

	// Pre-create `events_new` to force migrateLegacy's phase C to fail with
	// "table events_new already exists". This happens after phases A and B
	// have already executed inside the tx, exercising the rollback path.
	if _, err := db.Exec(`CREATE TABLE events_new (id INTEGER PRIMARY KEY)`); err != nil {
		t.Fatalf("pre-seed events_new: %v", err)
	}
	db.Close()

	// migrate() should fail — and leave the database byte-identical to before.
	s, err := New(dbPath)
	if err == nil {
		s.Close()
		t.Fatal("New() unexpectedly succeeded; phase-C conflict should have failed the migration")
	}

	// Re-open the file raw and verify nothing was corrupted.
	reopen, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatalf("reopen raw after failed migrate: %v", err)
	}
	defer reopen.Close()

	// Legacy tables must still be present with original row counts.
	var nNights, nEvents int
	if err := reopen.QueryRow(`SELECT COUNT(*) FROM nights`).Scan(&nNights); err != nil {
		t.Fatalf("nights count after failed migrate: %v", err)
	}
	if err := reopen.QueryRow(`SELECT COUNT(*) FROM events`).Scan(&nEvents); err != nil {
		t.Fatalf("events count after failed migrate: %v", err)
	}
	if nNights != 1 {
		t.Errorf("nights count after failed migrate = %d, want 1 (data preserved)", nNights)
	}
	if nEvents != 1 {
		t.Errorf("events count after failed migrate = %d, want 1 (data preserved)", nEvents)
	}

	// sessions table and unique index created in phase A must have rolled back.
	sessionsExists, err := tableExists(reopen, "sessions")
	if err != nil {
		t.Fatalf("tableExists sessions: %v", err)
	}
	if sessionsExists {
		t.Error("sessions table exists after failed migrate — transaction did not roll back")
	}

	// The legacy event must still reference night_id (schema not mutated).
	rows, err := reopen.Query(`SELECT night_id FROM events`)
	if err != nil {
		t.Fatalf("select night_id after failed migrate: %v", err)
	}
	rows.Close()

	// Remove the obstacle and verify a retry migrates cleanly — the failed
	// attempt left the DB in a state where the next boot can still recover.
	if _, err := reopen.Exec(`DROP TABLE events_new`); err != nil {
		t.Fatalf("drop events_new: %v", err)
	}
	reopen.Close()

	s2, err := New(dbPath)
	if err != nil {
		t.Fatalf("retry migrate after obstacle removed: %v", err)
	}
	defer s2.Close()
	sessions, err := s2.ListSessions(now.Add(-72*time.Hour), now.Add(time.Hour), "")
	if err != nil {
		t.Fatalf("ListSessions after retry: %v", err)
	}
	if len(sessions) != 1 || sessions[0].ID != 1 {
		t.Errorf("after retry: sessions=%+v, want one session with id=1", sessions)
	}
}

// TestPrevNextSessionChronological verifies PrevSessionBefore / NextSessionAfter
// order by started_at, not id. A session inserted with a HIGH id but an EARLY
// started_at (as a retroactive split would create) must be found as the
// chronological neighbor — not skipped because its id is out of order.
func TestPrevNextSessionChronological(t *testing.T) {
	s := newTestStore(t)
	base := time.Date(2026, 5, 26, 21, 0, 0, 0, time.Local)

	// mkClosed creates a session and immediately closes it, so multiple
	// fixtures can coexist without tripping the one_open_session index.
	mkClosed := func(kind domain.SessionKind, startedAt time.Time) *domain.Session {
		sess, err := s.CreateSession(kind, startedAt, false, 0, false)
		if err != nil {
			t.Fatalf("create session: %v", err)
		}
		if err := s.EndSession(sess.ID, startedAt.Add(time.Hour)); err != nil {
			t.Fatalf("end session: %v", err)
		}
		return sess
	}

	// Insert in chronological order first: A (21:00), C (next day 07:00).
	a := mkClosed(domain.SessionKindNight, base)
	c := mkClosed(domain.SessionKindDay, base.Add(10*time.Hour))

	// Now insert B with a started_at BETWEEN A and C but a HIGHER id than C
	// (simulates a retroactive split: new row, mid-chain timestamp).
	b := mkClosed(domain.SessionKindDay, base.Add(2*time.Hour))
	if b.ID < c.ID {
		t.Fatalf("test precondition: expected B.ID (%d) > C.ID (%d)", b.ID, c.ID)
	}

	// Chronological prev of C is B (23:00), NOT A (21:00) and NOT by-id (A).
	prev, err := s.PrevSessionBefore(c.ID)
	if err != nil {
		t.Fatalf("PrevSessionBefore: %v", err)
	}
	if prev == nil || prev.ID != b.ID {
		t.Fatalf("PrevSessionBefore(C) = %v, want B (id %d)", prev, b.ID)
	}

	// Chronological next of A is B, NOT C.
	next, err := s.NextSessionAfter(a.ID)
	if err != nil {
		t.Fatalf("NextSessionAfter: %v", err)
	}
	if next == nil || next.ID != b.ID {
		t.Fatalf("NextSessionAfter(A) = %v, want B (id %d)", next, b.ID)
	}
}

// seedSessionWithEvents inserts a session and its events verbatim (caller
// supplies seq/timestamps). Returns the session ID. Used to construct
// over-long sessions for split tests without routing through chain advances.
func seedSessionWithEvents(t *testing.T, s *Store, sess *domain.Session, events []domain.Event) int64 {
	t.Helper()
	created, err := s.CreateSession(sess.Kind, sess.StartedAt, sess.FerberEnabled, derefInt(sess.FerberNightNumber), sess.ChairEnabled)
	if err != nil {
		t.Fatalf("seed CreateSession: %v", err)
	}
	if sess.EndedAt != nil {
		if err := s.EndSession(created.ID, *sess.EndedAt); err != nil {
			t.Fatalf("seed EndSession: %v", err)
		}
	}
	for i := range events {
		events[i].SessionID = created.ID
		if err := s.AddEvent(&events[i]); err != nil {
			t.Fatalf("seed AddEvent: %v", err)
		}
	}
	return created.ID
}

func derefInt(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

func TestStoreSplitSession_NightRoundTrip(t *testing.T) {
	s := newTestStore(t)
	b := time.Date(2026, 5, 25, 21, 0, 0, 0, time.Local)
	end := b.Add(20 * time.Hour)
	sess := &domain.Session{Kind: domain.SessionKindNight, StartedAt: b, EndedAt: &end}
	events := []domain.Event{
		{FromState: domain.NightOff, Action: domain.StartNight, ToState: domain.Awake, Timestamp: b},
		{FromState: domain.SleepingOnMe, Action: domain.BabyWoke, ToState: domain.Awake, Timestamp: b.Add(10 * time.Hour)},
		{FromState: domain.Awake, Action: domain.StartFeed, ToState: domain.Feeding, Timestamp: b.Add(12 * time.Hour)},
		{FromState: domain.Feeding, Action: domain.DislatchAwake, ToState: domain.Awake, Timestamp: b.Add(13 * time.Hour)},
	}
	origID := seedSessionWithEvents(t, s, sess, events)

	loaded, loadedEvents, err := s.GetSession(origID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	splitAt := b.Add(10*time.Hour + 30*time.Minute) // 07:30
	plan, err := domain.SplitSession(*loaded, loadedEvents, splitAt)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	degID, trailID, err := s.SplitSession(origID, plan)
	if err != nil {
		t.Fatalf("SplitSession: %v", err)
	}
	if degID == 0 || trailID == 0 || degID == trailID {
		t.Fatalf("bad new ids: deg=%d trail=%d", degID, trailID)
	}

	// Original shortened.
	orig, origEvents, _ := s.GetSession(origID)
	if orig.EndedAt == nil || !orig.EndedAt.Equal(splitAt) {
		t.Errorf("orig.EndedAt = %v, want %v", orig.EndedAt, splitAt)
	}
	if len(origEvents) != 2 { // start_night + 07:00 wake
		t.Errorf("orig events = %d, want 2", len(origEvents))
	}

	// Degenerate: day, 1s, single start_day event.
	deg, degEvents, _ := s.GetSession(degID)
	if deg.Kind != domain.SessionKindDay || len(degEvents) != 1 || degEvents[0].Action != domain.StartDay {
		t.Errorf("degenerate wrong: kind=%s events=%d", deg.Kind, len(degEvents))
	}

	// Trailing: night, inherits original end, opener + 2 reparented events.
	trail, trailEvents, _ := s.GetSession(trailID)
	if trail.Kind != domain.SessionKindNight {
		t.Errorf("trailing kind = %s, want night", trail.Kind)
	}
	if trail.EndedAt == nil || !trail.EndedAt.Equal(end) {
		t.Errorf("trailing.EndedAt = %v, want %v", trail.EndedAt, end)
	}
	if len(trailEvents) != 3 { // start_night + 09:00 feed + 10:00 dislatch
		t.Fatalf("trailing events = %d, want 3", len(trailEvents))
	}
	// Seq is dense 1..3 and the FK points at the trailing session.
	for i, e := range trailEvents {
		if e.Seq != i+1 {
			t.Errorf("trailing event %d seq = %d, want %d", i, e.Seq, i+1)
		}
		if e.SessionID != trailID {
			t.Errorf("trailing event %d session_id = %d, want %d", i, e.SessionID, trailID)
		}
	}

	// AddEvent to the trailing continues from MAX(seq)+1.
	next := &domain.Event{SessionID: trailID, FromState: domain.Awake, Action: domain.StartFeed, ToState: domain.Feeding, Timestamp: end.Add(-time.Hour)}
	if err := s.AddEvent(next); err != nil {
		t.Fatalf("AddEvent post-split: %v", err)
	}
	if next.Seq != 4 {
		t.Errorf("post-split AddEvent seq = %d, want 4", next.Seq)
	}
}

func TestStoreSplitSession_OpenPartialIndex(t *testing.T) {
	s := newTestStore(t)
	b := time.Date(2026, 5, 25, 21, 0, 0, 0, time.Local)
	sess := &domain.Session{Kind: domain.SessionKindNight, StartedAt: b} // open (EndedAt nil)
	events := []domain.Event{
		{FromState: domain.NightOff, Action: domain.StartNight, ToState: domain.Awake, Timestamp: b},
		{FromState: domain.SleepingOnMe, Action: domain.BabyWoke, ToState: domain.Awake, Timestamp: b.Add(10 * time.Hour)},
		{FromState: domain.Awake, Action: domain.StartFeed, ToState: domain.Feeding, Timestamp: b.Add(12 * time.Hour)},
	}
	origID := seedSessionWithEvents(t, s, sess, events)
	loaded, loadedEvents, _ := s.GetSession(origID)
	plan, err := domain.SplitSession(*loaded, loadedEvents, b.Add(10*time.Hour+30*time.Minute))
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	if _, _, err := s.SplitSession(origID, plan); err != nil {
		t.Fatalf("SplitSession: %v", err)
	}

	// Exactly one open session after the split (the trailing).
	var openCount int
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM sessions WHERE ended_at IS NULL`).Scan(&openCount); err != nil {
		t.Fatalf("count open: %v", err)
	}
	if openCount != 1 {
		t.Errorf("open sessions = %d, want 1", openCount)
	}
	cur, _, err := s.CurrentSession()
	if err != nil || cur == nil || cur.Kind != domain.SessionKindNight {
		t.Errorf("CurrentSession after split = %v (err %v), want open night", cur, err)
	}
}

// Review finding: undoing a split's OPEN trailing pops real events first;
// reaching the synthetic opener reopens the degenerate, never silently
// destroying still-present reparented events.
func TestStoreSplitSession_UndoAfterSplitOpen(t *testing.T) {
	s := newTestStore(t)
	b := time.Date(2026, 5, 25, 21, 0, 0, 0, time.Local)
	sess := &domain.Session{Kind: domain.SessionKindNight, StartedAt: b}
	events := []domain.Event{
		{FromState: domain.NightOff, Action: domain.StartNight, ToState: domain.Awake, Timestamp: b},
		{FromState: domain.SleepingOnMe, Action: domain.BabyWoke, ToState: domain.Awake, Timestamp: b.Add(10 * time.Hour)},
		{FromState: domain.Awake, Action: domain.StartFeed, ToState: domain.Feeding, Timestamp: b.Add(12 * time.Hour)},
	}
	origID := seedSessionWithEvents(t, s, sess, events)
	loaded, loadedEvents, _ := s.GetSession(origID)
	plan, _ := domain.SplitSession(*loaded, loadedEvents, b.Add(10*time.Hour+30*time.Minute))
	_, trailID, err := s.SplitSession(origID, plan)
	if err != nil {
		t.Fatalf("SplitSession: %v", err)
	}

	// Trailing open session: [start_night(seq1), 09:00 feed(seq2)]. Pop the
	// one real reparented event — start_night remains, session still open.
	if _, err := s.PopEvent(trailID); err != nil {
		t.Fatalf("PopEvent: %v", err)
	}
	_, trailEvents, _ := s.GetSession(trailID)
	if len(trailEvents) != 1 || trailEvents[0].Action != domain.StartNight {
		t.Fatalf("after one undo, trailing = %d events, want lone start_night", len(trailEvents))
	}
}

// TestStoreSplitSession_MidChainRetro is the mid-chain retro-split integration
// test. Scenario: a broken multi-day night N (closed), followed by properly-chained
// sessions N+1 (day), N+2 (night, open). User retroactively splits N. The
// split inserts rows with HIGH ids but chronologically-middle started_at.
func TestStoreSplitSession_MidChainRetro(t *testing.T) {
	s := newTestStore(t)
	b := time.Date(2026, 5, 25, 21, 0, 0, 0, time.Local)

	// N: broken night Mon 21:00 → Wed 07:00 (ran through a whole day).
	nEnd := b.Add(34 * time.Hour) // Wed 07:00
	nSess := &domain.Session{Kind: domain.SessionKindNight, StartedAt: b, EndedAt: &nEnd}
	nEvents := []domain.Event{
		{FromState: domain.NightOff, Action: domain.StartNight, ToState: domain.Awake, Timestamp: b},
		{FromState: domain.SleepingOnMe, Action: domain.BabyWoke, ToState: domain.Awake, Timestamp: b.Add(10 * time.Hour)}, // Tue 07:00
		{FromState: domain.Awake, Action: domain.StartFeed, ToState: domain.Feeding, Timestamp: b.Add(24 * time.Hour)},     // Tue 21:00
		{FromState: domain.Feeding, Action: domain.DislatchAwake, ToState: domain.Awake, Timestamp: b.Add(33 * time.Hour)}, // Wed 06:00
	}
	nID := seedSessionWithEvents(t, s, nSess, nEvents)

	// N+1: day Wed 07:00 → Wed 19:00.
	d1End := b.Add(46 * time.Hour)
	d1 := &domain.Session{Kind: domain.SessionKindDay, StartedAt: nEnd, EndedAt: &d1End}
	d1ID := seedSessionWithEvents(t, s, d1, []domain.Event{
		{FromState: domain.Awake, Action: domain.StartDay, ToState: domain.DayAwake, Timestamp: nEnd},
	})

	// N+2: night Wed 19:00 → open.
	n2 := &domain.Session{Kind: domain.SessionKindNight, StartedAt: d1End}
	n2ID := seedSessionWithEvents(t, s, n2, []domain.Event{
		{FromState: domain.DayAwake, Action: domain.StartNight, ToState: domain.Awake, Timestamp: d1End},
	})

	// Retro-split N at Tue 07:30 (inside its Awake span).
	loaded, loadedEvents, _ := s.GetSession(nID)
	plan, err := domain.SplitSession(*loaded, loadedEvents, b.Add(10*time.Hour+30*time.Minute))
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	degID, trailID, err := s.SplitSession(nID, plan)
	if err != nil {
		t.Fatalf("SplitSession: %v", err)
	}
	// Sanity: new ids are higher than all pre-existing sessions.
	if degID <= n2ID || trailID <= n2ID {
		t.Fatalf("expected split ids > n2ID(%d), got deg=%d trail=%d", n2ID, degID, trailID)
	}

	// Chain invariant by started_at order across the whole table.
	all, err := s.ListSessions(b.Add(-24*time.Hour), b.Add(72*time.Hour), "")
	if err != nil {
		t.Fatalf("ListSessions: %v", err)
	}
	for i := 0; i+1 < len(all); i++ {
		a, c := all[i], all[i+1]
		if a.EndedAt == nil {
			t.Fatalf("non-final session %d has nil ended_at", a.ID)
		}
		if !a.EndedAt.Equal(c.StartedAt) {
			t.Errorf("chain break: session %d ended_at %v != session %d started_at %v",
				a.ID, a.EndedAt, c.ID, c.StartedAt)
		}
	}

	// Chronological-ordering exercise: prev of N+1 is the split's trailing
	// night, not the original N.
	prev, _ := s.PrevSessionBefore(d1ID)
	if prev == nil || prev.ID != trailID {
		t.Errorf("PrevSessionBefore(N+1) = %v, want trailing(%d)", prev, trailID)
	}
	// PostUndo's chain-advance detection (handlers.go) reopens prev iff
	// prev.EndedAt == session.StartedAt. After the split, N+1 chains from the
	// trailing (which inherited N's original ended_at == nEnd == N+1.StartedAt),
	// so undoing N+1's opener must reopen the TRAILING, not the original N.
	if prev != nil && (prev.EndedAt == nil || !prev.EndedAt.Equal(nEnd)) {
		t.Errorf("trailing.EndedAt = %v, want %v (PostUndo predicate)", prev.EndedAt, nEnd)
	}
	// Chronological next of N is the degenerate day.
	next, _ := s.NextSessionAfter(nID)
	if next == nil || next.ID != degID {
		t.Errorf("NextSessionAfter(N) = %v, want degenerate(%d)", next, degID)
	}

	// N+2 still open; AddEvent continues its seq cleanly.
	evt := &domain.Event{SessionID: n2ID, FromState: domain.Awake, Action: domain.StartFeed, ToState: domain.Feeding, Timestamp: d1End.Add(time.Hour)}
	if err := s.AddEvent(evt); err != nil {
		t.Fatalf("AddEvent to N+2: %v", err)
	}
	if evt.Seq != 2 {
		t.Errorf("N+2 AddEvent seq = %d, want 2", evt.Seq)
	}
}

func TestStoreSplitSession_MidChainRetroDay(t *testing.T) {
	s := newTestStore(t)
	b := time.Date(2026, 5, 25, 7, 0, 0, 0, time.Local) // Mon 07:00

	// D: broken day Mon 07:00 → Tue 19:00 (ran through a whole night).
	dEnd := b.Add(36 * time.Hour) // Tue 19:00
	dSess := &domain.Session{Kind: domain.SessionKindDay, StartedAt: b, EndedAt: &dEnd}
	dID := seedSessionWithEvents(t, s, dSess, []domain.Event{
		{FromState: domain.NightOff, Action: domain.StartDay, ToState: domain.DayAwake, Timestamp: b},
		{FromState: domain.DaySleeping, Action: domain.BabyWoke, ToState: domain.DayAwake, Timestamp: b.Add(4 * time.Hour)},  // Mon 11:00
		{FromState: domain.DayAwake, Action: domain.StartFeed, ToState: domain.DayFeeding, Timestamp: b.Add(30 * time.Hour)}, // Tue 13:00
		{FromState: domain.DayFeeding, Action: domain.DislatchAwake, ToState: domain.DayAwake, Timestamp: b.Add(31 * time.Hour)},
	})

	// N+1: night Tue 19:00 → Wed 07:00.
	n1End := b.Add(48 * time.Hour)
	n1 := &domain.Session{Kind: domain.SessionKindNight, StartedAt: dEnd, EndedAt: &n1End}
	n1ID := seedSessionWithEvents(t, s, n1, []domain.Event{
		{FromState: domain.DayAwake, Action: domain.StartNight, ToState: domain.Awake, Timestamp: dEnd},
	})

	// N+2: day Wed 07:00 → open.
	n2 := &domain.Session{Kind: domain.SessionKindDay, StartedAt: n1End}
	seedSessionWithEvents(t, s, n2, []domain.Event{
		{FromState: domain.Awake, Action: domain.StartDay, ToState: domain.DayAwake, Timestamp: n1End},
	})

	// Split D at Mon 20:00 (inside the 11:00→Tue 13:00 DayAwake span).
	loaded, loadedEvents, _ := s.GetSession(dID)
	plan, err := domain.SplitSession(*loaded, loadedEvents, b.Add(13*time.Hour))
	if err != nil {
		t.Fatalf("plan: %v", err)
	}
	degID, trailID, err := s.SplitSession(dID, plan)
	if err != nil {
		t.Fatalf("SplitSession: %v", err)
	}

	all, _ := s.ListSessions(b.Add(-24*time.Hour), b.Add(96*time.Hour), "")
	for i := 0; i+1 < len(all); i++ {
		a, c := all[i], all[i+1]
		if a.EndedAt == nil || !a.EndedAt.Equal(c.StartedAt) {
			t.Errorf("chain break at session %d → %d", a.ID, c.ID)
		}
	}
	next, _ := s.NextSessionAfter(dID)
	if next == nil || next.ID != degID {
		t.Errorf("NextSessionAfter(D) = %v, want degenerate night(%d)", next, degID)
	}
	prev, _ := s.PrevSessionBefore(n1ID)
	if prev == nil || prev.ID != trailID {
		t.Errorf("PrevSessionBefore(N+1) = %v, want trailing day(%d)", prev, trailID)
	}
}

// TestInsertEventAtSeq_RepairThenSplit covers the data-repair flow we hand
// users whose log jumps straight from sleep into a poop (no AWAKE moment to
// split on): inject a wake marker mid-stream, then confirm the splitter now
// accepts that instant.
func TestInsertEventAtSeq_RepairThenSplit(t *testing.T) {
	s := newTestStore(t)
	b := time.Date(2026, 5, 25, 21, 0, 0, 0, time.Local)
	end := b.Add(50 * time.Hour)
	sess := &domain.Session{Kind: domain.SessionKindNight, StartedAt: b, EndedAt: &end}
	events := []domain.Event{
		{FromState: domain.NightOff, Action: domain.StartNight, ToState: domain.Awake, Timestamp: b},
		{FromState: domain.Awake, Action: domain.StartResettle, ToState: domain.Resettling, Timestamp: b.Add(30 * time.Minute)},
		{FromState: domain.Resettling, Action: domain.Settled, ToState: domain.SleepingCrib, Timestamp: b.Add(time.Hour)},
		// Morning wake logged only as a diaper change — no awake state.
		{FromState: domain.SleepingCrib, Action: domain.PoopStart, ToState: domain.Poop, Timestamp: b.Add(10 * time.Hour)},
		{FromState: domain.Poop, Action: domain.PoopDone, ToState: domain.SleepingCrib, Timestamp: b.Add(10*time.Hour + 10*time.Minute)},
	}
	origID := seedSessionWithEvents(t, s, sess, events)

	loaded, loadedEvents, err := s.GetSession(origID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	wakeAt := b.Add(10*time.Hour - time.Minute) // just before the poop

	// Splitting here must fail first — the baby was sleeping, not awake.
	if _, err := domain.SplitSession(*loaded, loadedEvents, wakeAt); err == nil {
		t.Fatal("expected split to fail before the wake marker is inserted")
	}

	marker, err := domain.PlanWakeMarker(*loaded, loadedEvents, wakeAt)
	if err != nil {
		t.Fatalf("PlanWakeMarker: %v", err)
	}
	if err := s.InsertEventAtSeq(&marker); err != nil {
		t.Fatalf("InsertEventAtSeq: %v", err)
	}

	// Reload: the marker is spliced in with dense, ordered seqs.
	_, after, err := s.GetSession(origID)
	if err != nil {
		t.Fatalf("GetSession after insert: %v", err)
	}
	if len(after) != len(events)+1 {
		t.Fatalf("event count = %d, want %d", len(after), len(events)+1)
	}
	for i, e := range after {
		if e.Seq != i+1 {
			t.Errorf("event %d seq = %d, want dense %d", i, e.Seq, i+1)
		}
		if i > 0 && e.Timestamp.Before(after[i-1].Timestamp) {
			t.Errorf("event %d out of time order", i)
		}
	}
	if after[3].ToState != domain.Awake || after[3].Action != domain.BabyWoke {
		t.Errorf("spliced event = %+v, want baby_woke->awake at index 3", after[3])
	}

	// Now the split at the same instant succeeds.
	loaded2, events2, _ := s.GetSession(origID)
	plan, err := domain.SplitSession(*loaded2, events2, wakeAt)
	if err != nil {
		t.Fatalf("split after repair: %v", err)
	}
	if _, _, err := s.SplitSession(origID, plan); err != nil {
		t.Fatalf("store SplitSession after repair: %v", err)
	}
}
