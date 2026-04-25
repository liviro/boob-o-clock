package reports

import (
	"slices"
	"testing"

	"github.com/liviro/boob-o-clock/internal/domain"
)

func TestSuggestChair(t *testing.T) {
	tests := []struct {
		name string
		last *domain.Session
		want bool
	}{
		{"nil last session", nil, false},
		{"last was plain night", &domain.Session{Kind: domain.SessionKindNight}, false},
		{"last was Ferber night", &domain.Session{Kind: domain.SessionKindNight, FerberEnabled: true}, false},
		{"last was day", &domain.Session{Kind: domain.SessionKindDay}, false},
		{"last was Chair night", &domain.Session{Kind: domain.SessionKindNight, ChairEnabled: true}, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := SuggestChair(tt.last); got != tt.want {
				t.Errorf("SuggestChair() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestSelectActionsForNight_Chair(t *testing.T) {
	awakeActions := domain.ValidActions(domain.Awake)

	t.Run("chair night drops PutDownAwake, keeps SitChair", func(t *testing.T) {
		got := SelectActionsForNight(awakeActions, false, true)
		if slices.Contains(got,domain.PutDownAwake) {
			t.Error("chair night should drop put_down_awake (sit_chair takes its slot)")
		}
		if !slices.Contains(got,domain.SitChair) {
			t.Error("chair night should keep sit_chair")
		}
		if slices.Contains(got,domain.PutDownAwakeFerber) {
			t.Error("chair night should drop put_down_awake_ferber")
		}
	})

	t.Run("plain night drops SitChair and ExitChair", func(t *testing.T) {
		got := SelectActionsForNight(awakeActions, false, false)
		if slices.Contains(got,domain.SitChair) {
			t.Error("plain night should drop sit_chair")
		}
		if !slices.Contains(got,domain.PutDownAwake) {
			t.Error("plain night should keep put_down_awake")
		}
	})

	t.Run("ferber night drops SitChair and ExitChair", func(t *testing.T) {
		got := SelectActionsForNight(awakeActions, true, false)
		if slices.Contains(got,domain.SitChair) {
			t.Error("ferber night should drop sit_chair")
		}
		if !slices.Contains(got,domain.PutDownAwakeFerber) {
			t.Error("ferber night should keep put_down_awake_ferber")
		}
	})

	t.Run("Chair state actions filter correctly", func(t *testing.T) {
		// On a chair night, the Chair state itself exposes Settled and ExitChair.
		chairActions := domain.ValidActions(domain.Chair)
		got := SelectActionsForNight(chairActions, false, true)
		if !slices.Contains(got,domain.Settled) {
			t.Error("chair-state on chair night should keep settled")
		}
		if !slices.Contains(got,domain.ExitChair) {
			t.Error("chair-state on chair night should keep exit_chair")
		}
	})
}

