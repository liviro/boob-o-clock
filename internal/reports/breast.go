package reports

import "github.com/liviro/boob-o-clock/internal/domain"

// LastBreastUsed returns the breast side from the most recent feed event.
// Returns empty string if no feed events exist.
func LastBreastUsed(events []domain.Event) string {
	for i := len(events) - 1; i >= 0; i-- {
		if b, ok := events[i].Metadata["breast"]; ok {
			return b
		}
	}
	return ""
}

// SuggestedBreast returns the opposite of the last breast used.
func SuggestedBreast(last string) string {
	switch last {
	case "L":
		return "R"
	case "R":
		return "L"
	default:
		return ""
	}
}
