package reports

import (
	"time"

	"github.com/liviro/boob-o-clock/internal/domain"
)

// SessionMeta is the JSON shape for a session on a cycle response.
type SessionMeta struct {
	ID                int64              `json:"id"`
	Kind              domain.SessionKind `json:"kind"`
	StartedAt         time.Time          `json:"startedAt"`
	EndedAt           *time.Time         `json:"endedAt,omitempty"`
	FerberEnabled     bool               `json:"ferberEnabled,omitempty"`
	FerberNightNumber *int               `json:"ferberNightNumber,omitempty"`
	ChairEnabled      bool               `json:"chairEnabled,omitempty"`
}

// DaySegment is one contiguous span of the day, classified as awake or nap.
// The final awake span, if any, extends into the night until the first
// night-sleep event (see buildDaySegments).
type DaySegment struct {
	Kind     string        `json:"kind"` // "awake" or "nap"
	Duration time.Duration `json:"duration"`
}

type DayStats struct {
	DayDuration      time.Duration   `json:"dayDuration"`
	NapCount         int             `json:"napCount"`
	TotalNapTime     time.Duration   `json:"totalNapTime"`
	DayFeedCount     int             `json:"dayFeedCount"`
	DayTotalFeedTime time.Duration   `json:"dayTotalFeedTime"`
	WakeWindows      []time.Duration `json:"wakeWindows"`
	LastWakeWindow   *time.Duration  `json:"lastWakeWindow"`
	DaySegments      []DaySegment    `json:"daySegments"`
}

// Either half may be nil: day=nil for orphan historical cycles, night=nil
// for in-progress today.
type CycleStats struct {
	Day   *DayStats   `json:"day"`
	Night *NightStats `json:"night"`
}

type CycleEvent struct {
	Action    string            `json:"action"`
	FromState string            `json:"fromState"`
	ToState   string            `json:"toState"`
	Metadata  map[string]string `json:"metadata,omitempty"`
	Timestamp time.Time         `json:"timestamp"`
}

// CycleSummary is one row in the /api/cycles list response. Events are
// always included — a cycle's identity is its events, and the server already
// loads them to compute stats.
type CycleSummary struct {
	Day    *SessionMeta `json:"day"`
	Night  *SessionMeta `json:"night"`
	Events []CycleEvent `json:"events"` // day+night, timestamp-ordered
	Stats  CycleStats   `json:"stats"`
	Avg    *CycleStats  `json:"avg"`
}

// ComputeCycleStats assembles stats for a cycle. Either session can be nil.
// Night stats use the existing ComputeStats; day stats use ComputeDayStats.
func ComputeCycleStats(day, night *domain.Session, dayEvents, nightEvents []domain.Event) CycleStats {
	var cs CycleStats
	if night != nil {
		ns, _ := ComputeStats(nightEvents, night)
		cs.Night = &ns
	}
	if day != nil {
		ds := ComputeDayStats(day, dayEvents, night, nightEvents)
		cs.Day = &ds
	}
	return cs
}

// nightSleepStates are the night-subgraph sleep states that end the final
// cross-cycle wake window. A baby going to sleep at bedtime closes the open
// day-side wake window when they first enter one of these.
var nightSleepStates = map[domain.State]bool{
	domain.SleepingCrib:     true,
	domain.SleepingOnMe:     true,
	domain.SleepingStroller: true,
}

// ComputeDayStats produces per-cycle day metrics.
//
// Two passes: one over day events for feed stats (count, timestamps, total
// feed time), one for the awake/nap rhythm via buildDaySegments. Nap stats
// and wake windows are derived from the segments so they stay in sync with
// DaySegments by construction.
func ComputeDayStats(day *domain.Session, dayEvents []domain.Event, night *domain.Session, nightEvents []domain.Event) DayStats {
	var stats DayStats
	stats.DayDuration = dayEndTime(day).Sub(day.StartedAt)

	// Feed stats: count start_feed events and sum time spent inside
	// DayFeeding (start_feed → dislatch_*).
	var feedStart *time.Time
	for _, e := range dayEvents {
		if e.Action == domain.StartFeed {
			stats.DayFeedCount++
			if e.ToState == domain.DayFeeding {
				t := e.Timestamp
				feedStart = &t
			}
		} else if e.FromState == domain.DayFeeding && e.ToState != domain.DayFeeding && feedStart != nil {
			stats.DayTotalFeedTime += e.Timestamp.Sub(*feedStart)
			feedStart = nil
		}
	}
	if feedStart != nil {
		stats.DayTotalFeedTime += dayEndTime(day).Sub(*feedStart)
	}

	// Awake/nap rhythm — last awake span extends into the night until first
	// night-sleep (see buildDaySegments).
	stats.DaySegments = buildDaySegments(day, dayEvents, night, nightEvents)

	// Derive nap stats and wake windows from segments — no second events walk.
	for _, seg := range stats.DaySegments {
		switch seg.Kind {
		case "nap":
			stats.NapCount++
			stats.TotalNapTime += seg.Duration
		case "awake":
			stats.WakeWindows = append(stats.WakeWindows, seg.Duration)
		}
	}
	if len(stats.WakeWindows) > 0 {
		last := stats.WakeWindows[len(stats.WakeWindows)-1]
		stats.LastWakeWindow = &last
	}

	return stats
}

// dayEndTime returns day.EndedAt if closed, else time.Now() (for in-progress).
func dayEndTime(day *domain.Session) time.Time {
	if day != nil && day.EndedAt != nil {
		return *day.EndedAt
	}
	return time.Now()
}

