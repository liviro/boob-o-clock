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

	LongestSleep  time.Duration `json:"longestSleep"`
	TotalSleep    time.Duration `json:"totalSleep"`
	TotalFeed     time.Duration `json:"totalFeed"`
	FeedTimeLeft  time.Duration `json:"feedTimeLeft"`
	FeedTimeRight time.Duration `json:"feedTimeRight"`
	WakeCount     int           `json:"wakeCount"`
	FeedCount     int           `json:"feedCount"`

	// Moving averages (nil if insufficient data for the window)
	AvgLongestSleep  *time.Duration `json:"avgLongestSleep"`
	AvgTotalSleep    *time.Duration `json:"avgTotalSleep"`
	AvgTotalFeed     *time.Duration `json:"avgTotalFeed"`
	AvgFeedTimeLeft  *time.Duration `json:"avgFeedTimeLeft"`
	AvgFeedTimeRight *time.Duration `json:"avgFeedTimeRight"`
	AvgWakeCount     *float64       `json:"avgWakeCount"`
	AvgFeedCount     *float64       `json:"avgFeedCount"`
}

// ComputeTrends produces trend points with moving averages from night data.
func ComputeTrends(points []NightDataPoint, window int) []TrendPoint {
	trends := make([]TrendPoint, len(points))

	for i, p := range points {
		trends[i] = TrendPoint{
			Date:          p.Date,
			LongestSleep:  p.Stats.LongestSleepBlock,
			TotalSleep:    p.Stats.TotalSleepTime,
			TotalFeed:     p.Stats.TotalFeedTime,
			FeedTimeLeft:  p.Stats.FeedTimeLeft,
			FeedTimeRight: p.Stats.FeedTimeRight,
			WakeCount:     p.Stats.WakeCount,
			FeedCount:     p.Stats.FeedCount,
		}

		if i+1 >= window {
			var sumLS, sumTS, sumTF, sumFL, sumFR time.Duration
			var sumWC, sumFC int
			for _, wp := range points[i+1-window : i+1] {
				sumLS += wp.Stats.LongestSleepBlock
				sumTS += wp.Stats.TotalSleepTime
				sumTF += wp.Stats.TotalFeedTime
				sumFL += wp.Stats.FeedTimeLeft
				sumFR += wp.Stats.FeedTimeRight
				sumWC += wp.Stats.WakeCount
				sumFC += wp.Stats.FeedCount
			}
			w := time.Duration(window)
			avgLS := sumLS / w
			avgTS := sumTS / w
			avgTF := sumTF / w
			avgFL := sumFL / w
			avgFR := sumFR / w
			avgWC := float64(sumWC) / float64(window)
			avgFC := float64(sumFC) / float64(window)

			trends[i].AvgLongestSleep = &avgLS
			trends[i].AvgTotalSleep = &avgTS
			trends[i].AvgTotalFeed = &avgTF
			trends[i].AvgFeedTimeLeft = &avgFL
			trends[i].AvgFeedTimeRight = &avgFR
			trends[i].AvgWakeCount = &avgWC
			trends[i].AvgFeedCount = &avgFC
		}
	}

	return trends
}
