package deacon

import (
	"sort"
	"time"

	"github.com/steveyegge/gastown/internal/guardian"
)

// TrendConfig holds configurable parameters for trend analysis.
type TrendConfig struct {
	// Window is the analysis window for filtering recent results.
	Window time.Duration
}

// DefaultTrendConfig returns the default trend config (24h window).
func DefaultTrendConfig() *TrendConfig {
	return &TrendConfig{
		Window: 24 * time.Hour,
	}
}

// WorkerTrend holds trend analysis for a single worker.
type WorkerTrend struct {
	Worker        string  `json:"worker"`
	ReviewCount   int     `json:"review_count"`
	AvgScore      float64 `json:"avg_score"`
	RejectionRate float64 `json:"rejection_rate"`
	Status        string  `json:"status"`
	Trend         string  `json:"trend"`
}

// TrendScanResult contains the full results of a trend scan.
type TrendScanResult struct {
	ScannedAt    time.Time      `json:"scanned_at"`
	Window       time.Duration  `json:"window"`
	TotalWorkers int            `json:"total_workers"`
	BreachCount  int            `json:"breach_count"`
	WarnCount    int            `json:"warn_count"`
	Workers      []*WorkerTrend `json:"workers"`
}

// ScanTrends reads Guardian judgment state and computes quality trends per worker.
func ScanTrends(townRoot string, cfg *TrendConfig) (*TrendScanResult, error) {
	if cfg == nil {
		cfg = DefaultTrendConfig()
	}

	state, err := guardian.LoadState(townRoot)
	if err != nil {
		return nil, err
	}

	now := time.Now()
	cutoff := now.Add(-cfg.Window)

	result := &TrendScanResult{
		ScannedAt: now.UTC(),
		Window:    cfg.Window,
		Workers:   make([]*WorkerTrend, 0, len(state.Workers)),
	}

	for name, pj := range state.Workers {
		// Filter results within the window.
		var windowed []guardian.RecentResult
		for _, r := range pj.RecentResults {
			if !r.ReviewedAt.Before(cutoff) {
				windowed = append(windowed, r)
			}
		}

		if len(windowed) == 0 {
			continue
		}

		// Compute windowed avg score and rejection rate.
		var totalScore float64
		var rejections int
		for _, r := range windowed {
			totalScore += r.Score
			if r.Recommendation == guardian.RecommendRequestChanges {
				rejections++
			}
		}

		n := len(windowed)
		avgScore := totalScore / float64(n)
		rejectionRate := float64(rejections) / float64(n)

		wt := &WorkerTrend{
			Worker:        name,
			ReviewCount:   n,
			AvgScore:      avgScore,
			RejectionRate: rejectionRate,
			Status:        guardian.StatusForScore(avgScore),
			Trend:         computeTrend(windowed),
		}

		result.Workers = append(result.Workers, wt)

		switch wt.Status {
		case guardian.StatusBreach:
			result.BreachCount++
		case guardian.StatusWarn:
			result.WarnCount++
		}
	}

	result.TotalWorkers = len(result.Workers)

	// Sort workers by avg score ascending (worst first).
	sort.Slice(result.Workers, func(i, j int) bool {
		return result.Workers[i].AvgScore < result.Workers[j].AvgScore
	})

	return result, nil
}

// computeTrend compares first-half avg vs second-half avg.
// Returns "improving", "declining", or "stable".
func computeTrend(results []guardian.RecentResult) string {
	if len(results) < 2 {
		return "stable"
	}

	mid := len(results) / 2
	firstHalf := results[:mid]
	secondHalf := results[mid:]

	firstAvg := avgScore(firstHalf)
	secondAvg := avgScore(secondHalf)

	const threshold = 0.05
	diff := secondAvg - firstAvg

	switch {
	case diff > threshold:
		return "improving"
	case diff < -threshold:
		return "declining"
	default:
		return "stable"
	}
}

func avgScore(results []guardian.RecentResult) float64 {
	if len(results) == 0 {
		return 0
	}
	var total float64
	for _, r := range results {
		total += r.Score
	}
	return total / float64(len(results))
}
