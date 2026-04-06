package reports

import (
	"time"

	"github.com/liviro/boob-o-clock/internal/domain"
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
	LongestSleepBlock time.Duration   `json:"longestSleepBlock"`
	SleepBlocks       []time.Duration `json:"sleepBlocks"`
	FeedTimes         []time.Time    `json:"feedTimes"`
}

// sleepStates are states where the baby is sleeping or settling toward sleep.
var sleepStates = map[domain.State]bool{
	domain.SleepingOnMe:     true,
	domain.SleepingCrib:     true,
	domain.SleepingStroller: true,
	domain.Resettling:       true,
	domain.Strolling:        true,
	domain.SelfSoothing:     true,
}

// independentSleepStates are states where the baby sleeps independently (not on
// a person). Used to distinguish "real sleep" from contact napping.
var independentSleepStates = map[domain.State]bool{
	domain.SleepingCrib:     true,
	domain.SleepingStroller: true,
}

// contiguousSleepStates are states that form an unbroken sleep block.
// A sleep block is broken when the baby transitions to AWAKE, FEEDING, or POOP.
var contiguousSleepStates = map[domain.State]bool{
	domain.SleepingOnMe:     true,
	domain.SleepingCrib:     true,
	domain.SleepingStroller: true,
	domain.Resettling:       true,
	domain.Strolling:        true,
	domain.SelfSoothing:     true,
	domain.Transferring:     true, // instantaneous, doesn't break a sleep block
}

// BuildTimeline converts events into a sequence of state periods with durations.
// Instantaneous states (TRANSFERRING) are excluded since their duration is 0.
// nightEnd closes the final open state so in-progress nights include the current period.
func BuildTimeline(events []domain.Event, nightEnd time.Time) []TimelineEntry {
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
	}

	if currentState != "" && currentState != domain.Transferring && nightEnd.After(currentStart) {
		entries = append(entries, TimelineEntry{
			State:    currentState,
			Start:    currentStart,
			Duration: nightEnd.Sub(currentStart),
			Metadata: currentMeta,
		})
	}

	return entries
}

// ComputeStats calculates summary statistics and returns the timeline it built.
func ComputeStats(events []domain.Event, nightStart, nightEnd time.Time) (NightStats, []TimelineEntry) {
	if len(events) == 0 {
		return NightStats{}, nil
	}

	stats := NightStats{
		NightDuration: nightEnd.Sub(nightStart),
	}

	timeline := BuildTimeline(events, nightEnd)

	// Only count feeds after the first real sleep (crib or stroller, not "on me").
	// Pre-sleep feeds at night start are excluded — they're part of putting the
	// baby down, not mid-night wakes.
	seenRealSleep := false
	inFeedSession := false
	for _, evt := range events {
		if independentSleepStates[evt.ToState] {
			seenRealSleep = true
		}

		switch {
		case evt.Action == domain.StartFeed:
			if !inFeedSession && seenRealSleep {
				stats.FeedCount++
				stats.FeedTimes = append(stats.FeedTimes, evt.Timestamp)
			}
			inFeedSession = true
		case evt.ToState == domain.Awake:
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

	// Sleep blocks: contiguous sequences of sleep-ish states.
	// Blocks comprised solely of on-me sleep (no crib/stroller) are excluded —
	// e.g. baby fell asleep at breast but woke on transfer.
	var currentBlock time.Duration
	hasIndependentSleep := false
	flushBlock := func() {
		if currentBlock > 0 && hasIndependentSleep {
			stats.SleepBlocks = append(stats.SleepBlocks, currentBlock)
			if currentBlock > stats.LongestSleepBlock {
				stats.LongestSleepBlock = currentBlock
			}
		}
		currentBlock = 0
		hasIndependentSleep = false
	}
	for _, entry := range timeline {
		if contiguousSleepStates[entry.State] {
			currentBlock += entry.Duration
			if !hasIndependentSleep && independentSleepStates[entry.State] {
				hasIndependentSleep = true
			}
		} else {
			flushBlock()
		}
	}
	flushBlock()

	return stats, timeline
}
