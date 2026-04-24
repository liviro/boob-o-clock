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

// --- schema DDL (as constants so fresh and legacy paths stay in sync) ---

const sessionsTableDDL = `CREATE TABLE IF NOT EXISTS sessions (
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	kind TEXT NOT NULL CHECK (kind IN ('night','day')),
	started_at TEXT NOT NULL,
	ended_at TEXT,
	created_at TEXT NOT NULL,
	ferber_enabled INTEGER NOT NULL DEFAULT 0,
	ferber_night_number INTEGER
)`

// Indexing a constant expression rather than ended_at is deliberate: SQLite
// treats multiple NULLs as distinct even inside partial UNIQUE indexes, so
// UNIQUE(ended_at) WHERE ended_at IS NULL would silently permit any number of
// open sessions. Indexing the constant 1 (same value for every matching row)
// makes the UNIQUE constraint bite — at most one row may match the predicate.
const oneOpenSessionIndexDDL = `CREATE UNIQUE INDEX IF NOT EXISTS one_open_session ON sessions((1)) WHERE ended_at IS NULL`

// The column list is shared between the fresh-install DDL below and the
// phase-C table rewrite in migrateLegacy, so both paths stay in sync.
const eventsColumnsDDL = `(
	id INTEGER PRIMARY KEY AUTOINCREMENT,
	session_id INTEGER NOT NULL REFERENCES sessions(id) ON DELETE CASCADE,
	from_state TEXT NOT NULL,
	action TEXT NOT NULL,
	to_state TEXT NOT NULL,
	timestamp TEXT NOT NULL,
	metadata TEXT,
	created_at TEXT NOT NULL,
	seq INTEGER NOT NULL
)`

const newEventsTableDDL = `CREATE TABLE IF NOT EXISTS events ` + eventsColumnsDDL

const eventsSeqIndexDDL = `CREATE INDEX IF NOT EXISTS idx_events_session_seq ON events(session_id, seq)`

// migrate runs once per process start. Three paths:
//   - sessions table already exists → already migrated, no-op (just set pragma).
//   - nights table exists, sessions doesn't → legacy DB, run full migration.
//   - neither exists → fresh install, create new schema directly.
func (s *Store) migrate() error {
	sessionsExists, err := tableExists(s.db, "sessions")
	if err != nil {
		return err
	}
	if sessionsExists {
		_, err := s.db.Exec(`PRAGMA foreign_keys = ON`)
		return err
	}

	nightsExists, err := tableExists(s.db, "nights")
	if err != nil {
		return err
	}
	if nightsExists {
		return s.migrateLegacy()
	}
	return s.createFreshSchema()
}

// createFreshSchema is the first-install path.
func (s *Store) createFreshSchema() error {
	stmts := []string{
		sessionsTableDDL,
		oneOpenSessionIndexDDL,
		newEventsTableDDL,
		eventsSeqIndexDDL,
		`PRAGMA foreign_keys = ON`,
	}
	for _, stmt := range stmts {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("createFreshSchema: %w", err)
		}
	}
	return nil
}

