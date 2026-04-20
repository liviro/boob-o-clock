// Command seed inserts plausible test data into a boob-o-clock database.
// Usage: go run ./cmd/seed -db ./dev.db
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

	now := time.Now()
	nights := []nightSpec{
		// 7 nights ago — great night, 2 long blocks
		{
			start: now.Add(-7 * 24 * time.Hour).Truncate(time.Hour).Add(20*time.Hour + 30*time.Minute),
			blocks: []block{
				{feedBreast: "L", feedMins: 18, sleepOnMeMins: 5, cribMins: 210},
				{feedBreast: "R", feedMins: 12, sleepOnMeMins: 3, resettleMins: 8, cribMins: 180},
			},
		},
		// 6 nights ago — rough night, 4 short blocks (3rd one needed the stroller)
		{
			start: now.Add(-6 * 24 * time.Hour).Truncate(time.Hour).Add(21 * time.Hour),
			blocks: []block{
				{feedBreast: "L", feedMins: 20, sleepOnMeMins: 8, cribMins: 90},
				{feedBreast: "R", feedMins: 15, sleepOnMeMins: 5, cribMins: 60},
				{stroller: true, poopMins: 8, strollMins: 12, strollerMins: 45},
				{feedBreast: "R", feedMins: 12, sleepOnMeMins: 6, cribMins: 120},
			},
		},
		// 5 nights ago — decent, 3 blocks
		{
			start: now.Add(-5 * 24 * time.Hour).Truncate(time.Hour).Add(20*time.Hour + 45*time.Minute),
			blocks: []block{
				{feedBreast: "R", feedMins: 15, sleepOnMeMins: 4, cribMins: 150},
				{feedBreast: "L", feedMins: 18, sleepOnMeMins: 7, resettleMins: 5, cribMins: 120},
				{feedBreast: "R", feedMins: 10, sleepOnMeMins: 3, cribMins: 90},
			},
		},
		// 4 nights ago — unicorn night, 1 massive block
		{
			start: now.Add(-4 * 24 * time.Hour).Truncate(time.Hour).Add(19*time.Hour + 30*time.Minute),
			blocks: []block{
				{feedBreast: "L", feedMins: 22, sleepOnMeMins: 6, cribMins: 420},
			},
		},
		// 3 nights ago — average, 3 blocks with resettle
		{
			start: now.Add(-3 * 24 * time.Hour).Truncate(time.Hour).Add(20 * time.Hour),
			blocks: []block{
				{feedBreast: "R", feedMins: 16, sleepOnMeMins: 5, cribMins: 180},
				{feedBreast: "L", feedMins: 14, sleepOnMeMins: 4, resettleMins: 10, cribMins: 90},
				{feedBreast: "R", feedMins: 12, sleepOnMeMins: 3, cribMins: 150},
			},
		},
		// 2 nights ago — good night, 2 blocks
		{
			start: now.Add(-2 * 24 * time.Hour).Truncate(time.Hour).Add(21*time.Hour + 15*time.Minute),
			blocks: []block{
				{feedBreast: "L", feedMins: 20, sleepOnMeMins: 5, cribMins: 240},
				{feedBreast: "R", feedMins: 15, sleepOnMeMins: 4, resettleMins: 6, cribMins: 180},
			},
		},
		// Last night — 3 blocks, mixed
		{
			start: now.Add(-1 * 24 * time.Hour).Truncate(time.Hour).Add(20*time.Hour + 30*time.Minute),
			blocks: []block{
				{feedBreast: "R", feedMins: 17, sleepOnMeMins: 6, cribMins: 165},
				{feedBreast: "L", feedMins: 13, sleepOnMeMins: 3, cribMins: 75},
				{feedBreast: "R", feedMins: 11, sleepOnMeMins: 5, resettleMins: 8, cribMins: 120},
			},
		},
		// Tonight — in progress, baby sleeping in crib after 2 completed blocks
		{
			start:      now.Add(-3 * time.Hour),
			inProgress: true,
			blocks: []block{
				{feedBreast: "L", feedMins: 15, sleepOnMeMins: 4, cribMins: 90},
				{feedBreast: "R", feedMins: 12, sleepOnMeMins: 3, resettleMins: 5, cribMins: 45},
				{feedBreast: "L", feedMins: 10, sleepOnMeMins: 5, cribMins: 0}, // currently in crib
			},
		},
	}

	for i, ns := range nights {
		if err := seedNight(s, ns); err != nil {
			log.Fatalf("night %d: %v", i+1, err)
		}
	}

	fmt.Printf("Seeded %d nights into %s\n", len(nights), *dbPath)
}

