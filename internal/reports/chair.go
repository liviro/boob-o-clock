package reports

import "github.com/liviro/boob-o-clock/internal/domain"

// SuggestChair returns true if the most recent night session was a Chair night.
// Used to seed the Start Night form's chair toggle.
func SuggestChair(last *domain.Session) bool {
	return last != nil &&
		last.Kind == domain.SessionKindNight &&
		last.ChairEnabled
}
