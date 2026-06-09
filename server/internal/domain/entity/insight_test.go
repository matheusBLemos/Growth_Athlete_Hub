package entity_test

import (
	"testing"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

func TestNewInsight_Valid(t *testing.T) {
	i, err := entity.NewInsight(
		"user-1",
		valueobject.InsightTypeHRVDrop,
		valueobject.SeverityWarning,
		"HRV dropped 20% below 7-day baseline",
		time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if i.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if i.UserID != "user-1" {
		t.Fatalf("expected user-1, got %s", i.UserID)
	}
	if i.Message != "HRV dropped 20% below 7-day baseline" {
		t.Fatalf("unexpected message: %s", i.Message)
	}
}

func TestNewInsight_EmptyUserID(t *testing.T) {
	_, err := entity.NewInsight(
		"",
		valueobject.InsightTypeHRVDrop,
		valueobject.SeverityWarning,
		"msg",
		time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
	)
	if err != entity.ErrEmptyUserID {
		t.Fatalf("expected ErrEmptyUserID, got %v", err)
	}
}

func TestNewInsight_EmptyMessage(t *testing.T) {
	_, err := entity.NewInsight(
		"user-1",
		valueobject.InsightTypeHRVDrop,
		valueobject.SeverityWarning,
		"",
		time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
	)
	if err != entity.ErrEmptyMessage {
		t.Fatalf("expected ErrEmptyMessage, got %v", err)
	}
}

func TestNewInsight_CreatedAtIsSet(t *testing.T) {
	i, err := entity.NewInsight(
		"user-1",
		valueobject.InsightTypeOvertraining,
		valueobject.SeverityCritical,
		"ACWR is 2.1",
		time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if i.CreatedAt.IsZero() {
		t.Fatal("expected CreatedAt to be set")
	}
}