// migrateLegacy migrates a pre-day-mode DB (nights + events with night_id)
// to the new schema (sessions + events with session_id).
//
// All phases run inside a single transaction, so any crash — process kill,
// OS panic, power loss — rolls the DB back to its exact pre-migration state.
// There is no "half-migrated" outcome: either the migration commits in full
// and the old schema is gone, or the original data is untouched.
//
// One-time anomaly: there is a timestamp gap between the last pre-migration
// end_night event and the first post-migration start_day/start_night event.
// See design doc §7.4 — not worth bespoke code; user can manually close via
// long-press timestamp picker if visual continuity is desired.
func (s *Store) migrateLegacy() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("migrateLegacy begin: %w", err)
	}
	defer tx.Rollback()

	// Each DDL needs its own Exec — database/sql does not reliably run
	// multi-statement strings through modernc.org/sqlite.
	if _, err := tx.Exec(sessionsTableDDL); err != nil {
		return fmt.Errorf("sessions ddl: %w", err)
	}
	if _, err := tx.Exec(oneOpenSessionIndexDDL); err != nil {
		return fmt.Errorf("sessions unique index: %w", err)
	}

	// night.id becomes session.id 1:1 so the events copy below can alias
	// night_id → session_id without a mapping table.
	if _, err := tx.Exec(`
		INSERT INTO sessions (id, kind, started_at, ended_at, created_at, ferber_enabled, ferber_night_number)
		SELECT id, 'night', started_at, ended_at, created_at, ferber_enabled, ferber_night_number FROM nights
	`); err != nil {
		return fmt.Errorf("backfill sessions: %w", err)
	}

	// Rebuild events with the new session_id FK, preserving every column.
	if _, err := tx.Exec(`CREATE TABLE events_new ` + eventsColumnsDDL); err != nil {
		return fmt.Errorf("create events_new: %w", err)
	}
	if _, err := tx.Exec(`
		INSERT INTO events_new (id, session_id, from_state, action, to_state, timestamp, metadata, created_at, seq)
		SELECT id, night_id, from_state, action, to_state, timestamp, metadata, created_at, seq FROM events
	`); err != nil {
		return fmt.Errorf("copy events: %w", err)
	}
	if _, err := tx.Exec(`DROP TABLE events`); err != nil {
		return fmt.Errorf("drop old events: %w", err)
	}
	if _, err := tx.Exec(`ALTER TABLE events_new RENAME TO events`); err != nil {
		return fmt.Errorf("rename events_new: %w", err)
	}
	if _, err := tx.Exec(eventsSeqIndexDDL); err != nil {
		return fmt.Errorf("events seq index: %w", err)
	}
	if _, err := tx.Exec(`DROP TABLE nights`); err != nil {
		return fmt.Errorf("drop nights: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("migrateLegacy commit: %w", err)
	}

	// PRAGMA foreign_keys must be set outside a transaction to take effect.
	if _, err := s.db.Exec(`PRAGMA foreign_keys = ON`); err != nil {
		return fmt.Errorf("pragma foreign_keys: %w", err)
	}

	return nil
}