// buildDaySegments produces the day's alternating awake/nap rhythm.
// Precondition: day sessions begin in DayAwake (so the first segment is
// always "awake"). The final awake segment — when present — extends into
// the following night until the first night-sleep event, so the "last
// wake window before bedtime" spans the chain boundary naturally.
func buildDaySegments(day *domain.Session, dayEvents []domain.Event, night *domain.Session, nightEvents []domain.Event) []DaySegment {
	if day == nil {
		return nil
	}
	var segments []DaySegment
	isNap := false
	spanStart := day.StartedAt

	for _, e := range dayEvents {
		enteringNap := e.ToState == domain.DaySleeping && e.FromState != domain.DaySleeping
		leavingNap := e.FromState == domain.DaySleeping && e.ToState != domain.DaySleeping
		if enteringNap {
			segments = append(segments, DaySegment{Kind: "awake", Duration: e.Timestamp.Sub(spanStart)})
			isNap = true
			spanStart = e.Timestamp
		} else if leavingNap {
			segments = append(segments, DaySegment{Kind: "nap", Duration: e.Timestamp.Sub(spanStart)})
			isNap = false
			spanStart = e.Timestamp
		}
	}

	// Close the final open segment.
	closedAt := finalSegmentEnd(day, night, nightEvents, isNap)
	kind := "awake"
	if isNap {
		kind = "nap"
	}
	segments = append(segments, DaySegment{Kind: kind, Duration: closedAt.Sub(spanStart)})

	return segments
}

// A final awake span extends into the night until the first night-sleep
// event; a final nap does not cross the chain boundary.
func finalSegmentEnd(day *domain.Session, night *domain.Session, nightEvents []domain.Event, isNap bool) time.Time {
	if isNap || night == nil {
		return dayEndTime(day)
	}
	for _, e := range nightEvents {
		if nightSleepStates[e.ToState] {
			return e.Timestamp
		}
	}
	if night.EndedAt != nil {
		return *night.EndedAt
	}
	return time.Now()
}

// AttachMovingAverages sets each CycleSummary.Avg to a trailing window-wide
// mean of preceding cycles (nil until i+1 ≥ window).
func AttachMovingAverages(summaries []CycleSummary, window int) {
	for i := range summaries {
		if i+1 < window {
			continue
		}
		avg := averageCycles(summaries[i+1-window : i+1])
		summaries[i].Avg = &avg
	}
}

// Means numeric fields across non-nil halves — a day half with only 2 of 3
// cycles contributing is averaged across 2, not 3 (nil halves skipped).
func averageCycles(cycles []CycleSummary) CycleStats {
	var avg CycleStats
	var dayCount, nightCount int
	var dayAcc DayStats
	var nightAcc NightStats

	for _, c := range cycles {
		if c.Stats.Day != nil {
			dayCount++
			dayAcc.DayDuration += c.Stats.Day.DayDuration
			dayAcc.NapCount += c.Stats.Day.NapCount
			dayAcc.TotalNapTime += c.Stats.Day.TotalNapTime
			dayAcc.DayFeedCount += c.Stats.Day.DayFeedCount
			dayAcc.DayTotalFeedTime += c.Stats.Day.DayTotalFeedTime
		}
		if c.Stats.Night != nil {
			nightCount++
			nightAcc.NightDuration += c.Stats.Night.NightDuration
			nightAcc.TotalSleepTime += c.Stats.Night.TotalSleepTime
			nightAcc.TotalFeedTime += c.Stats.Night.TotalFeedTime
			nightAcc.FeedTimeLeft += c.Stats.Night.FeedTimeLeft
			nightAcc.FeedTimeRight += c.Stats.Night.FeedTimeRight
			nightAcc.LongestSleepBlock += c.Stats.Night.LongestSleepBlock
			nightAcc.FeedCount += c.Stats.Night.FeedCount
			nightAcc.WakeCount += c.Stats.Night.WakeCount
		}
	}

	if dayCount > 0 {
		d := dayAcc
		d.DayDuration = dayAcc.DayDuration / time.Duration(dayCount)
		d.NapCount = dayAcc.NapCount / dayCount
		d.TotalNapTime = dayAcc.TotalNapTime / time.Duration(dayCount)
		d.DayFeedCount = dayAcc.DayFeedCount / dayCount
		d.DayTotalFeedTime = dayAcc.DayTotalFeedTime / time.Duration(dayCount)
		d.WakeWindows = nil // not meaningful as a per-window average
		d.LastWakeWindow = nil
		avg.Day = &d
	}
	if nightCount > 0 {
		n := nightAcc
		n.NightDuration = nightAcc.NightDuration / time.Duration(nightCount)
		n.TotalSleepTime = nightAcc.TotalSleepTime / time.Duration(nightCount)
		n.TotalFeedTime = nightAcc.TotalFeedTime / time.Duration(nightCount)
		n.FeedTimeLeft = nightAcc.FeedTimeLeft / time.Duration(nightCount)
		n.FeedTimeRight = nightAcc.FeedTimeRight / time.Duration(nightCount)
		n.LongestSleepBlock = nightAcc.LongestSleepBlock / time.Duration(nightCount)
		n.FeedCount = nightAcc.FeedCount / nightCount
		n.WakeCount = nightAcc.WakeCount / nightCount
		n.SleepBlocks = nil
		n.FeedTimes = nil
		n.RealBedtime = nil
		n.Ferber = nil
		avg.Night = &n
	}

	return avg
}
