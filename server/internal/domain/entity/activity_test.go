package entity_test

import (
	"testing"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

func TestNewActivity_Valid(t *testing.T) {
	a, err := entity.NewActivity(
		"user-1",
		valueobject.ActivityTypeRunning,
		time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC),
		30*time.Minute,
		150,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if a.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if a.UserID != "user-1" {
		t.Fatalf("expected user-1, got %s", a.UserID)
	}
	if a.Duration != 30*time.Minute {
		t.Fatalf("expected 30m, got %v", a.Duration)
	}
}

func TestNewActivity_EmptyUserID(t *testing.T) {
	_, err := entity.NewActivity(
		"",
		valueobject.ActivityTypeRunning,
		time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC),
		30*time.Minute,
		150,
		"",
	)
	if err != entity.ErrEmptyUserID {
		t.Fatalf("expected ErrEmptyUserID, got %v", err)
	}
}

func TestNewActivity_ZeroDuration(t *testing.T) {
	_, err := entity.NewActivity(
		"user-1",
		valueobject.ActivityTypeRunning,
		time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC),
		0,
		150,
		"",
	)
	if err != entity.ErrInvalidDuration {
		t.Fatalf("expected ErrInvalidDuration, got %v", err)
	}
}

func TestNewActivity_NegativeDuration(t *testing.T) {
	_, err := entity.NewActivity(
		"user-1",
		valueobject.ActivityTypeRunning,
		time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC),
		-10*time.Minute,
		150,
		"",
	)
	if err != entity.ErrInvalidDuration {
		t.Fatalf("expected ErrInvalidDuration, got %v", err)
	}
}

func TestNewActivity_FutureDate(t *testing.T) {
	future := time.Now().AddDate(1, 0, 0)
	_, err := entity.NewActivity(
		"user-1",
		valueobject.ActivityTypeRunning,
		future,
		30*time.Minute,
		150,
		"",
	)
	if err != entity.ErrActivityDateFuture {
		t.Fatalf("expected ErrActivityDateFuture, got %v", err)
	}
}

func TestNewActivity_AvgHRTooLow(t *testing.T) {
	_, err := entity.NewActivity(
		"user-1",
		valueobject.ActivityTypeRunning,
		time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC),
		30*time.Minute,
		25,
		"",
	)
	if err != entity.ErrHROutOfRange {
		t.Fatalf("expected ErrHROutOfRange, got %v", err)
	}
}

func TestNewActivity_AvgHRTooHigh(t *testing.T) {
	_, err := entity.NewActivity(
		"user-1",
		valueobject.ActivityTypeRunning,
		time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC),
		30*time.Minute,
		225,
		"",
	)
	if err != entity.ErrHROutOfRange {
		t.Fatalf("expected ErrHROutOfRange, got %v", err)
	}
}

func TestNewActivity_ZeroAvgHR_IsValid(t *testing.T) {
	// Zero HR means "not recorded" — should be allowed
	a, err := entity.NewActivity(
		"user-1",
		valueobject.ActivityTypeYoga,
		time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC),
		60*time.Minute,
		0,
		"",
	)
	if err != nil {
		t.Fatalf("expected no error for zero HR, got %v", err)
	}
	if a.AvgHeartRate != 0 {
		t.Fatalf("expected 0, got %d", a.AvgHeartRate)
	}
}

func TestNewActivity_WithExternalID(t *testing.T) {
	a, err := entity.NewActivity(
		"user-1",
		valueobject.ActivityTypeRunning,
		time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC),
		30*time.Minute,
		150,
		"strava-12345",
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if a.ExternalID != "strava-12345" {
		t.Fatalf("expected strava-12345, got %s", a.ExternalID)
	}
}
