package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/usecase"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

func TestRecordMetric_Success(t *testing.T) {
	repo := newMockMetricRepo()
	pub := &mockEventPublisher{}
	uc := usecase.NewRecordMetric(repo, pub)

	input := usecase.RecordMetricInput{
		UserID:     "user-1",
		MetricType: "hrv",
		Value:      65.0,
		Date:       time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
	}

	output, err := uc.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if output.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if repo.saveCalled != 1 {
		t.Fatalf("expected save called once, got %d", repo.saveCalled)
	}
	if len(pub.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(pub.events))
	}
}

func TestRecordMetric_InvalidMetricType(t *testing.T) {
	repo := newMockMetricRepo()
	pub := &mockEventPublisher{}
	uc := usecase.NewRecordMetric(repo, pub)

	input := usecase.RecordMetricInput{
		UserID:     "user-1",
		MetricType: "invalid",
		Value:      65.0,
		Date:       time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
	}

	_, err := uc.Execute(context.Background(), input)
	if err != valueobject.ErrInvalidMetricType {
		t.Fatalf("expected ErrInvalidMetricType, got %v", err)
	}
}

func TestRecordMetric_OutOfRange(t *testing.T) {
	repo := newMockMetricRepo()
	pub := &mockEventPublisher{}
	uc := usecase.NewRecordMetric(repo, pub)

	input := usecase.RecordMetricInput{
		UserID:     "user-1",
		MetricType: "hrv",
		Value:      -10,
		Date:       time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
	}

	_, err := uc.Execute(context.Background(), input)
	if err != entity.ErrMetricOutOfRange {
		t.Fatalf("expected ErrMetricOutOfRange, got %v", err)
	}
}
