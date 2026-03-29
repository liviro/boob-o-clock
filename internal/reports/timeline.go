package reports

import (
	"time"

	"github.com/polina/boob-o-clock/internal/domain"
)

// TimelineEntry represents a period spent in a single state.
type TimelineEntry struct {
	State    domain.State      `json:"state"`
	Start    time.Time         `json:"start"`
	Duration time.Duration     `json:"duration"`
	Metadata map[string]string `json:"metadata,omitempty"`
}

// NightStats summarizes a single night.
type NightStats struct {
	NightDuration     time.Duration `json:"nightDuration"`
	TotalSleepTime    time.Duration `json:"totalSleepTime"`
	TotalFeedTime     time.Duration `json:"totalFeedTime"`
	FeedTimeLeft      time.Duration `json:"feedTimeLeft"`
	FeedTimeRight     time.Duration `json:"feedTimeRight"`
	TotalAwakeTime    time.Duration `json:"totalAwakeTime"`
	FeedCount         int           `json:"feedCount"`
	WakeCount         int           `json:"wakeCount"`
	LongestSleepBlock time.Duration `json:"longestSleepBlock"`
}

// sleepStates are states where the baby is sleeping or settling toward sleep.
var sleepStates = map[domain.State]bool{
	domain.SleepingOnMe:     true,
	domain.SleepingCrib:     true,
	domain.SleepingStroller: true,
	domain.Resettling:       true,
	domain.Strolling:        true,
}

// contiguousSleepStates are states that form an unbroken sleep block.
// A sleep block is broken when the baby transitions to AWAKE, FEEDING, or POOP.
var contiguousSleepStates = map[domain.State]bool{
	domain.SleepingOnMe:     true,
	domain.SleepingCrib:     true,
	domain.SleepingStroller: true,
	domain.Resettling:       true,
	domain.Strolling:        true,
	domain.Transferring:     true, // instantaneous, doesn't break a sleep block
}

// BuildTimeline converts events into a sequence of state periods with durations.
// Instantaneous states (TRANSFERRING) are excluded since their duration is 0.
func BuildTimeline(events []domain.Event) []TimelineEntry {
	if len(events) == 0 {
		return nil
	}

	var entries []TimelineEntry
	var currentState domain.State
	var currentStart time.Time
	var currentMeta map[string]string

	for _, evt := range events {
		if currentState != "" && evt.Timestamp.After(currentStart) && currentState != domain.Transferring {
			entries = append(entries, TimelineEntry{
				State:    currentState,
				Start:    currentStart,
				Duration: evt.Timestamp.Sub(currentStart),
				Metadata: currentMeta,
			})
		}

		currentState = evt.ToState
		currentStart = evt.Timestamp
		currentMeta = evt.Metadata

		// For switch_breast, carry the metadata forward but don't start a new entry
		// (handled by the normal flow — previous feeding entry ends, new one starts)
	}

	return entries
}

// ComputeStats calculates summary statistics for a night.
func ComputeStats(events []domain.Event, nightStart, nightEnd time.Time) NightStats {
	if len(events) == 0 {
		return NightStats{}
	}

	stats := NightStats{
		NightDuration: nightEnd.Sub(nightStart),
	}

	timeline := BuildTimeline(events)

	// Track whether we're in a feed session (StartFeed started it, not SwitchBreast)
	inFeedSession := false
	for _, evt := range events {
		switch evt.Action {
		case domain.StartFeed:
			if !inFeedSession {
				stats.FeedCount++
				inFeedSession = true
			}
		case domain.DislatchAwake, domain.DislatchAsleep:
			inFeedSession = false
		}

		// Count wakes: transitions INTO awake via BabyWoke action
		if evt.Action == domain.BabyWoke {
			stats.WakeCount++
		}
	}

	// Accumulate durations from timeline
	for _, entry := range timeline {
		if sleepStates[entry.State] {
			stats.TotalSleepTime += entry.Duration
		}
		if entry.State == domain.Feeding {
			stats.TotalFeedTime += entry.Duration
			switch entry.Metadata["breast"] {
			case "L":
				stats.FeedTimeLeft += entry.Duration
			case "R":
				stats.FeedTimeRight += entry.Duration
			}
		}
		if entry.State == domain.Awake || entry.State == domain.Poop {
			stats.TotalAwakeTime += entry.Duration
		}
	}

	// Longest sleep block: contiguous sequence of sleep-ish states
	var currentBlock time.Duration
	for _, entry := range timeline {
		if contiguousSleepStates[entry.State] {
			currentBlock += entry.Duration
		} else {
			if currentBlock > stats.LongestSleepBlock {
				stats.LongestSleepBlock = currentBlock
			}
			currentBlock = 0
		}
	}
	// Check final block
	if currentBlock > stats.LongestSleepBlock {
		stats.LongestSleepBlock = currentBlock
	}

	return stats
}
