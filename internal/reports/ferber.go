package reports

import (
	"time"

	"github.com/liviro/boob-o-clock/internal/domain"
)

// FerberStats summarizes Ferber-specific metrics for a single night.
// Only populated when the night has ferber_enabled = true.
type FerberStats struct {
	Sessions          int           `json:"sessions"`
	CheckIns          int           `json:"checkIns"`
	CryTime           time.Duration `json:"cryTime"`
	FussTime          time.Duration `json:"fussTime"`
	QuietTime         time.Duration `json:"quietTime"`
	SessionsAbandoned int           `json:"sessionsAbandoned"`
	AvgTimeToSettle   time.Duration `json:"avgTimeToSettle"`
}

// SuggestFerberNight returns the suggested Ferber night number for the next
// night (last + 1) based on the most recent night, or nil when the last night
// was not Ferber (or there is no history). Used to seed the Start Night form.
func SuggestFerberNight(last *domain.Night) *int {
	if last == nil || !last.FerberEnabled || last.FerberNightNumber == nil {
		return nil
	}
	n := *last.FerberNightNumber + 1
	return &n
}

// SelectActionsForNight returns the actions appropriate for the night's Ferber
// state: on Ferber nights, drop the plain variants and keep the _ferber
// aliases; on normal nights, drop the _ferber aliases and keep the plain ones.
// Clients render exactly what they receive without branching on ferber state.
func SelectActionsForNight(actions []domain.Action, ferberEnabled bool) []domain.Action {
	drop := map[domain.Action]bool{}
	if ferberEnabled {
		drop[domain.PutDownAwake] = true
		drop[domain.BabyStirred] = true
	} else {
		drop[domain.PutDownAwakeFerber] = true
		drop[domain.BabyStirredFerber] = true
	}
	out := make([]domain.Action, 0, len(actions))
	for _, a := range actions {
		if drop[a] {
			continue
		}
		out = append(out, a)
	}
	return out
}

// ferberEntryActions are the actions that begin a Ferber session.
var ferberEntryActions = map[domain.Action]bool{
	domain.PutDownAwakeFerber: true,
	domain.BabyStirredFerber:  true,
}

// ferberIntervals is the classic Ferber graduated-extinction table, in minutes.
// Rows are 1-indexed Ferber nights; columns are 1-indexed check-ins within a
// session. Nights beyond 7 use night 7's row; check-ins beyond 3 use column 3.
var ferberIntervals = [7][3]int{
	{3, 5, 10},
	{5, 10, 12},
	{10, 12, 15},
	{12, 15, 17},
	{15, 17, 20},
	{17, 20, 25},
	{20, 25, 30},
}

