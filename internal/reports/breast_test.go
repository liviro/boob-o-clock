package reports

import (
	"testing"
	"time"

	"github.com/liviro/boob-o-clock/internal/domain"
)

func TestLastBreastUsed(t *testing.T) {
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, t0(), nil),
		mkEvent(2, domain.Awake, domain.StartFeed, domain.Feeding, t0(), breast("L")),
		mkEvent(3, domain.Feeding, domain.SwitchBreast, domain.Feeding, t0().Add(10*time.Minute), breast("R")),
		mkEvent(4, domain.Feeding, domain.DislatchAwake, domain.Awake, t0().Add(20*time.Minute), nil),
	}

	got := LastBreastUsed(events)
	if got != "R" {
		t.Errorf("LastBreastUsed = %q, want R", got)
	}
}

func TestLastBreastUsedNoFeeds(t *testing.T) {
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, t0(), nil),
	}

	got := LastBreastUsed(events)
	if got != "" {
		t.Errorf("LastBreastUsed = %q, want empty", got)
	}
}

func TestLastBreastUsedEmpty(t *testing.T) {
	got := LastBreastUsed(nil)
	if got != "" {
		t.Errorf("LastBreastUsed = %q, want empty", got)
	}
}

func TestSuggestedBreast(t *testing.T) {
	tests := []struct {
		last string
		want string
	}{
		{"L", "R"},
		{"R", "L"},
		{"", ""},
	}

	for _, tt := range tests {
		got := SuggestedBreast(tt.last)
		if got != tt.want {
			t.Errorf("SuggestedBreast(%q) = %q, want %q", tt.last, got, tt.want)
		}
	}
}
