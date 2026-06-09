package usecase_test

import (
	"context"
	"testing"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/usecase"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/valueobject"
)

func TestRegisterActivity_Success(t *testing.T) {
	repo := newMockActivityRepo()
	pub := &mockEventPublisher{}
	uc := usecase.NewRegisterActivity(repo, pub)

	input := usecase.RegisterActivityInput{
		UserID:       "user-1",
		ActivityType: "running",
		Date:         time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC),
		Duration:     30 * time.Minute,
		AvgHeartRate: 150,
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
		t.Fatalf("expected 1 event published, got %d", len(pub.events))
	}
	if pub.events[0].Type != "activity.registered" {
		t.Fatalf("expected event type activity.registered, got %s", pub.events[0].Type)
	}
}

func TestRegisterActivity_InvalidActivityType(t *testing.T) {
	repo := newMockActivityRepo()
	pub := &mockEventPublisher{}
	uc := usecase.NewRegisterActivity(repo, pub)

	input := usecase.RegisterActivityInput{
		UserID:       "user-1",
		ActivityType: "flying",
		Date:         time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC),
		Duration:     30 * time.Minute,
		AvgHeartRate: 150,
	}

	_, err := uc.Execute(context.Background(), input)
	if err != valueobject.ErrInvalidActivityType {
		t.Fatalf("expected ErrInvalidActivityType, got %v", err)
	}
	if repo.saveCalled != 0 {
		t.Fatal("expected save not called")
	}
}

func TestRegisterActivity_DuplicateExternalID(t *testing.T) {
	repo := newMockActivityRepo()
	pub := &mockEventPublisher{}
	uc := usecase.NewRegisterActivity(repo, pub)

	input := usecase.RegisterActivityInput{
		UserID:       "user-1",
		ActivityType: "running",
		Date:         time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC),
		Duration:     30 * time.Minute,
		AvgHeartRate: 150,
		ExternalID:   "strava-123",
	}

	_, err := uc.Execute(context.Background(), input)
	if err != nil {
		t.Fatalf("first call should succeed, got %v", err)
	}

	_, err = uc.Execute(context.Background(), input)
	if err != usecase.ErrDuplicateActivity {
		t.Fatalf("expected ErrDuplicateActivity, got %v", err)
	}
}

func TestRegisterActivity_InvalidDomain(t *testing.T) {
	repo := newMockActivityRepo()
	pub := &mockEventPublisher{}
	uc := usecase.NewRegisterActivity(repo, pub)

	input := usecase.RegisterActivityInput{
		UserID:       "user-1",
		ActivityType: "running",
		Date:         time.Date(2025, 6, 1, 8, 0, 0, 0, time.UTC),
		Duration:     -10 * time.Minute,
		AvgHeartRate: 150,
	}

	_, err := uc.Execute(context.Background(), input)
	if err == nil {
		t.Fatal("expected error for negative duration")
	}
}
