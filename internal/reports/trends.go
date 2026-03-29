package reports

import "time"

// NightDataPoint pairs a night's date with its stats.
type NightDataPoint struct {
	Date  time.Time  `json:"date"`
	Stats NightStats `json:"stats"`
}

// TrendPoint contains raw values and optional moving averages for one night.
type TrendPoint struct {
	Date time.Time `json:"date"`

	// Raw values
	LongestSleep time.Duration `json:"longestSleep"`
	TotalSleep   time.Duration `json:"totalSleep"`
	TotalFeed    time.Duration `json:"totalFeed"`
	WakeCount    int           `json:"wakeCount"`
	FeedCount    int           `json:"feedCount"`

	// Moving averages (nil if insufficient data for the window)
	AvgLongestSleep *time.Duration `json:"avgLongestSleep"`
	AvgTotalSleep   *time.Duration `json:"avgTotalSleep"`
	AvgTotalFeed    *time.Duration `json:"avgTotalFeed"`
	AvgWakeCount    *float64       `json:"avgWakeCount"`
	AvgFeedCount    *float64       `json:"avgFeedCount"`
}

// ComputeTrends produces trend points with moving averages from night data.
// The window parameter controls the moving average lookback (e.g., 3 for 3-night average).
func ComputeTrends(points []NightDataPoint, window int) []TrendPoint {
	if len(points) == 0 {
		return nil
	}

	trends := make([]TrendPoint, len(points))
	for i, p := range points {
		trends[i] = TrendPoint{
			Date:         p.Date,
			LongestSleep: p.Stats.LongestSleepBlock,
			TotalSleep:   p.Stats.TotalSleepTime,
			TotalFeed:    p.Stats.TotalFeedTime,
			WakeCount:    p.Stats.WakeCount,
			FeedCount:    p.Stats.FeedCount,
		}

		if i+1 >= window {
			start := i + 1 - window
			slice := points[start : i+1]

			avgLS := avgDuration(slice, func(p NightDataPoint) time.Duration { return p.Stats.LongestSleepBlock })
			avgTS := avgDuration(slice, func(p NightDataPoint) time.Duration { return p.Stats.TotalSleepTime })
			avgTF := avgDuration(slice, func(p NightDataPoint) time.Duration { return p.Stats.TotalFeedTime })
			avgWC := avgFloat(slice, func(p NightDataPoint) int { return p.Stats.WakeCount })
			avgFC := avgFloat(slice, func(p NightDataPoint) int { return p.Stats.FeedCount })

			trends[i].AvgLongestSleep = &avgLS
			trends[i].AvgTotalSleep = &avgTS
			trends[i].AvgTotalFeed = &avgTF
			trends[i].AvgWakeCount = &avgWC
			trends[i].AvgFeedCount = &avgFC
		}
	}

	return trends
}

func avgDuration(points []NightDataPoint, get func(NightDataPoint) time.Duration) time.Duration {
	var sum time.Duration
	for _, p := range points {
		sum += get(p)
	}
	return sum / time.Duration(len(points))
}

func avgFloat(points []NightDataPoint, get func(NightDataPoint) int) float64 {
	var sum int
	for _, p := range points {
		sum += get(p)
	}
	return float64(sum) / float64(len(points))
}