// tableExists reports whether a table by the given name exists in the DB.
func tableExists(db *sql.DB, name string) (bool, error) {
	var got string
	err := db.QueryRow(
		`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, name,
	).Scan(&got)
	if err == sql.ErrNoRows {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("tableExists(%s): %w", name, err)
	}
	return true, nil
}

// --- session mutators ---

// CreateSession inserts a new session row. ferberEnabled only meaningful when
// kind == SessionKindNight; callers must pass false (and a zero number) for day.
func (s *Store) CreateSession(kind domain.SessionKind, startedAt time.Time, ferberEnabled bool, ferberNightNumber int) (*domain.Session, error) {
	now := time.Now()
	var ferberNumArg any
	var ferberNumPtr *int
	ferberEnabledInt := 0
	if ferberEnabled {
		n := ferberNightNumber
		ferberNumArg = n
		ferberNumPtr = &n
		ferberEnabledInt = 1
	}

	result, err := s.db.Exec(
		`INSERT INTO sessions (kind, started_at, created_at, ferber_enabled, ferber_night_number)
		 VALUES (?, ?, ?, ?, ?)`,
		string(kind),
		startedAt.Format(time.RFC3339Nano), now.Format(time.RFC3339Nano),
		ferberEnabledInt, ferberNumArg,
	)
	if err != nil {
		return nil, fmt.Errorf("insert session: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("last insert id: %w", err)
	}
	return &domain.Session{
		ID:                id,
		Kind:              kind,
		StartedAt:         startedAt,
		CreatedAt:         now,
		FerberEnabled:     ferberEnabled,
		FerberNightNumber: ferberNumPtr,
	}, nil
}

// StartNewSession atomically closes any currently-open session and creates a
// new session, writing startEvent as the new session's first event. All three
// operations happen inside one BEGIN/COMMIT so the unique partial index
// invariant "at most one open session" is never transiently violated.
//
// Used by POST /api/session/start for both first-start (no prior session) and
// chain advance (close old, open new). On first-start, the UPDATE is a no-op.
//
// startEvent is mutated: SessionID, Seq (=1), ID, and CreatedAt are populated.
// The session's started_at is set to startEvent.Timestamp — they must match for
// the chain-advance invariant (old.ended_at == new.started_at).
func (s *Store) StartNewSession(
	kind domain.SessionKind,
	ferberEnabled bool,
	ferberNightNumber int,
	startEvent *domain.Event,
) (*domain.Session, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("start session: begin tx: %w", err)
	}
	defer tx.Rollback()

	ts := startEvent.Timestamp.Format(time.RFC3339Nano)

	// 1. Close any currently-open session at startEvent.Timestamp.
	if _, err := tx.Exec(
		"UPDATE sessions SET ended_at = ? WHERE ended_at IS NULL",
		ts,
	); err != nil {
		return nil, fmt.Errorf("start session: close prior: %w", err)
	}

	// 2. Insert new session row.
	now := time.Now()
	var ferberNumArg any
	var ferberNumPtr *int
	ferberEnabledInt := 0
	if ferberEnabled {
		n := ferberNightNumber
		ferberNumArg = n
		ferberNumPtr = &n
		ferberEnabledInt = 1
	}

	res, err := tx.Exec(
		`INSERT INTO sessions (kind, started_at, created_at, ferber_enabled, ferber_night_number)
		 VALUES (?, ?, ?, ?, ?)`,
		string(kind), ts, now.Format(time.RFC3339Nano),
		ferberEnabledInt, ferberNumArg,
	)
	if err != nil {
		return nil, fmt.Errorf("start session: insert session: %w", err)
	}
	sessID, err := res.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("start session: session last id: %w", err)
	}

	// 3. Insert start event with seq=1.
	startEvent.SessionID = sessID
	startEvent.Seq = 1
	startEvent.CreatedAt = now

	var metadataJSON []byte
	if startEvent.Metadata != nil {
		metadataJSON, err = json.Marshal(startEvent.Metadata)
		if err != nil {
			return nil, fmt.Errorf("start session: marshal metadata: %w", err)
		}
	}

	evtRes, err := tx.Exec(
		`INSERT INTO events (session_id, from_state, action, to_state, timestamp, metadata, created_at, seq)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		startEvent.SessionID,
		string(startEvent.FromState), string(startEvent.Action), string(startEvent.ToState),
		ts, metadataJSON,
		startEvent.CreatedAt.Format(time.RFC3339Nano), startEvent.Seq,
	)
	if err != nil {
		return nil, fmt.Errorf("start session: insert event: %w", err)
	}
	startEvent.ID, err = evtRes.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("start session: event last id: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("start session: commit: %w", err)
	}

	return &domain.Session{
		ID:                sessID,
		Kind:              kind,
		StartedAt:         startEvent.Timestamp,
		CreatedAt:         now,
		FerberEnabled:     ferberEnabled,
		FerberNightNumber: ferberNumPtr,
	}, nil
}