// IntervalFor returns the Ferber wait interval between check-ins for the given
// night and check-in number.
func IntervalFor(nightNumber, checkInNumber int) time.Duration {
	n := clamp(nightNumber, 1, 7) - 1
	c := clamp(checkInNumber, 1, 3) - 1
	return time.Duration(ferberIntervals[n][c]) * time.Minute
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

// FerberSession captures the state of an in-progress Ferber session.
// Returned only when the caller is in Learning or CheckIn; nil otherwise.
type FerberSession struct {
	CheckIns     int
	SessionStart time.Time
	// CheckInAvailableAt is populated only when state is Learning: the
	// absolute timestamp at which the next check-in becomes available
	// (sessionStart-or-last-EndCheckIn + graduated interval). Nil during
	// CheckIn, where the current check-in is still in progress.
	CheckInAvailableAt *time.Time
	Mood               string
}

// CurrentFerberSession derives the current Ferber session from the event log.
// Returns nil when state is not Learning/CheckIn or when no Ferber session
// entry is found.
func CurrentFerberSession(state domain.State, events []domain.Event, nightNumber int) *FerberSession {
	if state != domain.Learning && state != domain.CheckIn {
		return nil
	}
	sessionIdx := -1
	for i := len(events) - 1; i >= 0; i-- {
		if ferberEntryActions[events[i].Action] {
			sessionIdx = i
			break
		}
	}
	if sessionIdx == -1 {
		return nil
	}
	session := &FerberSession{
		SessionStart: events[sessionIdx].Timestamp,
		Mood:         events[sessionIdx].Metadata["mood"],
	}
	// intervalBase is sessionStart or the most recent EndCheckIn timestamp —
	// the point from which the next check-in's countdown runs. Kept local
	// because it's only meaningful when combined with the graduated interval.
	intervalBase := events[sessionIdx].Timestamp
	for j := sessionIdx + 1; j < len(events); j++ {
		e := events[j]
		switch e.Action {
		case domain.CheckInStart:
			session.CheckIns++
		case domain.EndCheckIn:
			intervalBase = e.Timestamp
			session.Mood = e.Metadata["mood"]
		case domain.MoodChange:
			session.Mood = e.Metadata["mood"]
		}
	}
	if state == domain.Learning {
		avail := intervalBase.Add(IntervalFor(nightNumber, session.CheckIns+1))
		session.CheckInAvailableAt = &avail
	}
	return session
}

// computeFerberStats returns Ferber metrics derived from the event log.
// nightEnd closes the final open state so in-progress sessions still contribute
// their elapsed mood time.
func computeFerberStats(events []domain.Event, nightEnd time.Time) FerberStats {
	var stats FerberStats
	var settleDurations []time.Duration

	// Walk the event log; track each time we enter LEARNING, the current mood,
	// and close out mood spans on every Learning/CheckIn transition.
	i := 0
	for i < len(events) {
		// Find the next session entry.
		for i < len(events) && !ferberEntryActions[events[i].Action] {
			i++
		}
		if i >= len(events) {
			break
		}
		stats.Sessions++
		sessionStart := events[i].Timestamp
		currentMood := events[i].Metadata["mood"]
		spanStart := sessionStart
		i++

		// Walk events within this session until we leave LEARNING/CHECK_IN.
		sessionClosed := false
		for i < len(events) {
			e := events[i]
			// Any event whose FromState is LEARNING or CHECK_IN belongs to this session.
			if e.FromState != domain.Learning && e.FromState != domain.CheckIn {
				break
			}

			// Close the mood span running up to this event.
			addMood(&stats, currentMood, e.Timestamp.Sub(spanStart))

			switch e.Action {
			case domain.MoodChange:
				currentMood = e.Metadata["mood"]
				spanStart = e.Timestamp
			case domain.CheckInStart:
				stats.CheckIns++
				// CHECK_IN time is attributed to the mood we entered it from.
				// currentMood stays the same; spanStart moves to the check-in start.
				spanStart = e.Timestamp
			case domain.EndCheckIn:
				currentMood = e.Metadata["mood"]
				spanStart = e.Timestamp
			case domain.Settled:
				settleDurations = append(settleDurations, e.Timestamp.Sub(sessionStart))
				sessionClosed = true
				i++
			case domain.ExitFerber:
				stats.SessionsAbandoned++
				sessionClosed = true
				i++
			}
			if sessionClosed {
				break
			}
			i++
		}

		if !sessionClosed {
			// Open session (nightEnd closes the final mood span).
			addMood(&stats, currentMood, nightEnd.Sub(spanStart))
		}
	}

	if len(settleDurations) > 0 {
		var total time.Duration
		for _, d := range settleDurations {
			total += d
		}
		stats.AvgTimeToSettle = total / time.Duration(len(settleDurations))
	}

	return stats
}

func addMood(stats *FerberStats, mood string, d time.Duration) {
	if d <= 0 {
		return
	}
	switch mood {
	case "quiet":
		stats.QuietTime += d
	case "fussy":
		stats.FussTime += d
	case "crying":
		stats.CryTime += d
	}
}
