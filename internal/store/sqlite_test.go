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

	// Opening again should be idempotent (migrations run twice without error)
	s2, err := New(s.dbPath)
	if err != nil {
		t.Fatalf("second open failed: %v", err)
	}
	s2.Close()
}

func TestCreateNight(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().Truncate(time.Millisecond)

	night, err := s.CreateNight(now)
	if err != nil {
		t.Fatalf("CreateNight: %v", err)
	}
	if night.ID == 0 {
		t.Error("expected non-zero night ID")
	}
	if !night.StartedAt.Equal(now) {
		t.Errorf("StartedAt = %v, want %v", night.StartedAt, now)
	}
	if night.EndedAt != nil {
		t.Error("EndedAt should be nil for new night")
	}
}

func TestEndNight(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().Truncate(time.Millisecond)

	night, _ := s.CreateNight(now)
	endTime := now.Add(8 * time.Hour)

	err := s.EndNight(night.ID, endTime)
	if err != nil {
		t.Fatalf("EndNight: %v", err)
	}

	got, _, err := s.GetNight(night.ID)
	if err != nil {
		t.Fatalf("GetNight: %v", err)
	}
	if got.EndedAt == nil || !got.EndedAt.Equal(endTime) {
		t.Errorf("EndedAt = %v, want %v", got.EndedAt, endTime)
	}
}