// UndoChainAdvance atomically reverses a chain advance: deletes the newSession
// (and its single start event) and clears ended_at on prevSession. Ordering
// is critical — delete first, then clear — so the unique partial index never
// sees two open sessions within the transaction (SQLite checks UNIQUE at
// statement level).
func (s *Store) UndoChainAdvance(newSessionID, prevSessionID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("undo chain: begin tx: %w", err)
	}
	defer tx.Rollback()

	// 1. Delete events for the new session first (explicit, not relying on FK cascade).
	if _, err := tx.Exec("DELETE FROM events WHERE session_id = ?", newSessionID); err != nil {
		return fmt.Errorf("undo chain: delete events: %w", err)
	}
	// 2. Delete the new session row.
	if _, err := tx.Exec("DELETE FROM sessions WHERE id = ?", newSessionID); err != nil {
		return fmt.Errorf("undo chain: delete session: %w", err)
	}
	// 3. Now safe to reopen the prior session — no other open session exists.
	if _, err := tx.Exec("UPDATE sessions SET ended_at = NULL WHERE id = ?", prevSessionID); err != nil {
		return fmt.Errorf("undo chain: reopen prior: %w", err)
	}
	return tx.Commit()
}

// EndSession sets ended_at on the given session. Kind-agnostic.
func (s *Store) EndSession(sessionID int64, endedAt time.Time) error {
	_, err := s.db.Exec(
		"UPDATE sessions SET ended_at = ? WHERE id = ?",
		endedAt.Format(time.RFC3339Nano), sessionID,
	)
	if err != nil {
		return fmt.Errorf("end session: %w", err)
	}
	return nil
}

