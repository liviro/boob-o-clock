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

// SelectFerberVariants returns the subset of actions appropriate for the night's
// Ferber state: on Ferber nights, drop the plain variants and keep the _ferber
// aliases; on normal nights, drop the _ferber aliases and keep the plain ones.
// Clients render exactly what they receive without branching on ferber state.
func SelectFerberVariants(actions []domain.Action, ferberEnabled bool) []domain.Action {
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

// ComputeFerberStats returns Ferber metrics derived from the event log.
// nightEnd closes the final open state so in-progress sessions still contribute
// their elapsed mood time.
func ComputeFerberStats(events []domain.Event, nightEnd time.Time) FerberStats {
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
