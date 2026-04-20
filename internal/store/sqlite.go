package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/liviro/boob-o-clock/internal/domain"
	_ "modernc.org/sqlite"
)

type Store struct {
	db     *sql.DB
	dbPath string
}

func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL&_busy_timeout=5000")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	db.SetMaxOpenConns(1)

	s := &Store{db: db, dbPath: path}
	if err := s.migrate(); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return s, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Ping() error {
	return s.db.Ping()
}

func (s *Store) migrate() error {
	if _, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS nights (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			started_at TEXT NOT NULL,
			ended_at TEXT,
			created_at TEXT NOT NULL
		);

		CREATE TABLE IF NOT EXISTS events (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			night_id INTEGER NOT NULL REFERENCES nights(id) ON DELETE CASCADE,
			from_state TEXT NOT NULL,
			action TEXT NOT NULL,
			to_state TEXT NOT NULL,
			timestamp TEXT NOT NULL,
			metadata TEXT,
			created_at TEXT NOT NULL,
			seq INTEGER NOT NULL
		);

		CREATE INDEX IF NOT EXISTS idx_events_night_seq ON events(night_id, seq);

		PRAGMA foreign_keys = ON;
	`); err != nil {
		return err
	}

	// Idempotent column additions for Ferber mode.
	if err := s.addColumnIfMissing("nights", "ferber_enabled", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := s.addColumnIfMissing("nights", "ferber_night_number", "INTEGER"); err != nil {
		return err
	}
	return nil
}

func (s *Store) addColumnIfMissing(table, column, typeDecl string) error {
	rows, err := s.db.Query(fmt.Sprintf("PRAGMA table_info(%s)", table))
	if err != nil {
		return fmt.Errorf("pragma %s: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		if err := rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk); err != nil {
			return err
		}
		if name == column {
			return nil // already present
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	_, err = s.db.Exec(fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s", table, column, typeDecl))
	return err
}

func (s *Store) CreateNight(startedAt time.Time, ferberEnabled bool, ferberNightNumber int) (*domain.Night, error) {
	now := time.Now()
	var ferberNumArg any
	var ferberNumPtr *int
	if ferberEnabled {
		n := ferberNightNumber
		ferberNumArg = n
		ferberNumPtr = &n
	}
	ferberEnabledInt := 0
	if ferberEnabled {
		ferberEnabledInt = 1
	}

	result, err := s.db.Exec(
		`INSERT INTO nights (started_at, created_at, ferber_enabled, ferber_night_number)
		 VALUES (?, ?, ?, ?)`,
		startedAt.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano),
		ferberEnabledInt, ferberNumArg,
	)
	if err != nil {
		return nil, fmt.Errorf("insert night: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
	return &domain.Night{
		ID:                id,
		StartedAt:         startedAt,
		CreatedAt:         now,
		FerberEnabled:     ferberEnabled,
		FerberNightNumber: ferberNumPtr,
	}, nil
}

func (s *Store) EndNight(nightID int64, endedAt time.Time) error {
	_, err := s.db.Exec(
		"UPDATE nights SET ended_at = ? WHERE id = ?",
		endedAt.Format(time.RFC3339Nano), nightID,
	)
	if err != nil {
		return fmt.Errorf("end night: %w", err)
	}
	return nil
}

func (s *Store) DeleteNight(nightID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("delete night: begin tx: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM events WHERE night_id = ?", nightID); err != nil {
		return fmt.Errorf("delete night: delete events: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM nights WHERE id = ?", nightID); err != nil {
		return fmt.Errorf("delete night: delete row: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("delete night: commit: %w", err)
	}
	return nil
}

func (s *Store) AddEvent(evt *domain.Event) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	var maxSeq sql.NullInt64
	err = tx.QueryRow(
		"SELECT MAX(seq) FROM events WHERE night_id = ?", evt.NightID,
	).Scan(&maxSeq)
	if err != nil {
		return fmt.Errorf("get max seq: %w", err)
	}

	evt.Seq = 1
	if maxSeq.Valid {
		evt.Seq = int(maxSeq.Int64) + 1
	}

	var metadataJSON []byte
	if evt.Metadata != nil {
		metadataJSON, err = json.Marshal(evt.Metadata)
		if err != nil {
			return fmt.Errorf("marshal metadata: %w", err)
		}
	}

	evt.CreatedAt = time.Now()
	result, err := tx.Exec(
		`INSERT INTO events (night_id, from_state, action, to_state, timestamp, metadata, created_at, seq)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		evt.NightID, string(evt.FromState), string(evt.Action), string(evt.ToState),
		evt.Timestamp.Format(time.RFC3339Nano), metadataJSON,
		evt.CreatedAt.Format(time.RFC3339Nano), evt.Seq,
	)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}

	evt.ID, err = result.LastInsertId()
	if err != nil {
		return fmt.Errorf("last insert id: %w", err)
	}
	return tx.Commit()
}

