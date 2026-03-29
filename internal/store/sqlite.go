package store

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	"github.com/polina/boob-o-clock/internal/domain"
	_ "modernc.org/sqlite"
)

type Store struct {
	db     *sql.DB
	dbPath string
}

func New(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_journal_mode=WAL")
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}

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

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
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
	`)
	return err
}

func (s *Store) CreateNight(startedAt time.Time) (*domain.Night, error) {
	now := time.Now()
	result, err := s.db.Exec(
		"INSERT INTO nights (started_at, created_at) VALUES (?, ?)",
		startedAt.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("insert night: %w", err)
	}

	id, _ := result.LastInsertId()
	return &domain.Night{
		ID:        id,
		StartedAt: startedAt,
		CreatedAt: now,
	}, nil
}

func (s *Store) EndNight(nightID int64, endedAt time.Time) error {
	_, err := s.db.Exec(
		"UPDATE nights SET ended_at = ? WHERE id = ?",
		endedAt.Format(time.RFC3339Nano), nightID,
	)
	return err
}

func (s *Store) DeleteNight(nightID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec("DELETE FROM events WHERE night_id = ?", nightID); err != nil {
		return err
	}
	if _, err := tx.Exec("DELETE FROM nights WHERE id = ?", nightID); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) AddEvent(evt *domain.Event) error {
	// Get next seq for this night
	var maxSeq sql.NullInt64
	err := s.db.QueryRow(
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
		metadataJSON, _ = json.Marshal(evt.Metadata)
	}

	evt.CreatedAt = time.Now()
	result, err := s.db.Exec(
		`INSERT INTO events (night_id, from_state, action, to_state, timestamp, metadata, created_at, seq)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		evt.NightID, string(evt.FromState), string(evt.Action), string(evt.ToState),
		evt.Timestamp.Format(time.RFC3339Nano), metadataJSON,
		evt.CreatedAt.Format(time.RFC3339Nano), evt.Seq,
	)
	if err != nil {
		return fmt.Errorf("insert event: %w", err)
	}

	evt.ID, _ = result.LastInsertId()
	return nil
}

func (s *Store) PopEvent(nightID int64) (*domain.Event, error) {
	var evt domain.Event
	var metadataJSON sql.NullString
	var ts, createdAt string

	err := s.db.QueryRow(
		`SELECT id, night_id, from_state, action, to_state, timestamp, metadata, created_at, seq
		 FROM events WHERE night_id = ? ORDER BY seq DESC LIMIT 1`, nightID,
	).Scan(&evt.ID, &evt.NightID, &evt.FromState, &evt.Action, &evt.ToState,
		&ts, &metadataJSON, &createdAt, &evt.Seq)
	if err == sql.ErrNoRows {
		return nil, fmt.Errorf("no events to undo")
	}
	if err != nil {
		return nil, fmt.Errorf("query last event: %w", err)
	}

	evt.Timestamp, _ = time.Parse(time.RFC3339Nano, ts)
	evt.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	if metadataJSON.Valid {
		json.Unmarshal([]byte(metadataJSON.String), &evt.Metadata)
	}

	_, err = s.db.Exec("DELETE FROM events WHERE id = ?", evt.ID)
	if err != nil {
		return nil, fmt.Errorf("delete event: %w", err)
	}

	return &evt, nil
}

func (s *Store) CurrentSession() (*domain.Night, []domain.Event, error) {
	night, err := s.scanNight(
		s.db.QueryRow("SELECT id, started_at, ended_at, created_at FROM nights WHERE ended_at IS NULL ORDER BY id DESC LIMIT 1"),
	)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("query current night: %w", err)
	}

	events, err := s.getEvents(night.ID)
	if err != nil {
		return nil, nil, err
	}

	return night, events, nil
}

func (s *Store) GetNight(id int64) (*domain.Night, []domain.Event, error) {
	night, err := s.scanNight(
		s.db.QueryRow("SELECT id, started_at, ended_at, created_at FROM nights WHERE id = ?", id),
	)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("query night: %w", err)
	}

	events, err := s.getEvents(id)
	if err != nil {
		return nil, nil, err
	}

	return night, events, nil
}

func (s *Store) ListNights(from, to time.Time) ([]domain.Night, error) {
	rows, err := s.db.Query(
		"SELECT id, started_at, ended_at, created_at FROM nights WHERE started_at >= ? AND started_at < ? ORDER BY started_at",
		from.Format(time.RFC3339Nano), to.Format(time.RFC3339Nano),
	)
	if err != nil {
		return nil, fmt.Errorf("query nights: %w", err)
	}
	defer rows.Close()

	var nights []domain.Night
	for rows.Next() {
		var n domain.Night
		var startedAt, createdAt string
		var endedAt sql.NullString

		if err := rows.Scan(&n.ID, &startedAt, &endedAt, &createdAt); err != nil {
			return nil, fmt.Errorf("scan night: %w", err)
		}
		n.StartedAt, _ = time.Parse(time.RFC3339Nano, startedAt)
		n.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		if endedAt.Valid {
			t, _ := time.Parse(time.RFC3339Nano, endedAt.String)
			n.EndedAt = &t
		}
		nights = append(nights, n)
	}
	return nights, rows.Err()
}

// -- internal helpers --

type scanner interface {
	Scan(dest ...any) error
}

func (s *Store) scanNight(row scanner) (*domain.Night, error) {
	var n domain.Night
	var startedAt, createdAt string
	var endedAt sql.NullString

	if err := row.Scan(&n.ID, &startedAt, &endedAt, &createdAt); err != nil {
		return nil, err
	}
	n.StartedAt, _ = time.Parse(time.RFC3339Nano, startedAt)
	n.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
	if endedAt.Valid {
		t, _ := time.Parse(time.RFC3339Nano, endedAt.String)
		n.EndedAt = &t
	}
	return &n, nil
}

func (s *Store) getEvents(nightID int64) ([]domain.Event, error) {
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
		var evt domain.Event
		var ts, createdAt string
		var metadataJSON sql.NullString

		if err := rows.Scan(&evt.ID, &evt.NightID, &evt.FromState, &evt.Action, &evt.ToState,
			&ts, &metadataJSON, &createdAt, &evt.Seq); err != nil {
			return nil, fmt.Errorf("scan event: %w", err)
		}
		evt.Timestamp, _ = time.Parse(time.RFC3339Nano, ts)
		evt.CreatedAt, _ = time.Parse(time.RFC3339Nano, createdAt)
		if metadataJSON.Valid {
			json.Unmarshal([]byte(metadataJSON.String), &evt.Metadata)
		}
		events = append(events, evt)
	}
	return events, rows.Err()
}