type block struct {
	feedBreast    string
	feedMins      int
	sleepOnMeMins int
	resettleMins  int  // 0 = no resettle, transfer succeeds directly
	cribMins      int
	stroller      bool // if true: strolling → sleeping_stroller instead of feed → crib
	strollMins    int  // time spent strolling before baby falls asleep
	strollerMins  int  // time spent sleeping in stroller
	poopMins      int  // if > 0, poop happens before feed/stroller (from awake)
}

type nightSpec struct {
	start      time.Time
	blocks     []block
	inProgress bool // if true, baby is currently sleeping in crib (no EndNight)
}

func seedNight(s *store.Store, ns nightSpec) error {
	night, err := s.CreateNight(ns.start, false, 0)
	if err != nil {
		return err
	}

	cursor := ns.start
	seq := 0

	add := func(from domain.State, action domain.Action, to domain.State, meta map[string]string) {
		seq++
		evt := &domain.Event{
			NightID:   night.ID,
			FromState: from,
			Action:    action,
			ToState:   to,
			Timestamp: cursor,
			Metadata:  meta,
		}
		if err2 := s.AddEvent(evt); err2 != nil {
			err = err2
		}
	}

	breast := func(side string) map[string]string {
		return map[string]string{"breast": side}
	}

	// Start night
	add(domain.NightOff, domain.StartNight, domain.Awake, nil)

	for i, b := range ns.blocks {
		isLast := i == len(ns.blocks)-1

		if b.poopMins > 0 {
			add(domain.Awake, domain.PoopStart, domain.Poop, nil)
			cursor = cursor.Add(time.Duration(b.poopMins) * time.Minute)
			add(domain.Poop, domain.PoopDone, domain.Awake, nil)
		}

		if b.stroller {
			// Nuclear option: stroller walk
			add(domain.Awake, domain.StartStrolling, domain.Strolling, nil)
			cursor = cursor.Add(time.Duration(b.strollMins) * time.Minute)
			add(domain.Strolling, domain.FellAsleep, domain.SleepingStroller, nil)

			if ns.inProgress && isLast {
				break
			}

			cursor = cursor.Add(time.Duration(b.strollerMins) * time.Minute)
			add(domain.SleepingStroller, domain.BabyWoke, domain.Awake, nil)
			continue
		}

		// Feed
		add(domain.Awake, domain.StartFeed, domain.Feeding, breast(b.feedBreast))
		cursor = cursor.Add(time.Duration(b.feedMins) * time.Minute)

		// Dislatch asleep → sleeping on me
		add(domain.Feeding, domain.DislatchAsleep, domain.SleepingOnMe, nil)
		cursor = cursor.Add(time.Duration(b.sleepOnMeMins) * time.Minute)

		// Transfer
		add(domain.SleepingOnMe, domain.StartTransfer, domain.Transferring, nil)

		if b.resettleMins > 0 {
			add(domain.Transferring, domain.TransferNeedResettle, domain.Resettling, nil)
			cursor = cursor.Add(time.Duration(b.resettleMins) * time.Minute)
			add(domain.Resettling, domain.Settled, domain.SleepingCrib, nil)
		} else {
			add(domain.Transferring, domain.TransferSuccess, domain.SleepingCrib, nil)
		}

		// For in-progress night, leave baby sleeping in crib on the last block
		if ns.inProgress && isLast {
			break
		}

		// Sleep in crib
		cursor = cursor.Add(time.Duration(b.cribMins) * time.Minute)

		// Wake
		add(domain.SleepingCrib, domain.BabyWoke, domain.Awake, nil)
	}

	if err != nil {
		return err
	}

	if ns.inProgress {
		return nil // no EndNight
	}

	// End night
	add(domain.Awake, domain.EndNight, domain.NightOff, nil)
	if err != nil {
		return err
	}

	return s.EndNight(night.ID, cursor)
}
