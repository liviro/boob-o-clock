package reports

import (
	"time"

	"github.com/liviro/boob-o-clock/internal/domain"
)

// LastFeedStart returns the timestamp of the most recent start_feed event.
// Returns nil if no start_feed events exist. SwitchBreast is not a feed start.
func LastFeedStart(events []domain.Event) *time.Time {
	for i := len(events) - 1; i >= 0; i-- {
		if events[i].Action == domain.StartFeed {
			t := events[i].Timestamp
			return &t
		}
	}
	return nil
}
