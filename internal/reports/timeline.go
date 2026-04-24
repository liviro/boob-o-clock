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
	NightDuration     time.Duration   `json:"nightDuration"`
	TotalSleepTime    time.Duration   `json:"totalSleepTime"`
	TotalFeedTime     time.Duration   `json:"totalFeedTime"`
	FeedTimeLeft      time.Duration   `json:"feedTimeLeft"`
	FeedTimeRight     time.Duration   `json:"feedTimeRight"`
	TotalAwakeTime    time.Duration   `json:"totalAwakeTime"`
	FeedCount         int             `json:"feedCount"`
	WakeCount         int             `json:"wakeCount"`
	LongestSleepBlock time.Duration   `json:"longestSleepBlock"`
	SleepBlocks       []time.Duration `json:"sleepBlocks"`
	FeedTimes         []time.Time     `json:"feedTimes"`
	RealBedtime       *time.Time      `json:"realBedtime,omitempty"`
	Ferber            *FerberStats    `json:"ferber,omitempty"`
}

// sleepStates are states where the baby is sleeping or settling toward sleep.
// SelfSoothing is deliberately excluded — baby is in the crib but still awake.
var sleepStates = map[domain.State]bool{
	domain.SleepingOnMe:     true,
	domain.SleepingCrib:     true,
	domain.SleepingStroller: true,
	domain.Resettling:       true,
	domain.Strolling:        true,
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
	domain.Transferring:     true, // doesn't break a sleep block; duration classified retroactively based on outcome
}

// BuildTimeline converts events into a sequence of state periods with durations.
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
		if currentState != "" && evt.Timestamp.After(currentStart) {
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

	if currentState != "" && nightEnd.After(currentStart) {
		entries = append(entries, TimelineEntry{
			State:    currentState,
			Start:    currentStart,
			Duration: nightEnd.Sub(currentStart),
			Metadata: currentMeta,
		})
	}

	return entries
}

// computeBaseStats calculates non-Ferber summary statistics and returns the
// timeline it built.
func computeBaseStats(events []domain.Event, nightStart, nightEnd time.Time) (NightStats, []TimelineEntry) {
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

	// Build a lookup: for each Transferring start time, what state comes next?
	// We use events (not the timeline) so we catch zero-duration transitions that
	// are filtered out of the timeline (e.g. a failed transfer at night's end).
	transferOutcome := make(map[time.Time]domain.State)
	for j, evt := range events {
		if evt.ToState == domain.Transferring && j+1 < len(events) {
			transferOutcome[evt.Timestamp] = events[j+1].ToState
		}
	}

	// Accumulate durations from timeline
	for _, entry := range timeline {
		if sleepStates[entry.State] {
			stats.TotalSleepTime += entry.Duration
		}
		// Transfer: classify retroactively based on outcome.
		// Success (→ SleepingCrib) or stir (→ Resettling) = sleep.
		// Failure (→ anything else) = awake. In-progress (no outcome yet) = uncounted.
		if entry.State == domain.Transferring {
			if next, resolved := transferOutcome[entry.Start]; resolved {
				if next == domain.SleepingCrib || next == domain.Resettling {
					stats.TotalSleepTime += entry.Duration
				} else {
					stats.TotalAwakeTime += entry.Duration
				}
			}
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
		if entry.State == domain.Awake || entry.State == domain.Poop || entry.State == domain.SelfSoothing {
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

	stats.RealBedtime = computeRealBedtime(events)

	return stats, timeline
}

// ComputeStats returns the full per-night stat bag for a single night session,
// including Ferber stats when the night has Ferber enabled. An unended session
// closes out at now so in-progress state spans still contribute.
//
// Night-only: callers should pass a session with Kind == Night. Day sessions
// use a separate day-stats computation (not yet migrated to this file).
func ComputeStats(events []domain.Event, night *domain.Session) (NightStats, []TimelineEntry) {
	nightEnd := time.Now()
	if night.EndedAt != nil {
		nightEnd = *night.EndedAt
	}
	stats, timeline := computeBaseStats(events, night.StartedAt, nightEnd)
	if night.FerberEnabled {
		fs := computeFerberStats(events, nightEnd)
		stats.Ferber = &fs
	}
	return stats, timeline
}

// asleepSignalActions are the actions that mean "baby just fell asleep".
// The first such action inside a block-leading-to-independent-sleep is the bedtime.
var asleepSignalActions = map[domain.Action]bool{
	domain.DislatchAsleep:  true, // Feeding → SleepingOnMe
	domain.FellAsleep:      true, // Strolling → SleepingStroller
	domain.Settled:         true, // Resettling/SelfSoothing/Learning/CheckIn → SleepingCrib
	domain.TransferSuccess: true, // Transferring → SleepingCrib (fallback when no earlier signal in block)
}

// computeRealBedtime finds the timestamp when the baby first fell asleep in
// the first contiguous sleep block that reaches independent sleep (crib or
// stroller). Returns nil if no such block exists in the event log yet.
func computeRealBedtime(events []domain.Event) *time.Time {
	inBlock := false
	var firstAsleep *time.Time
	reachedIndependent := false

	finish := func() *time.Time {
		if reachedIndependent && firstAsleep != nil {
			return firstAsleep
		}
		return nil
	}

	for i := range events {
		evt := events[i]
		nowInBlock := contiguousSleepStates[evt.ToState]

		// Transition out of a block: maybe return a result; else reset and keep walking.
		if inBlock && !nowInBlock {
			if bt := finish(); bt != nil {
				return bt
			}
			firstAsleep = nil
			reachedIndependent = false
		}

		if nowInBlock {
			if independentSleepStates[evt.ToState] {
				reachedIndependent = true
			}
			if firstAsleep == nil && asleepSignalActions[evt.Action] {
				ts := evt.Timestamp
				firstAsleep = &ts
			}
		}
		inBlock = nowInBlock
	}

	// End of events: if still inside a qualifying block, return it.
	return finish()
}
