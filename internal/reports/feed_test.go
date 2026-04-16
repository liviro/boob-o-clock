package reports

import (
	"testing"
	"time"

	"github.com/liviro/boob-o-clock/internal/domain"
)

func TestLastFeedStartEmpty(t *testing.T) {
	got := LastFeedStart(nil)
	if got != nil {
		t.Errorf("LastFeedStart(nil) = %v, want nil", got)
	}
}

func TestLastFeedStartNoFeeds(t *testing.T) {
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, t0(), nil),
	}
	got := LastFeedStart(events)
	if got != nil {
		t.Errorf("LastFeedStart = %v, want nil", got)
	}
}

func TestLastFeedStartSingleFeed(t *testing.T) {
	feedAt := t0().Add(3 * time.Hour)
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, t0(), nil),
		mkEvent(2, domain.Awake, domain.StartFeed, domain.Feeding, feedAt, breast("L")),
	}
	got := LastFeedStart(events)
	if got == nil || !got.Equal(feedAt) {
		t.Errorf("LastFeedStart = %v, want %v", got, feedAt)
	}
}

func TestLastFeedStartMultipleFeeds(t *testing.T) {
	firstFeed := t0().Add(1 * time.Hour)
	secondFeed := t0().Add(4 * time.Hour)
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, t0(), nil),
		mkEvent(2, domain.Awake, domain.StartFeed, domain.Feeding, firstFeed, breast("L")),
		mkEvent(3, domain.Feeding, domain.DislatchAwake, domain.Awake, firstFeed.Add(20*time.Minute), nil),
		mkEvent(4, domain.Awake, domain.StartFeed, domain.Feeding, secondFeed, breast("R")),
	}
	got := LastFeedStart(events)
	if got == nil || !got.Equal(secondFeed) {
		t.Errorf("LastFeedStart = %v, want %v", got, secondFeed)
	}
}

func TestLastFeedStartIgnoresSwitchBreast(t *testing.T) {
	feedAt := t0().Add(1 * time.Hour)
	switchAt := feedAt.Add(15 * time.Minute)
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, t0(), nil),
		mkEvent(2, domain.Awake, domain.StartFeed, domain.Feeding, feedAt, breast("L")),
		mkEvent(3, domain.Feeding, domain.SwitchBreast, domain.Feeding, switchAt, breast("R")),
	}
	got := LastFeedStart(events)
	if got == nil || !got.Equal(feedAt) {
		t.Errorf("LastFeedStart = %v, want %v (switch_breast should not reset)", got, feedAt)
	}
}

func TestLastFeedStartAfterDislatch(t *testing.T) {
	feedAt := t0().Add(1 * time.Hour)
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, t0(), nil),
		mkEvent(2, domain.Awake, domain.StartFeed, domain.Feeding, feedAt, breast("L")),
		mkEvent(3, domain.Feeding, domain.DislatchAwake, domain.Awake, feedAt.Add(20*time.Minute), nil),
	}
	got := LastFeedStart(events)
	if got == nil || !got.Equal(feedAt) {
		t.Errorf("LastFeedStart = %v, want %v", got, feedAt)
	}
}

func TestLastFeedStartFromSleepingOnMe(t *testing.T) {
	firstFeed := t0().Add(1 * time.Hour)
	secondFeed := t0().Add(2 * time.Hour)
	events := []domain.Event{
		mkEvent(1, domain.NightOff, domain.StartNight, domain.Awake, t0(), nil),
		mkEvent(2, domain.Awake, domain.StartFeed, domain.Feeding, firstFeed, breast("L")),
		mkEvent(3, domain.Feeding, domain.DislatchAsleep, domain.SleepingOnMe, firstFeed.Add(15*time.Minute), nil),
		mkEvent(4, domain.SleepingOnMe, domain.StartFeed, domain.Feeding, secondFeed, breast("R")),
	}
	got := LastFeedStart(events)
	if got == nil || !got.Equal(secondFeed) {
		t.Errorf("LastFeedStart = %v, want %v", got, secondFeed)
	}
}