func (s *Store) PopEvent(nightID int64) (*domain.Event, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	evt, err := scanEvent(tx.QueryRow(
		`SELECT id, night_id, from_state, action, to_state, timestamp, metadata, created_at, seq
		 FROM events WHERE night_id = ? ORDER BY seq DESC LIMIT 1`, nightID,
	))
	if err != nil {
		return nil, fmt.Errorf("query last event: %w", err)
	}

	if _, err = tx.Exec("DELETE FROM events WHERE id = ?", evt.ID); err != nil {
		return nil, fmt.Errorf("delete event: %w", err)
	}

	return evt, tx.Commit()
}

func (s *Store) CurrentSession() (*domain.Night, []domain.Event, error) {
	night, err := s.scanNight(
		s.db.QueryRow("SELECT id, started_at, ended_at, created_at, ferber_enabled, ferber_night_number FROM nights WHERE ended_at IS NULL ORDER BY id DESC LIMIT 1"),
	)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("query current night: %w", err)
	}

	events, err := s.GetEvents(night.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("current session: %w", err)
	}

	return night, events, nil
}

func (s *Store) GetNight(id int64) (*domain.Night, []domain.Event, error) {
	night, err := s.scanNight(
		s.db.QueryRow("SELECT id, started_at, ended_at, created_at, ferber_enabled, ferber_night_number FROM nights WHERE id = ?", id),
	)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("query night: %w", err)
	}

	events, err := s.GetEvents(id)
	if err != nil {
		return nil, nil, fmt.Errorf("get night: %w", err)
	}

	return night, events, nil
}

func (s *Store) ListNights(from, to time.Time) ([]domain.Night, error) {
	rows, err := s.db.Query(
		"SELECT id, started_at, ended_at, created_at, ferber_enabled, ferber_night_number FROM nights WHERE started_at >= ? AND started_at < ? ORDER BY started_at",
		from.Format(time.RFC3339Nano), to.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("query nights: %w", err)
	}
	defer rows.Close()

	var nights []domain.Night
	for rows.Next() {
		n, err := s.scanNight(rows)
		if err != nil {
			return nil, fmt.Errorf("scan night: %w", err)
		}
		nights = append(nights, *n)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list nights: iterate: %w", err)
	}
	return nights, nil
}

// LastNight returns the most recently started night (by started_at),
// regardless of whether it has ended. Returns nil if no nights exist.
func (s *Store) LastNight() (*domain.Night, error) {
	night, err := s.scanNight(
		s.db.QueryRow(
			`SELECT id, started_at, ended_at, created_at, ferber_enabled, ferber_night_number
			 FROM nights ORDER BY started_at DESC LIMIT 1`,
		),
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query last night: %w", err)
	}
	return night, nil
}

// -- internal helpers --

type scanner interface {
	Scan(dest ...any) error
}