func TestAddEventAndGetNight(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().Truncate(time.Millisecond)

	night, _ := s.CreateNight(now)

	evt := &domain.Event{
		NightID:   night.ID,
		FromState: domain.NightOff,
		Action:    domain.StartNight,
		ToState:   domain.Awake,
		Timestamp: now,
		Metadata:  nil,
	}

	err := s.AddEvent(evt)
	if err != nil {
		t.Fatalf("AddEvent: %v", err)
	}
	if evt.ID == 0 {
		t.Error("expected non-zero event ID")
	}
	if evt.Seq != 1 {
		t.Errorf("first event Seq = %d, want 1", evt.Seq)
	}

	// Add a second event, verify seq increments
	evt2 := &domain.Event{
		NightID:   night.ID,
		FromState: domain.Awake,
		Action:    domain.StartFeed,
		ToState:   domain.Feeding,
		Timestamp: now.Add(5 * time.Minute),
		Metadata:  map[string]string{"breast": "L"},
	}
	err = s.AddEvent(evt2)
	if err != nil {
		t.Fatalf("AddEvent 2: %v", err)
	}
	if evt2.Seq != 2 {
		t.Errorf("second event Seq = %d, want 2", evt2.Seq)
	}

	// Read back
	_, events, err := s.GetNight(night.ID)
	if err != nil {
		t.Fatalf("GetNight: %v", err)
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

	night, _ := s.CreateNight(now)

	s.AddEvent(&domain.Event{
		NightID: night.ID, FromState: domain.NightOff,
		Action: domain.StartNight, ToState: domain.Awake, Timestamp: now,
	})
	s.AddEvent(&domain.Event{
		NightID: night.ID, FromState: domain.Awake,
		Action: domain.StartFeed, ToState: domain.Feeding,
		Timestamp: now.Add(5 * time.Minute),
		Metadata:  map[string]string{"breast": "L"},
	})

	// Pop the second event
	popped, err := s.PopEvent(night.ID)
	if err != nil {
		t.Fatalf("PopEvent: %v", err)
	}
	if popped.Action != domain.StartFeed {
		t.Errorf("popped action = %s, want start_feed", popped.Action)
	}

	// Should have 1 event left
	_, events, _ := s.GetNight(night.ID)
	if len(events) != 1 {
		t.Fatalf("got %d events after pop, want 1", len(events))
	}
}

func TestPopEventEmpty(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().Truncate(time.Millisecond)

	night, _ := s.CreateNight(now)

	_, err := s.PopEvent(night.ID)
	if err == nil {
		t.Error("PopEvent on empty night should return error")
	}
}

func TestCurrentSession(t *testing.T) {
	s := newTestStore(t)

	// No active night
	night, events, err := s.CurrentSession()
	if err != nil {
		t.Fatalf("CurrentSession: %v", err)
	}
	if night != nil {
		t.Error("expected nil night when no active session")
	}

	// Start a night
	now := time.Now().Truncate(time.Millisecond)
	n, _ := s.CreateNight(now)
	s.AddEvent(&domain.Event{
		NightID: n.ID, FromState: domain.NightOff,
		Action: domain.StartNight, ToState: domain.Awake, Timestamp: now,
	})

	night, events, err = s.CurrentSession()
	if err != nil {
		t.Fatalf("CurrentSession: %v", err)
	}
	if night == nil {
		t.Fatal("expected active night")
	}
	if night.ID != n.ID {
		t.Errorf("night ID = %d, want %d", night.ID, n.ID)
	}
	if len(events) != 1 {
		t.Errorf("got %d events, want 1", len(events))
	}

	// End the night — should no longer be current
	s.EndNight(n.ID, now.Add(8*time.Hour))
	night, _, err = s.CurrentSession()
	if err != nil {
		t.Fatalf("CurrentSession after end: %v", err)
	}
	if night != nil {
		t.Error("expected nil night after ending session")
	}
}

func TestListNights(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().Truncate(time.Millisecond)

	n1, _ := s.CreateNight(now)
	s.EndNight(n1.ID, now.Add(8*time.Hour))

	n2, _ := s.CreateNight(now.Add(24 * time.Hour))
	s.EndNight(n2.ID, now.Add(32*time.Hour))

	// Query all
	nights, err := s.ListNights(now.Add(-time.Hour), now.Add(48*time.Hour))
	if err != nil {
		t.Fatalf("ListNights: %v", err)
	}
	if len(nights) != 2 {
		t.Errorf("got %d nights, want 2", len(nights))
	}

	// Query subset
	nights, err = s.ListNights(now.Add(12*time.Hour), now.Add(48*time.Hour))
	if err != nil {
		t.Fatalf("ListNights: %v", err)
	}
	if len(nights) != 1 {
		t.Errorf("got %d nights, want 1", len(nights))
	}
}

func TestGetEventsForNights(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().Truncate(time.Millisecond)

	n1, _ := s.CreateNight(now)
	s.AddEvent(&domain.Event{
		NightID: n1.ID, FromState: domain.NightOff,
		Action: domain.StartNight, ToState: domain.Awake, Timestamp: now,
	})
	s.AddEvent(&domain.Event{
		NightID: n1.ID, FromState: domain.Awake,
		Action: domain.EndNight, ToState: domain.NightOff, Timestamp: now.Add(time.Hour),
	})
	s.EndNight(n1.ID, now.Add(time.Hour))

	n2, _ := s.CreateNight(now.Add(24 * time.Hour))
	s.AddEvent(&domain.Event{
		NightID: n2.ID, FromState: domain.NightOff,
		Action: domain.StartNight, ToState: domain.Awake, Timestamp: now.Add(24 * time.Hour),
	})
	s.EndNight(n2.ID, now.Add(25*time.Hour))

	// Batch fetch
	eventsMap, err := s.GetEventsForNights([]int64{n1.ID, n2.ID})
	if err != nil {
		t.Fatalf("GetEventsForNights: %v", err)
	}
	if len(eventsMap[n1.ID]) != 2 {
		t.Errorf("night 1: got %d events, want 2", len(eventsMap[n1.ID]))
	}
	if len(eventsMap[n2.ID]) != 1 {
		t.Errorf("night 2: got %d events, want 1", len(eventsMap[n2.ID]))
	}

	// Empty input
	eventsMap, err = s.GetEventsForNights(nil)
	if err != nil {
		t.Fatalf("GetEventsForNights(nil): %v", err)
	}
	if len(eventsMap) != 0 {
		t.Errorf("empty input: got %d entries, want 0", len(eventsMap))
	}
}

func TestDeleteNight(t *testing.T) {
	s := newTestStore(t)
	now := time.Now().Truncate(time.Millisecond)

	night, _ := s.CreateNight(now)
	s.AddEvent(&domain.Event{
		NightID: night.ID, FromState: domain.NightOff,
		Action: domain.StartNight, ToState: domain.Awake, Timestamp: now,
	})

	err := s.DeleteNight(night.ID)
	if err != nil {
		t.Fatalf("DeleteNight: %v", err)
	}

	got, _, err := s.GetNight(night.ID)
	if err != nil {
		t.Fatalf("GetNight: %v", err)
	}
	if got != nil {
		t.Error("expected nil night after delete")
	}
}

func TestMigrateAddsFerberColumns(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "test.db")
	s, err := New(dbPath)
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	// Re-running New on the same file should be a no-op (idempotent migration).
	s.Close()
	s2, err := New(dbPath)
	if err != nil {
		t.Fatalf("New (re-open): %v", err)
	}
	defer s2.Close()

	rows, err := s2.db.Query("PRAGMA table_info(nights)")
	if err != nil {
		t.Fatalf("PRAGMA: %v", err)
	}
	defer rows.Close()
	seen := map[string]bool{}
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			t.Fatalf("scan: %v", err)
		}
		seen[name] = true
	}
	for _, col := range []string{"ferber_enabled", "ferber_night_number"} {
		if !seen[col] {
			t.Errorf("expected column %q on nights, not found", col)
		}
	}
}
