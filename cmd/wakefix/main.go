// Command wakefix is a maintenance tool for repairing a session whose log
// jumped straight from a sleep state into a diaper change or feed, leaving no
// AWAKE moment for the history-view splitter to land on. It splices a synthetic
// "baby woke" marker into an existing session at the times you give, so those
// instants become valid split points. After running it, open the app's History
// view and split the over-long session at each marked moment as usual.
//
// Usage:
//
//	# See your sessions and their ids first:
//	go run ./cmd/wakefix -db ./data.db -list
//
//	# Inject wake markers (RFC3339 timestamps, repeat -at as needed):
//	go run ./cmd/wakefix -db ./data.db -session 1 \
//	    -at 2026-05-26T07:00:00+02:00 -at 2026-05-27T07:15:00+02:00
//
//	# Preview without writing:
//	go run ./cmd/wakefix -db ./data.db -session 1 -at 2026-05-26T07:00:00+02:00 -dry-run
//
// It only ever inserts events; it never deletes or rewrites existing ones, and
// each insert runs in its own transaction. Back up the DB file first anyway.
package main

import (
	"errors"
	"flag"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/liviro/boob-o-clock/internal/domain"
	"github.com/liviro/boob-o-clock/internal/store"
)

// stringSlice collects a repeatable string flag (-at ... -at ...).
type stringSlice []string

func (s *stringSlice) String() string     { return fmt.Sprintf("%v", []string(*s)) }
func (s *stringSlice) Set(v string) error { *s = append(*s, v); return nil }

func main() {
	dbPath := flag.String("db", "", "path to SQLite database (required)")
	list := flag.Bool("list", false, "list sessions and exit")
	sessionID := flag.Int64("session", 0, "session id to repair")
	dryRun := flag.Bool("dry-run", false, "show what would change without writing")
	var ats stringSlice
	flag.Var(&ats, "at", "RFC3339 time to inject a wake marker (repeatable)")
	flag.Parse()

	if *dbPath == "" {
		flag.Usage()
		log.Fatal("\n-db is required")
	}

	s, err := store.New(*dbPath)
	if err != nil {
		log.Fatalf("open db: %v", err)
	}
	defer s.Close()

	if *list {
		if err := listSessions(s); err != nil {
			log.Fatalf("list: %v", err)
		}
		return
	}

	if *sessionID == 0 || len(ats) == 0 {
		flag.Usage()
		log.Fatal("\nprovide -session and at least one -at, or use -list")
	}

	if err := repair(s, *sessionID, ats, *dryRun); err != nil {
		log.Fatalf("repair: %v", err)
	}
}

func listSessions(s *store.Store) error {
	from := time.Time{}
	to := time.Date(3000, 1, 1, 0, 0, 0, 0, time.UTC)
	sessions, err := s.ListSessions(from, to, "")
	if err != nil {
		return err
	}
	if len(sessions) == 0 {
		fmt.Println("No sessions found.")
		return nil
	}
	fmt.Printf("%-5s %-6s %-25s %-25s %7s\n", "ID", "KIND", "STARTED", "ENDED", "EVENTS")
	for _, sess := range sessions {
		_, events, err := s.GetSession(sess.ID)
		if err != nil {
			return fmt.Errorf("session %d: %w", sess.ID, err)
		}
		ended := "(open)"
		if sess.EndedAt != nil {
			ended = sess.EndedAt.Local().Format(time.RFC3339)
		}
		fmt.Printf("%-5d %-6s %-25s %-25s %7d\n",
			sess.ID, sess.Kind, sess.StartedAt.Local().Format(time.RFC3339), ended, len(events))
	}
	return nil
}

func repair(s *store.Store, sessionID int64, ats stringSlice, dryRun bool) error {
	// Parse all timestamps up front so a typo fails before any write.
	times := make([]time.Time, 0, len(ats))
	for _, a := range ats {
		t, err := time.Parse(time.RFC3339, a)
		if err != nil {
			return fmt.Errorf("bad -at %q: use RFC3339 like 2026-05-26T07:00:00+02:00", a)
		}
		times = append(times, t)
	}
	// Ascending so each insert's seq math is computed against prior inserts.
	sort.Slice(times, func(i, j int) bool { return times[i].Before(times[j]) })

	if dryRun {
		fmt.Println("DRY RUN — no changes will be written.")
	}

	for _, t := range times {
		// Reload each time: a prior insert shifted seqs and added an event.
		session, events, err := s.GetSession(sessionID)
		if err != nil {
			return fmt.Errorf("load session %d: %w", sessionID, err)
		}
		if session == nil {
			return fmt.Errorf("session %d not found", sessionID)
		}

		marker, err := domain.PlanWakeMarker(*session, events, t)
		if err != nil {
			var aae *domain.AlreadyAwakeError
			if errors.As(err, &aae) {
				fmt.Printf("skip  %s — %v (already a valid split point)\n", t.Local().Format(time.RFC3339), err)
				continue
			}
			return fmt.Errorf("plan %s: %w", t.Local().Format(time.RFC3339), err)
		}

		if dryRun {
			fmt.Printf("would insert baby_woke -> %s at %s (seq %d, after %s)\n",
				marker.ToState, t.Local().Format(time.RFC3339), marker.Seq, marker.FromState)
			continue
		}
		if err := s.InsertEventAtSeq(&marker); err != nil {
			return fmt.Errorf("insert at %s: %w", t.Local().Format(time.RFC3339), err)
		}
		fmt.Printf("ok    inserted baby_woke -> %s at %s (seq %d)\n",
			marker.ToState, t.Local().Format(time.RFC3339), marker.Seq)
	}

	if !dryRun {
		fmt.Println("\nDone. Open History, find this session, and use \"Split this session\" at each marked time.")
	}
	return nil
}
