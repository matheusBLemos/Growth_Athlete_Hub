package deterministic_test

import (
	"context"
	"testing"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/insights/deterministic"
)

func makeMetrics(metricType valueobject.MetricType, values []float64) []*entity.Metric {
	var metrics []*entity.Metric
	now := time.Now()
	for i, v := range values {
		m, _ := entity.NewMetric("user-1", metricType, v, now.AddDate(0, 0, -len(values)+1+i))
		metrics = append(metrics, m)
	}
	return metrics
}

// --- HRV Rule ---

func TestHRVRule_DropBelow15Percent_Warning(t *testing.T) {
	// Baseline 7 days avg = 70, current = 55 → drop ~21% → warning
	baseline := []float64{70, 72, 68, 71, 69, 70, 70}
	current := makeMetrics(valueobject.MetricTypeHRV, append(baseline, 55))

	rule := deterministic.NewHRVRule()
	insights, err := rule.Evaluate(context.Background(), "user-1", current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(insights) != 1 {
		t.Fatalf("expected 1 insight, got %d", len(insights))
	}
	if insights[0].Severity != valueobject.SeverityWarning {
		t.Fatalf("expected warning, got %s", insights[0].Severity)
	}
	if insights[0].Type != valueobject.InsightTypeHRVDrop {
		t.Fatalf("expected hrv_drop, got %s", insights[0].Type)
	}
}

func TestHRVRule_NoDrop_NoInsight(t *testing.T) {
	// Baseline avg = 70, current = 65 → drop ~7% → no insight
	baseline := []float64{70, 72, 68, 71, 69, 70, 70}
	current := makeMetrics(valueobject.MetricTypeHRV, append(baseline, 65))

	rule := deterministic.NewHRVRule()
	insights, err := rule.Evaluate(context.Background(), "user-1", current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(insights) != 0 {
		t.Fatalf("expected 0 insights, got %d", len(insights))
	}
}

func TestHRVRule_InsufficientData_NoInsight(t *testing.T) {
	metrics := makeMetrics(valueobject.MetricTypeHRV, []float64{70, 65})

	rule := deterministic.NewHRVRule()
	insights, err := rule.Evaluate(context.Background(), "user-1", metrics)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(insights) != 0 {
		t.Fatalf("expected 0 insights with insufficient data, got %d", len(insights))
	}
}

// --- Resting HR Rule ---

func TestRestingHRRule_RiseAbove10Percent_Warning(t *testing.T) {
	// Baseline avg = 60, current = 67 → rise ~12% → warning
	baseline := []float64{60, 58, 62, 60, 59, 61, 60}
	current := makeMetrics(valueobject.MetricTypeRestingHR, append(baseline, 67))

	rule := deterministic.NewRestingHRRule()
	insights, err := rule.Evaluate(context.Background(), "user-1", current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(insights) != 1 {
		t.Fatalf("expected 1 insight, got %d", len(insights))
	}
	if insights[0].Severity != valueobject.SeverityWarning {
		t.Fatalf("expected warning, got %s", insights[0].Severity)
	}
}

func TestRestingHRRule_NoRise_NoInsight(t *testing.T) {
	baseline := []float64{60, 58, 62, 60, 59, 61, 60}
	current := makeMetrics(valueobject.MetricTypeRestingHR, append(baseline, 62))

	rule := deterministic.NewRestingHRRule()
	insights, err := rule.Evaluate(context.Background(), "user-1", current)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(insights) != 0 {
		t.Fatalf("expected 0 insights, got %d", len(insights))
	}
}

// --- Sleep Rule ---

func TestSleepRule_Below6h_3ConsecutiveDays_Warning(t *testing.T) {
	values := []float64{7.5, 7, 8, 5.5, 5.8, 5.5}
	metrics := makeMetrics(valueobject.MetricTypeSleepDuration, values)

	rule := deterministic.NewSleepRule()
	insights, err := rule.Evaluate(context.Background(), "user-1", metrics)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(insights) != 1 {
		t.Fatalf("expected 1 insight, got %d", len(insights))
	}
	if insights[0].Severity != valueobject.SeverityWarning {
		t.Fatalf("expected warning, got %s", insights[0].Severity)
	}
}

func TestSleepRule_Below5h_Critical(t *testing.T) {
	values := []float64{7, 7.5, 4.5, 4.8, 4.5}
	metrics := makeMetrics(valueobject.MetricTypeSleepDuration, values)

	rule := deterministic.NewSleepRule()
	insights, err := rule.Evaluate(context.Background(), "user-1", metrics)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, i := range insights {
		if i.Severity == valueobject.SeverityCritical {
			found = true
		}
	}
	if !found {
		t.Fatal("expected at least one critical insight for sleep below 5h")
	}
}

func TestSleepRule_AdequateSleep_NoInsight(t *testing.T) {
	values := []float64{7, 7.5, 8, 7, 6.5}
	metrics := makeMetrics(valueobject.MetricTypeSleepDuration, values)

	rule := deterministic.NewSleepRule()
	insights, err := rule.Evaluate(context.Background(), "user-1", metrics)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(insights) != 0 {
		t.Fatalf("expected 0 insights, got %d", len(insights))
	}
}

// --- ACWR Rule ---

func TestACWRRule_Above1_5_Warning(t *testing.T) {
	// 7-day acute high, 28-day chronic low → ACWR > 1.5
	var values []float64
	for i := 0; i < 21; i++ {
		values = append(values, 100)
	}
	for i := 0; i < 7; i++ {
		values = append(values, 250)
	}
	metrics := makeMetrics(valueobject.MetricTypeTrainingLoad, values)

	rule := deterministic.NewACWRRule()
	insights, err := rule.Evaluate(context.Background(), "user-1", metrics)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(insights) == 0 {
		t.Fatal("expected at least 1 insight for ACWR > 1.5")
	}
	if insights[0].Severity != valueobject.SeverityWarning {
		t.Fatalf("expected warning, got %s", insights[0].Severity)
	}
}

func TestACWRRule_Above2_0_Critical(t *testing.T) {
	var values []float64
	for i := 0; i < 21; i++ {
		values = append(values, 80)
	}
	for i := 0; i < 7; i++ {
		values = append(values, 300)
	}
	metrics := makeMetrics(valueobject.MetricTypeTrainingLoad, values)

	rule := deterministic.NewACWRRule()
	insights, err := rule.Evaluate(context.Background(), "user-1", metrics)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	found := false
	for _, i := range insights {
		if i.Severity == valueobject.SeverityCritical {
			found = true
		}
	}
	if !found {
		t.Fatal("expected critical insight for ACWR > 2.0")
	}
}

func TestACWRRule_Below0_8_Info(t *testing.T) {
	var values []float64
	for i := 0; i < 21; i++ {
		values = append(values, 200)
	}
	for i := 0; i < 7; i++ {
		values = append(values, 50)
	}
	metrics := makeMetrics(valueobject.MetricTypeTrainingLoad, values)

	rule := deterministic.NewACWRRule()
	insights, err := rule.Evaluate(context.Background(), "user-1", metrics)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(insights) == 0 {
		t.Fatal("expected insight for undertraining (ACWR < 0.8)")
	}
	if insights[0].Type != valueobject.InsightTypeUndertraining {
		t.Fatalf("expected undertraining, got %s", insights[0].Type)
	}
	if insights[0].Severity != valueobject.SeverityInfo {
		t.Fatalf("expected info, got %s", insights[0].Severity)
	}
}

func TestACWRRule_NormalRange_NoInsight(t *testing.T) {
	var values []float64
	for i := 0; i < 28; i++ {
		values = append(values, 150)
	}
	metrics := makeMetrics(valueobject.MetricTypeTrainingLoad, values)

	rule := deterministic.NewACWRRule()
	insights, err := rule.Evaluate(context.Background(), "user-1", metrics)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(insights) != 0 {
		t.Fatalf("expected 0 insights for normal ACWR, got %d", len(insights))
	}
}

// --- Composite Evaluator ---

func TestCompositeEvaluator_CombinesRules(t *testing.T) {
	evaluator := deterministic.NewCompositeEvaluator(
		deterministic.NewHRVRule(),
		deterministic.NewRestingHRRule(),
	)

	// HRV drop scenario
	hrvMetrics := makeMetrics(valueobject.MetricTypeHRV, []float64{70, 72, 68, 71, 69, 70, 70, 55})
	// Resting HR normal
	hrMetrics := makeMetrics(valueobject.MetricTypeRestingHR, []float64{60, 58, 62, 60, 59, 61, 60, 61})

	all := append(hrvMetrics, hrMetrics...)
	insights, err := evaluator.Evaluate(context.Background(), "user-1", all)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(insights) != 1 {
		t.Fatalf("expected 1 insight (HRV drop only), got %d", len(insights))
	}
}

// --- Recovery Rule ---

func TestRecoveryRule_MultipleSignals_Critical(t *testing.T) {
	// HRV drop + sleep deficit + resting HR elevated = recovery needed
	hrvMetrics := makeMetrics(valueobject.MetricTypeHRV, []float64{70, 72, 68, 71, 69, 70, 70, 50})
	sleepMetrics := makeMetrics(valueobject.MetricTypeSleepDuration, []float64{7, 7, 5.5, 5.5, 5.5})
	hrMetrics := makeMetrics(valueobject.MetricTypeRestingHR, []float64{60, 58, 62, 60, 59, 61, 60, 70})

	all := append(append(hrvMetrics, sleepMetrics...), hrMetrics...)

	evaluator := deterministic.NewCompositeEvaluator(
		deterministic.NewHRVRule(),
		deterministic.NewSleepRule(),
		deterministic.NewRestingHRRule(),
		deterministic.NewRecoveryRule(),
	)

	insights, err := evaluator.Evaluate(context.Background(), "user-1", all)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	foundRecovery := false
	for _, i := range insights {
		if i.Type == valueobject.InsightTypeRecoveryNeeded && i.Severity == valueobject.SeverityCritical {
			foundRecovery = true
		}
	}
	if !foundRecovery {
		t.Fatal("expected recovery_needed critical insight when 3 signals combine")
	}
}

func TestRecoveryRule_SingleSignal_NoRecoveryInsight(t *testing.T) {
	// Only HRV drop, no other signals
	hrvMetrics := makeMetrics(valueobject.MetricTypeHRV, []float64{70, 72, 68, 71, 69, 70, 70, 55})

	evaluator := deterministic.NewCompositeEvaluator(
		deterministic.NewHRVRule(),
		deterministic.NewRecoveryRule(),
	)

	insights, err := evaluator.Evaluate(context.Background(), "user-1", hrvMetrics)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for _, i := range insights {
		if i.Type == valueobject.InsightTypeRecoveryNeeded {
			t.Fatal("should not generate recovery_needed with only 1 signal")
		}
	}
}
