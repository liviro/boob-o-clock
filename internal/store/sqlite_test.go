package store

import (
	"database/sql"
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

	sess, err := s.CreateSession(domain.SessionKindNight, now, false, 0)
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

	sess, err := s.CreateSession(domain.SessionKindDay, now, false, 0)
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
	sess, err := s.CreateSession(domain.SessionKindNight, started, true, 3)
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

func TestEndSession(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().Truncate(time.Millisecond)

	sess, _ := s.CreateSession(domain.SessionKindNight, now, false, 0)
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
	if _, err := s.CreateSession(domain.SessionKindNight, now, false, 0); err != nil {
		t.Fatalf("first CreateSession: %v", err)
	}

	// Second open session (without closing the first) must fail via the
	// unique partial index on ended_at.
	if _, err := s.CreateSession(domain.SessionKindDay, now.Add(time.Hour), false, 0); err == nil {
		t.Error("expected unique constraint error on second open session, got none")
	}
}

func TestAddEventAndGetSession(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().Truncate(time.Millisecond)

	sess, _ := s.CreateSession(domain.SessionKindNight, now, false, 0)

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

	sess, _ := s.CreateSession(domain.SessionKindNight, now, false, 0)

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

	sess, _ := s.CreateSession(domain.SessionKindNight, now, false, 0)

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
	opened, _ := s.CreateSession(domain.SessionKindNight, now, false, 0)
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

	n1, _ := s.CreateSession(domain.SessionKindNight, now, false, 0)
	s.EndSession(n1.ID, now.Add(8*time.Hour))

	d1, _ := s.CreateSession(domain.SessionKindDay, now.Add(8*time.Hour), false, 0)
	s.EndSession(d1.ID, now.Add(22*time.Hour))

	n2, _ := s.CreateSession(domain.SessionKindNight, now.Add(22*time.Hour), false, 0)
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

	n1, _ := s.CreateSession(domain.SessionKindNight, now, false, 0)
	s.AddEvent(&domain.Event{
		SessionID: n1.ID, FromState: domain.NightOff,
		Action: domain.StartNight, ToState: domain.Awake, Timestamp: now,
	})
	s.AddEvent(&domain.Event{
		SessionID: n1.ID, FromState: domain.Awake,
		Action: domain.StartDay, ToState: domain.DayAwake, Timestamp: now.Add(8 * time.Hour),
	})
	s.EndSession(n1.ID, now.Add(8*time.Hour))

	n2, _ := s.CreateSession(domain.SessionKindDay, now.Add(8*time.Hour), false, 0)
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

	sess, _ := s.CreateSession(domain.SessionKindNight, now, false, 0)
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
	_, _ = s.CreateSession(domain.SessionKindNight, time.Now().Add(-48*time.Hour), false, 0)
	s.EndSession(1, time.Now().Add(-40*time.Hour))
	n2, _ := s.CreateSession(domain.SessionKindNight, time.Now().Add(-24*time.Hour), true, 2)
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

	n1, _ := s.CreateSession(domain.SessionKindNight, t0, false, 0)
	s.EndSession(n1.ID, t0.Add(8*time.Hour))
	d1, _ := s.CreateSession(domain.SessionKindDay, t0.Add(8*time.Hour), false, 0)
	s.EndSession(d1.ID, t0.Add(22*time.Hour))
	n2, _ := s.CreateSession(domain.SessionKindNight, t0.Add(22*time.Hour), false, 0)
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
	// Open a raw sqlite connection and seed the pre-day-mode schema.
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
	// Seed two historical nights, one with Ferber, both ending with end_night.
	now := time.Now().Truncate(time.Second)
	_, err = db.Exec(`INSERT INTO nights (id, started_at, ended_at, created_at, ferber_enabled, ferber_night_number) VALUES
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
