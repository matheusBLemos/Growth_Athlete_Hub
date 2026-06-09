package entity_test

import (
	"testing"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

func TestNewMetric_Valid(t *testing.T) {
	m, err := entity.NewMetric(
		"user-1",
		valueobject.MetricTypeHRV,
		65.0,
		time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if m.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if m.Value != 65.0 {
		t.Fatalf("expected 65.0, got %f", m.Value)
	}
}

func TestNewMetric_EmptyUserID(t *testing.T) {
	_, err := entity.NewMetric(
		"",
		valueobject.MetricTypeHRV,
		65.0,
		time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
	)
	if err != entity.ErrEmptyUserID {
		t.Fatalf("expected ErrEmptyUserID, got %v", err)
	}
}

func TestNewMetric_HRV_ValidRange(t *testing.T) {
	cases := []struct {
		name  string
		value float64
	}{
		{"min", 0},
		{"mid", 150},
		{"max", 300},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := entity.NewMetric("user-1", valueobject.MetricTypeHRV, tc.value, time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC))
			if err != nil {
				t.Fatalf("expected no error for HRV=%f, got %v", tc.value, err)
			}
		})
	}
}

func TestNewMetric_HRV_OutOfRange(t *testing.T) {
	cases := []struct {
		name  string
		value float64
	}{
		{"negative", -1},
		{"too_high", 301},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := entity.NewMetric("user-1", valueobject.MetricTypeHRV, tc.value, time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC))
			if err != entity.ErrMetricOutOfRange {
				t.Fatalf("expected ErrMetricOutOfRange for HRV=%f, got %v", tc.value, err)
			}
		})
	}
}

func TestNewMetric_RestingHR_ValidRange(t *testing.T) {
	_, err := entity.NewMetric("user-1", valueobject.MetricTypeRestingHR, 60, time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestNewMetric_RestingHR_OutOfRange(t *testing.T) {
	cases := []struct {
		name  string
		value float64
	}{
		{"too_low", 29},
		{"too_high", 121},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := entity.NewMetric("user-1", valueobject.MetricTypeRestingHR, tc.value, time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC))
			if err != entity.ErrMetricOutOfRange {
				t.Fatalf("expected ErrMetricOutOfRange for RestingHR=%f, got %v", tc.value, err)
			}
		})
	}
}

func TestNewMetric_SleepDuration_ValidRange(t *testing.T) {
	_, err := entity.NewMetric("user-1", valueobject.MetricTypeSleepDuration, 8, time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestNewMetric_SleepDuration_OutOfRange(t *testing.T) {
	cases := []struct {
		name  string
		value float64
	}{
		{"negative", -1},
		{"too_high", 25},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := entity.NewMetric("user-1", valueobject.MetricTypeSleepDuration, tc.value, time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC))
			if err != entity.ErrMetricOutOfRange {
				t.Fatalf("expected ErrMetricOutOfRange for SleepDuration=%f, got %v", tc.value, err)
			}
		})
	}
}

func TestNewMetric_Weight_ValidRange(t *testing.T) {
	_, err := entity.NewMetric("user-1", valueobject.MetricTypeWeight, 75.5, time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
}

func TestNewMetric_Weight_OutOfRange(t *testing.T) {
	cases := []struct {
		name  string
		value float64
	}{
		{"too_low", 19},
		{"too_high", 301},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := entity.NewMetric("user-1", valueobject.MetricTypeWeight, tc.value, time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC))
			if err != entity.ErrMetricOutOfRange {
				t.Fatalf("expected ErrMetricOutOfRange for Weight=%f, got %v", tc.value, err)
			}
		})
	}
}

func TestNewMetric_ZeroDate(t *testing.T) {
	_, err := entity.NewMetric("user-1", valueobject.MetricTypeHRV, 65, time.Time{})
	if err != entity.ErrEmptyDate {
		t.Fatalf("expected ErrEmptyDate, got %v", err)
	}
}