func (s *Store) scanNight(row scanner) (*domain.Night, error) {
	var n domain.Night
	var startedAt, createdAt string
	var endedAt sql.NullString
	var ferberEnabled int
	var ferberNightNumber sql.NullInt64

	if err := row.Scan(&n.ID, &startedAt, &endedAt, &createdAt, &ferberEnabled, &ferberNightNumber); err != nil {
		return nil, err
	}
	var err error
	n.StartedAt, err = time.Parse(time.RFC3339Nano, startedAt)
	if err != nil {
		return nil, fmt.Errorf("parse startedAt: %w", err)
	}
	n.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse createdAt: %w", err)
	}
	if endedAt.Valid {
		t, err := time.Parse(time.RFC3339Nano, endedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse endedAt: %w", err)
		}
		n.EndedAt = &t
	}
	n.FerberEnabled = ferberEnabled == 1
	if ferberNightNumber.Valid {
		v := int(ferberNightNumber.Int64)
		n.FerberNightNumber = &v
	}
	return &n, nil
}

// GetAllEvents returns all events across all nights, ordered by night and sequence.
func (s *Store) GetAllEvents() ([]domain.Event, error) {
	rows, err := s.db.Query(
		`SELECT id, night_id, from_state, action, to_state, timestamp, metadata, created_at, seq
		 FROM events ORDER BY night_id, seq`,
	)
	if err != nil {
		return nil, fmt.Errorf("query all events: %w", err)
	}
	defer rows.Close()

	var events []domain.Event
	for rows.Next() {
		evt, err := scanEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("get all events: %w", err)
		}
		events = append(events, *evt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get all events: iterate: %w", err)
	}
	return events, nil
}

// GetEventsForNights fetches events for multiple nights in a single query,
// returning them partitioned by night ID.
func (s *Store) GetEventsForNights(nightIDs []int64) (map[int64][]domain.Event, error) {
	if len(nightIDs) == 0 {
		return map[int64][]domain.Event{}, nil
	}

	args := make([]any, len(nightIDs))
	for i, id := range nightIDs {
		args[i] = id
	}
	placeholders := strings.Repeat("?,", len(nightIDs))
	placeholders = placeholders[:len(placeholders)-1] // trim trailing comma

	rows, err := s.db.Query(
		`SELECT id, night_id, from_state, action, to_state, timestamp, metadata, created_at, seq
		 FROM events WHERE night_id IN (`+placeholders+`) ORDER BY night_id, seq`, args...,
	)
	if err != nil {
		return nil, fmt.Errorf("query events batch: %w", err)
	}
	defer rows.Close()

	result := make(map[int64][]domain.Event)
	for rows.Next() {
		evt, err := scanEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("get events batch: %w", err)
		}
		result[evt.NightID] = append(result[evt.NightID], *evt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get events batch: iterate: %w", err)
	}
	return result, nil
}

func (s *Store) GetEvents(nightID int64) ([]domain.Event, error) {
	rows, err := s.db.Query(
		`SELECT id, night_id, from_state, action, to_state, timestamp, metadata, created_at, seq
		 FROM events WHERE night_id = ? ORDER BY seq`, nightID,
	)
	if err != nil {
		return nil, fmt.Errorf("query events: %w", err)
	}
	defer rows.Close()

	var events []domain.Event
	for rows.Next() {
		evt, err := scanEvent(rows)
		if err != nil {
			return nil, fmt.Errorf("get events: %w", err)
		}
		events = append(events, *evt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get events: iterate: %w", err)
	}
	return events, nil
}

func scanEvent(row scanner) (*domain.Event, error) {
	var evt domain.Event
	var ts, createdAt string
	var metadataJSON sql.NullString

	if err := row.Scan(&evt.ID, &evt.NightID, &evt.FromState, &evt.Action, &evt.ToState,
		&ts, &metadataJSON, &createdAt, &evt.Seq); err != nil {
		return nil, fmt.Errorf("scan event: %w", err)
	}
	var err error
	evt.Timestamp, err = time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return nil, fmt.Errorf("parse timestamp: %w", err)
	}
	evt.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse createdAt: %w", err)
	}
	if metadataJSON.Valid {
		if err := json.Unmarshal([]byte(metadataJSON.String), &evt.Metadata); err != nil {
			return nil, fmt.Errorf("unmarshal metadata: %w", err)
		}
	}
	return &evt, nil
}