// DeleteSession removes a session and (via ON DELETE CASCADE) its events.
func (s *Store) DeleteSession(sessionID int64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("delete session: begin tx: %w", err)
	}
	defer tx.Rollback()

	// Explicit events delete for safety; FK cascade would also do this.
	if _, err := tx.Exec("DELETE FROM events WHERE session_id = ?", sessionID); err != nil {
		return fmt.Errorf("delete session: delete events: %w", err)
	}
	if _, err := tx.Exec("DELETE FROM sessions WHERE id = ?", sessionID); err != nil {
		return fmt.Errorf("delete session: delete row: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("delete session: commit: %w", err)
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
		"SELECT MAX(seq) FROM events WHERE session_id = ?", evt.SessionID,
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
		`INSERT INTO events (session_id, from_state, action, to_state, timestamp, metadata, created_at, seq)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		evt.SessionID, string(evt.FromState), string(evt.Action), string(evt.ToState),
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

func (s *Store) PopEvent(sessionID int64) (*domain.Event, error) {
	tx, err := s.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	evt, err := scanEvent(tx.QueryRow(
		`SELECT id, session_id, from_state, action, to_state, timestamp, metadata, created_at, seq
		 FROM events WHERE session_id = ? ORDER BY seq DESC LIMIT 1`, sessionID,
	))
	if err != nil {
		return nil, fmt.Errorf("query last event: %w", err)
	}

	if _, err = tx.Exec("DELETE FROM events WHERE id = ?", evt.ID); err != nil {
		return nil, fmt.Errorf("delete event: %w", err)
	}

	return evt, tx.Commit()
}

// --- session queries ---

const sessionColumns = `id, kind, started_at, ended_at, created_at, ferber_enabled, ferber_night_number`

func (s *Store) CurrentSession() (*domain.Session, []domain.Event, error) {
	session, err := s.scanSession(
		s.db.QueryRow(`SELECT ` + sessionColumns + ` FROM sessions WHERE ended_at IS NULL ORDER BY id DESC LIMIT 1`),
	)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("query current session: %w", err)
	}

	events, err := s.GetEvents(session.ID)
	if err != nil {
		return nil, nil, fmt.Errorf("current session: %w", err)
	}

	return session, events, nil
}

func (s *Store) GetSession(id int64) (*domain.Session, []domain.Event, error) {
	session, err := s.scanSession(
		s.db.QueryRow(`SELECT `+sessionColumns+` FROM sessions WHERE id = ?`, id),
	)
	if err == sql.ErrNoRows {
		return nil, nil, nil
	}
	if err != nil {
		return nil, nil, fmt.Errorf("query session: %w", err)
	}

	events, err := s.GetEvents(id)
	if err != nil {
		return nil, nil, fmt.Errorf("get session: %w", err)
	}

	return session, events, nil
}

// ListSessions returns all sessions whose started_at is in [from, to), ordered
// chronologically. An empty kind filter returns sessions of both kinds.
func (s *Store) ListSessions(from, to time.Time, kind domain.SessionKind) ([]domain.Session, error) {
	var rows *sql.Rows
	var err error
	if kind == "" {
		rows, err = s.db.Query(
			`SELECT `+sessionColumns+` FROM sessions WHERE started_at >= ? AND started_at < ? ORDER BY started_at`,
			from.Format(time.RFC3339Nano), to.Format(time.RFC3339Nano),
		)
	} else {
		rows, err = s.db.Query(
			`SELECT `+sessionColumns+` FROM sessions WHERE started_at >= ? AND started_at < ? AND kind = ? ORDER BY started_at`,
			from.Format(time.RFC3339Nano), to.Format(time.RFC3339Nano), string(kind),
		)
	}
	if err != nil {
		return nil, fmt.Errorf("query sessions: %w", err)
	}
	defer rows.Close()

	var sessions []domain.Session
	for rows.Next() {
		sess, err := s.scanSession(rows)
		if err != nil {
			return nil, fmt.Errorf("scan session: %w", err)
		}
		sessions = append(sessions, *sess)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("list sessions: iterate: %w", err)
	}
	return sessions, nil
}

// LastSession returns the most recently started session matching the kind
// filter. Empty kind matches any. Returns nil if none exist.
func (s *Store) LastSession(kind domain.SessionKind) (*domain.Session, error) {
	var row *sql.Row
	if kind == "" {
		row = s.db.QueryRow(`SELECT ` + sessionColumns + ` FROM sessions ORDER BY started_at DESC LIMIT 1`)
	} else {
		row = s.db.QueryRow(`SELECT `+sessionColumns+` FROM sessions WHERE kind = ? ORDER BY started_at DESC LIMIT 1`, string(kind))
	}
	sess, err := s.scanSession(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("query last session: %w", err)
	}
	return sess, nil
}

// PrevSessionBefore returns the session with the largest id strictly less than
// the given id, or nil if none exists. Used by the cycle resolver and
// chain-advance undo detection.
func (s *Store) PrevSessionBefore(id int64) (*domain.Session, error) {
	sess, err := s.scanSession(
		s.db.QueryRow(`SELECT `+sessionColumns+` FROM sessions WHERE id < ? ORDER BY id DESC LIMIT 1`, id),
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("prev session: %w", err)
	}
	return sess, nil
}

// NextSessionAfter returns the session with the smallest id strictly greater
// than the given id, or nil if none exists.
func (s *Store) NextSessionAfter(id int64) (*domain.Session, error) {
	sess, err := s.scanSession(
		s.db.QueryRow(`SELECT `+sessionColumns+` FROM sessions WHERE id > ? ORDER BY id ASC LIMIT 1`, id),
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("next session: %w", err)
	}
	return sess, nil
}

// --- event queries ---

const eventColumns = `id, session_id, from_state, action, to_state, timestamp, metadata, created_at, seq`

// LastFeedStart returns the timestamp of the most recent start_feed event
// across all sessions (day or night), or nil if no feeds have been recorded.
func (s *Store) LastFeedStart() (*time.Time, error) {
	return s.queryLastEventTimestamp("action = ?", string(domain.StartFeed))
}

// LastSleepStart returns the timestamp of the most recent event that
// transitioned into a sleep state, or nil if none exist.
func (s *Store) LastSleepStart() (*time.Time, error) {
	args := make([]any, len(domain.SleepingStates))
	placeholders := make([]string, len(domain.SleepingStates))
	for i, st := range domain.SleepingStates {
		args[i] = string(st)
		placeholders[i] = "?"
	}
	return s.queryLastEventTimestamp(
		"to_state IN ("+strings.Join(placeholders, ",")+")",
		args...,
	)
}

// queryLastEventTimestamp returns the timestamp of the most recent event
// matching the given WHERE clause, or nil if none match. where is a raw
// SQL fragment — callers must only pass hardcoded strings, not user input.
func (s *Store) queryLastEventTimestamp(where string, args ...any) (*time.Time, error) {
	var ts string
	err := s.db.QueryRow(
		`SELECT timestamp FROM events WHERE `+where+` ORDER BY timestamp DESC LIMIT 1`,
		args...,
	).Scan(&ts)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("last event timestamp: %w", err)
	}
	t, err := time.Parse(time.RFC3339Nano, ts)
	if err != nil {
		return nil, fmt.Errorf("parse last event timestamp: %w", err)
	}
	return &t, nil
}

// GetAllEvents returns all events across all sessions, ordered by session and sequence.
func (s *Store) GetAllEvents() ([]domain.Event, error) {
	rows, err := s.db.Query(
		`SELECT ` + eventColumns + ` FROM events ORDER BY session_id, seq`,
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

// GetEventsForSessions fetches events for multiple sessions in a single query,
// returning them partitioned by session ID.
func (s *Store) GetEventsForSessions(sessionIDs []int64) (map[int64][]domain.Event, error) {
	if len(sessionIDs) == 0 {
		return map[int64][]domain.Event{}, nil
	}

	args := make([]any, len(sessionIDs))
	for i, id := range sessionIDs {
		args[i] = id
	}
	placeholders := strings.Repeat("?,", len(sessionIDs))
	placeholders = placeholders[:len(placeholders)-1]

	rows, err := s.db.Query(
		`SELECT `+eventColumns+` FROM events WHERE session_id IN (`+placeholders+`) ORDER BY session_id, seq`, args...,
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
		result[evt.SessionID] = append(result[evt.SessionID], *evt)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("get events batch: iterate: %w", err)
	}
	return result, nil
}

func (s *Store) GetEvents(sessionID int64) ([]domain.Event, error) {
	rows, err := s.db.Query(
		`SELECT `+eventColumns+` FROM events WHERE session_id = ? ORDER BY seq`, sessionID,
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

// --- scanners ---

type scanner interface {
	Scan(dest ...any) error
}

func (s *Store) scanSession(row scanner) (*domain.Session, error) {
	var sess domain.Session
	var kindStr, startedAt, createdAt string
	var endedAt sql.NullString
	var ferberEnabled int
	var ferberNightNumber sql.NullInt64

	if err := row.Scan(&sess.ID, &kindStr, &startedAt, &endedAt, &createdAt, &ferberEnabled, &ferberNightNumber); err != nil {
		return nil, err
	}
	sess.Kind = domain.SessionKind(kindStr)
	var err error
	sess.StartedAt, err = time.Parse(time.RFC3339Nano, startedAt)
	if err != nil {
		return nil, fmt.Errorf("parse startedAt: %w", err)
	}
	sess.CreatedAt, err = time.Parse(time.RFC3339Nano, createdAt)
	if err != nil {
		return nil, fmt.Errorf("parse createdAt: %w", err)
	}
	if endedAt.Valid {
		t, err := time.Parse(time.RFC3339Nano, endedAt.String)
		if err != nil {
			return nil, fmt.Errorf("parse endedAt: %w", err)
		}
		sess.EndedAt = &t
	}
	sess.FerberEnabled = ferberEnabled == 1
	if ferberNightNumber.Valid {
		v := int(ferberNightNumber.Int64)
		sess.FerberNightNumber = &v
	}
	return &sess, nil
}

func scanEvent(row scanner) (*domain.Event, error) {
	var evt domain.Event
	var ts, createdAt string
	var metadataJSON sql.NullString

	if err := row.Scan(&evt.ID, &evt.SessionID, &evt.FromState, &evt.Action, &evt.ToState,
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
