package deacon

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/steveyegge/gastown/internal/guardian"
)

func setupState(t *testing.T, state *guardian.GuardianState) string {
	t.Helper()
	dir := t.TempDir()
	if err := guardian.SaveState(dir, state); err != nil {
		t.Fatalf("saving state: %v", err)
	}
	// Verify the file exists at expected path.
	_ = filepath.Join(dir, "guardian", "judgment-state.json")
	return dir
}

func makeResult(score float64, rec string, ago time.Duration) guardian.RecentResult {
	return guardian.RecentResult{
		BeadID:         "gt-test",
		Score:          score,
		Recommendation: rec,
		ReviewedAt:     time.Now().Add(-ago),
	}
}

func TestScanTrendsEmptyState(t *testing.T) {
	dir := setupState(t, guardian.NewGuardianState())

	result, err := ScanTrends(dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalWorkers != 0 {
		t.Errorf("expected 0 workers, got %d", result.TotalWorkers)
	}
	if result.BreachCount != 0 {
		t.Errorf("expected 0 breaches, got %d", result.BreachCount)
	}
}

func TestScanTrendsNoStateFile(t *testing.T) {
	dir := t.TempDir()

	result, err := ScanTrends(dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalWorkers != 0 {
		t.Errorf("expected 0 workers, got %d", result.TotalWorkers)
	}
}

func TestScanTrendsAllOK(t *testing.T) {
	state := guardian.NewGuardianState()
	state.AddResult("polecat-A", guardian.RecentResult{
		BeadID: "gt-1", Score: 0.85, Recommendation: guardian.RecommendApprove,
		ReviewedAt: time.Now().Add(-1 * time.Hour),
	})
	state.AddResult("polecat-A", guardian.RecentResult{
		BeadID: "gt-2", Score: 0.90, Recommendation: guardian.RecommendApprove,
		ReviewedAt: time.Now().Add(-30 * time.Minute),
	})

	dir := setupState(t, state)
	result, err := ScanTrends(dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalWorkers != 1 {
		t.Fatalf("expected 1 worker, got %d", result.TotalWorkers)
	}
	w := result.Workers[0]
	if w.Status != guardian.StatusOK {
		t.Errorf("expected OK status, got %s", w.Status)
	}
	if w.ReviewCount != 2 {
		t.Errorf("expected 2 reviews, got %d", w.ReviewCount)
	}
}

func TestScanTrendsBreachWorker(t *testing.T) {
	state := guardian.NewGuardianState()
	state.AddResult("polecat-Bad", guardian.RecentResult{
		BeadID: "gt-1", Score: 0.30, Recommendation: guardian.RecommendRequestChanges,
		ReviewedAt: time.Now().Add(-2 * time.Hour),
	})
	state.AddResult("polecat-Bad", guardian.RecentResult{
		BeadID: "gt-2", Score: 0.35, Recommendation: guardian.RecommendRequestChanges,
		ReviewedAt: time.Now().Add(-1 * time.Hour),
	})

	dir := setupState(t, state)
	result, err := ScanTrends(dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.BreachCount != 1 {
		t.Errorf("expected 1 breach, got %d", result.BreachCount)
	}
	w := result.Workers[0]
	if w.Status != guardian.StatusBreach {
		t.Errorf("expected BREACH, got %s", w.Status)
	}
	if w.RejectionRate != 1.0 {
		t.Errorf("expected 100%% rejection rate, got %.2f", w.RejectionRate)
	}
}

func TestScanTrendsWarnWorker(t *testing.T) {
	state := guardian.NewGuardianState()
	state.AddResult("polecat-Meh", guardian.RecentResult{
		BeadID: "gt-1", Score: 0.50, Recommendation: guardian.RecommendApprove,
		ReviewedAt: time.Now().Add(-2 * time.Hour),
	})
	state.AddResult("polecat-Meh", guardian.RecentResult{
		BeadID: "gt-2", Score: 0.55, Recommendation: guardian.RecommendApprove,
		ReviewedAt: time.Now().Add(-1 * time.Hour),
	})

	dir := setupState(t, state)
	result, err := ScanTrends(dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.WarnCount != 1 {
		t.Errorf("expected 1 warn, got %d", result.WarnCount)
	}
	w := result.Workers[0]
	if w.Status != guardian.StatusWarn {
		t.Errorf("expected WARN, got %s", w.Status)
	}
}

func TestScanTrendsTrendImproving(t *testing.T) {
	state := guardian.NewGuardianState()
	// First half: low scores, second half: high scores.
	state.AddResult("polecat-Up", guardian.RecentResult{
		BeadID: "gt-1", Score: 0.50, Recommendation: guardian.RecommendApprove,
		ReviewedAt: time.Now().Add(-4 * time.Hour),
	})
	state.AddResult("polecat-Up", guardian.RecentResult{
		BeadID: "gt-2", Score: 0.55, Recommendation: guardian.RecommendApprove,
		ReviewedAt: time.Now().Add(-3 * time.Hour),
	})
	state.AddResult("polecat-Up", guardian.RecentResult{
		BeadID: "gt-3", Score: 0.80, Recommendation: guardian.RecommendApprove,
		ReviewedAt: time.Now().Add(-2 * time.Hour),
	})
	state.AddResult("polecat-Up", guardian.RecentResult{
		BeadID: "gt-4", Score: 0.85, Recommendation: guardian.RecommendApprove,
		ReviewedAt: time.Now().Add(-1 * time.Hour),
	})

	dir := setupState(t, state)
	result, err := ScanTrends(dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	w := result.Workers[0]
	if w.Trend != "improving" {
		t.Errorf("expected improving trend, got %s", w.Trend)
	}
}

func TestScanTrendsTrendDeclining(t *testing.T) {
	state := guardian.NewGuardianState()
	// First half: high scores, second half: low scores.
	state.AddResult("polecat-Down", guardian.RecentResult{
		BeadID: "gt-1", Score: 0.90, Recommendation: guardian.RecommendApprove,
		ReviewedAt: time.Now().Add(-4 * time.Hour),
	})
	state.AddResult("polecat-Down", guardian.RecentResult{
		BeadID: "gt-2", Score: 0.85, Recommendation: guardian.RecommendApprove,
		ReviewedAt: time.Now().Add(-3 * time.Hour),
	})
	state.AddResult("polecat-Down", guardian.RecentResult{
		BeadID: "gt-3", Score: 0.60, Recommendation: guardian.RecommendRequestChanges,
		ReviewedAt: time.Now().Add(-2 * time.Hour),
	})
	state.AddResult("polecat-Down", guardian.RecentResult{
		BeadID: "gt-4", Score: 0.55, Recommendation: guardian.RecommendRequestChanges,
		ReviewedAt: time.Now().Add(-1 * time.Hour),
	})

	dir := setupState(t, state)
	result, err := ScanTrends(dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	w := result.Workers[0]
	if w.Trend != "declining" {
		t.Errorf("expected declining trend, got %s", w.Trend)
	}
}

func TestScanTrendsTrendStable(t *testing.T) {
	state := guardian.NewGuardianState()
	state.AddResult("polecat-Flat", guardian.RecentResult{
		BeadID: "gt-1", Score: 0.80, Recommendation: guardian.RecommendApprove,
		ReviewedAt: time.Now().Add(-4 * time.Hour),
	})
	state.AddResult("polecat-Flat", guardian.RecentResult{
		BeadID: "gt-2", Score: 0.82, Recommendation: guardian.RecommendApprove,
		ReviewedAt: time.Now().Add(-3 * time.Hour),
	})
	state.AddResult("polecat-Flat", guardian.RecentResult{
		BeadID: "gt-3", Score: 0.79, Recommendation: guardian.RecommendApprove,
		ReviewedAt: time.Now().Add(-2 * time.Hour),
	})
	state.AddResult("polecat-Flat", guardian.RecentResult{
		BeadID: "gt-4", Score: 0.81, Recommendation: guardian.RecommendApprove,
		ReviewedAt: time.Now().Add(-1 * time.Hour),
	})

	dir := setupState(t, state)
	result, err := ScanTrends(dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	w := result.Workers[0]
	if w.Trend != "stable" {
		t.Errorf("expected stable trend, got %s", w.Trend)
	}
}

func TestScanTrendsSingleResultStable(t *testing.T) {
	state := guardian.NewGuardianState()
	state.AddResult("polecat-Solo", guardian.RecentResult{
		BeadID: "gt-1", Score: 0.75, Recommendation: guardian.RecommendApprove,
		ReviewedAt: time.Now().Add(-1 * time.Hour),
	})

	dir := setupState(t, state)
	result, err := ScanTrends(dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	w := result.Workers[0]
	if w.Trend != "stable" {
		t.Errorf("single result should be stable, got %s", w.Trend)
	}
}

func TestScanTrendsWindowFiltering(t *testing.T) {
	state := guardian.NewGuardianState()
	// One result inside 1h window, one outside.
	state.AddResult("polecat-W", guardian.RecentResult{
		BeadID: "gt-old", Score: 0.30, Recommendation: guardian.RecommendRequestChanges,
		ReviewedAt: time.Now().Add(-3 * time.Hour),
	})
	state.AddResult("polecat-W", guardian.RecentResult{
		BeadID: "gt-new", Score: 0.90, Recommendation: guardian.RecommendApprove,
		ReviewedAt: time.Now().Add(-30 * time.Minute),
	})

	dir := setupState(t, state)
	cfg := &TrendConfig{Window: 1 * time.Hour}
	result, err := ScanTrends(dir, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalWorkers != 1 {
		t.Fatalf("expected 1 worker, got %d", result.TotalWorkers)
	}
	w := result.Workers[0]
	if w.ReviewCount != 1 {
		t.Errorf("expected 1 review (old one filtered out), got %d", w.ReviewCount)
	}
	if w.AvgScore != 0.90 {
		t.Errorf("expected avg score 0.90, got %.2f", w.AvgScore)
	}
}

func TestScanTrendsAllOutsideWindow(t *testing.T) {
	state := guardian.NewGuardianState()
	state.AddResult("polecat-Old", guardian.RecentResult{
		BeadID: "gt-1", Score: 0.50, Recommendation: guardian.RecommendApprove,
		ReviewedAt: time.Now().Add(-48 * time.Hour),
	})

	dir := setupState(t, state)
	cfg := &TrendConfig{Window: 1 * time.Hour}
	result, err := ScanTrends(dir, cfg)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalWorkers != 0 {
		t.Errorf("expected 0 workers (all outside window), got %d", result.TotalWorkers)
	}
}

func TestScanTrendsMultipleWorkersSorted(t *testing.T) {
	state := guardian.NewGuardianState()
	// Good worker.
	state.AddResult("polecat-Good", guardian.RecentResult{
		BeadID: "gt-1", Score: 0.90, Recommendation: guardian.RecommendApprove,
		ReviewedAt: time.Now().Add(-1 * time.Hour),
	})
	// Bad worker.
	state.AddResult("polecat-Bad", guardian.RecentResult{
		BeadID: "gt-2", Score: 0.30, Recommendation: guardian.RecommendRequestChanges,
		ReviewedAt: time.Now().Add(-1 * time.Hour),
	})
	// Mid worker.
	state.AddResult("polecat-Mid", guardian.RecentResult{
		BeadID: "gt-3", Score: 0.55, Recommendation: guardian.RecommendApprove,
		ReviewedAt: time.Now().Add(-1 * time.Hour),
	})

	dir := setupState(t, state)
	result, err := ScanTrends(dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.TotalWorkers != 3 {
		t.Fatalf("expected 3 workers, got %d", result.TotalWorkers)
	}
	// Worst first.
	if result.Workers[0].Worker != "polecat-Bad" {
		t.Errorf("expected worst worker first, got %s", result.Workers[0].Worker)
	}
	if result.Workers[1].Worker != "polecat-Mid" {
		t.Errorf("expected mid worker second, got %s", result.Workers[1].Worker)
	}
	if result.Workers[2].Worker != "polecat-Good" {
		t.Errorf("expected best worker last, got %s", result.Workers[2].Worker)
	}
}

func TestDefaultTrendConfig(t *testing.T) {
	cfg := DefaultTrendConfig()
	if cfg.Window != 24*time.Hour {
		t.Errorf("expected 24h window, got %v", cfg.Window)
	}
}

func TestScanTrendsNilConfig(t *testing.T) {
	dir := setupState(t, guardian.NewGuardianState())
	result, err := ScanTrends(dir, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Window != 24*time.Hour {
		t.Errorf("nil config should default to 24h, got %v", result.Window)
	}
}
