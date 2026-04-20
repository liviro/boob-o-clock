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

// ComputeFerberStats returns Ferber metrics derived from the event log.
// Events outside LEARNING/CHECK_IN are irrelevant to Ferber stats and ignored.
func ComputeFerberStats(events []domain.Event, nightEnd time.Time) FerberStats {
	// Implementation in Task 3.2.
	return FerberStats{}
}
