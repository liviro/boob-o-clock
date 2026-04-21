package reports

import (
	"testing"
	"time"
)

func makeNightStats(longestSleep, totalSleep, totalFeed time.Duration, wakes, feeds int) NightStats {
	return NightStats{
		NightDuration:     8 * time.Hour,
		TotalSleepTime:    totalSleep,
		TotalFeedTime:     totalFeed,
		TotalAwakeTime:    8*time.Hour - totalSleep - totalFeed,
		FeedCount:         feeds,
		WakeCount:         wakes,
		LongestSleepBlock: longestSleep,
	}
}

func TestComputeTrends(t *testing.T) {
	baseDate := time.Date(2026, 3, 20, 21, 0, 0, 0, time.Local)

	points := []NightDataPoint{
		{Date: baseDate, Stats: makeNightStats(2*time.Hour, 5*time.Hour, 1*time.Hour, 4, 4)},
		{Date: baseDate.Add(24 * time.Hour), Stats: makeNightStats(3*time.Hour, 6*time.Hour, 45*time.Minute, 3, 3)},
		{Date: baseDate.Add(48 * time.Hour), Stats: makeNightStats(4*time.Hour, 7*time.Hour, 30*time.Minute, 2, 2)},
		{Date: baseDate.Add(72 * time.Hour), Stats: makeNightStats(3*time.Hour, 6*time.Hour, 40*time.Minute, 3, 3)},
		{Date: baseDate.Add(96 * time.Hour), Stats: makeNightStats(5*time.Hour, 7*time.Hour+30*time.Minute, 30*time.Minute, 1, 2)},
	}

	trends := ComputeTrends(points, 3)

	if len(trends) != 5 {
		t.Fatalf("got %d trend points, want 5", len(trends))
	}

	// First two points have no moving average (window=3 needs at least 3)
	if trends[0].AvgLongestSleep != nil {
		t.Error("first point should have nil moving average")
	}
	if trends[1].AvgLongestSleep != nil {
		t.Error("second point should have nil moving average")
	}

	// Third point: avg of first 3 longest sleeps = (2+3+4)/3 = 3h
	if trends[2].AvgLongestSleep == nil {
		t.Fatal("third point should have moving average")
	}
	expectedAvg := 3 * time.Hour
	if *trends[2].AvgLongestSleep != expectedAvg {
		t.Errorf("avg longest sleep at point 3 = %v, want %v", *trends[2].AvgLongestSleep, expectedAvg)
	}

	// Fifth point: avg of points 3,4,5 = (4+3+5)/3 = 4h
	expectedAvg5 := 4 * time.Hour
	if *trends[4].AvgLongestSleep != expectedAvg5 {
		t.Errorf("avg longest sleep at point 5 = %v, want %v", *trends[4].AvgLongestSleep, expectedAvg5)
	}

	// Raw values should be preserved
	if trends[0].LongestSleep != 2*time.Hour {
		t.Errorf("raw longest sleep at point 1 = %v, want 2h", trends[0].LongestSleep)
	}
}

func TestComputeTrendsEmpty(t *testing.T) {
	trends := ComputeTrends(nil, 3)
	if len(trends) != 0 {
		t.Errorf("got %d trends for empty input, want 0", len(trends))
	}
}

func TestComputeTrendsSingleNight(t *testing.T) {
	points := []NightDataPoint{
		{Date: time.Now(), Stats: makeNightStats(3*time.Hour, 6*time.Hour, 30*time.Minute, 2, 2)},
	}

	trends := ComputeTrends(points, 3)
	if len(trends) != 1 {
		t.Fatalf("got %d trends, want 1", len(trends))
	}
	if trends[0].AvgLongestSleep != nil {
		t.Error("single point should have no moving average")
	}
	if trends[0].LongestSleep != 3*time.Hour {
		t.Errorf("raw value wrong: %v", trends[0].LongestSleep)
	}
}

func TestComputeTrends_FerberFields(t *testing.T) {
	n1 := NightDataPoint{
		Date: time.Date(2026, 4, 18, 21, 0, 0, 0, time.UTC),
		Stats: NightStats{
			Ferber: &FerberStats{
				CryTime:         10 * time.Minute,
				CheckIns:        3,
				AvgTimeToSettle: 20 * time.Minute,
			},
		},
	}
	n2 := NightDataPoint{
		Date: time.Date(2026, 4, 19, 21, 0, 0, 0, time.UTC),
		Stats: NightStats{
			Ferber: &FerberStats{
				CryTime:         6 * time.Minute,
				CheckIns:        2,
				AvgTimeToSettle: 15 * time.Minute,
			},
		},
	}
	trends := ComputeTrends([]NightDataPoint{n1, n2}, 2)

	if trends[0].FerberCryTime == nil || *trends[0].FerberCryTime != 10*time.Minute {
		t.Errorf("night 1 FerberCryTime = %v, want 10m", trends[0].FerberCryTime)
	}
	if trends[1].FerberCheckIns == nil || *trends[1].FerberCheckIns != 2 {
		t.Errorf("night 2 FerberCheckIns = %v, want 2", trends[1].FerberCheckIns)
	}
	// Nights without Ferber should have nil fields.
	nonFerber := NightDataPoint{Date: time.Now(), Stats: NightStats{}}
	trends = ComputeTrends([]NightDataPoint{nonFerber}, 1)
	if trends[0].FerberCryTime != nil {
		t.Error("non-Ferber night should have nil FerberCryTime")
	}
}

func TestComputeTrendsWakeCountAverage(t *testing.T) {
	baseDate := time.Date(2026, 3, 20, 21, 0, 0, 0, time.Local)

	points := []NightDataPoint{
		{Date: baseDate, Stats: makeNightStats(2*time.Hour, 5*time.Hour, time.Hour, 6, 4)},
		{Date: baseDate.Add(24 * time.Hour), Stats: makeNightStats(2*time.Hour, 5*time.Hour, time.Hour, 3, 3)},
		{Date: baseDate.Add(48 * time.Hour), Stats: makeNightStats(2*time.Hour, 5*time.Hour, time.Hour, 3, 2)},
	}

	trends := ComputeTrends(points, 3)

	// Avg wakes: (6+3+3)/3 = 4.0
	if trends[2].AvgWakeCount == nil {
		t.Fatal("expected avg wake count")
	}
	if *trends[2].AvgWakeCount != 4.0 {
		t.Errorf("avg wake count = %v, want 4.0", *trends[2].AvgWakeCount)
	}
}
