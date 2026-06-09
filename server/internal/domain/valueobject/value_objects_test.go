package valueobject_test

import (
	"testing"

	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

func TestNewActivityType_Valid(t *testing.T) {
	valid := []string{"running", "cycling", "swimming", "weightlifting", "yoga", "hiking", "crossfit", "other"}
	for _, v := range valid {
		t.Run(v, func(t *testing.T) {
			at, err := valueobject.NewActivityType(v)
			if err != nil {
				t.Fatalf("expected no error for %q, got %v", v, err)
			}
			if string(at) != v {
				t.Fatalf("expected %q, got %q", v, at)
			}
		})
	}
}

func TestNewActivityType_Invalid(t *testing.T) {
	_, err := valueobject.NewActivityType("flying")
	if err != valueobject.ErrInvalidActivityType {
		t.Fatalf("expected ErrInvalidActivityType, got %v", err)
	}
}

func TestNewMetricType_Valid(t *testing.T) {
	valid := []string{"hrv", "resting_hr", "sleep_duration", "sleep_quality", "weight", "body_fat", "calories_in", "calories_out", "training_load"}
	for _, v := range valid {
		t.Run(v, func(t *testing.T) {
			mt, err := valueobject.NewMetricType(v)
			if err != nil {
				t.Fatalf("expected no error for %q, got %v", v, err)
			}
			if string(mt) != v {
				t.Fatalf("expected %q, got %q", v, mt)
			}
		})
	}
}

func TestNewMetricType_Invalid(t *testing.T) {
	_, err := valueobject.NewMetricType("invalid_metric")
	if err != valueobject.ErrInvalidMetricType {
		t.Fatalf("expected ErrInvalidMetricType, got %v", err)
	}
}

func TestNewSeverity_Valid(t *testing.T) {
	valid := []string{"info", "warning", "critical"}
	for _, v := range valid {
		t.Run(v, func(t *testing.T) {
			s, err := valueobject.NewSeverity(v)
			if err != nil {
				t.Fatalf("expected no error for %q, got %v", v, err)
			}
			if string(s) != v {
				t.Fatalf("expected %q, got %q", v, s)
			}
		})
	}
}

func TestNewSeverity_Invalid(t *testing.T) {
	_, err := valueobject.NewSeverity("urgent")
	if err != valueobject.ErrInvalidSeverity {
		t.Fatalf("expected ErrInvalidSeverity, got %v", err)
	}
}

func TestNewInsightType_Valid(t *testing.T) {
	valid := []string{"hrv_drop", "resting_hr_high", "sleep_deficit", "overtraining", "undertraining", "recovery_needed"}
	for _, v := range valid {
		t.Run(v, func(t *testing.T) {
			it, err := valueobject.NewInsightType(v)
			if err != nil {
				t.Fatalf("expected no error for %q, got %v", v, err)
			}
			if string(it) != v {
				t.Fatalf("expected %q, got %q", v, it)
			}
		})
	}
}

func TestNewInsightType_Invalid(t *testing.T) {
	_, err := valueobject.NewInsightType("invalid_insight")
	if err != valueobject.ErrInvalidInsightType {
		t.Fatalf("expected ErrInvalidInsightType, got %v", err)
	}
}
