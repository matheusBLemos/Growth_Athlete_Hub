package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/application/usecase"
)

func TestSyncProviderActivities_Success(t *testing.T) {
	tokenRepo := newMockProviderTokenRepo()
	tokenRepo.tokens["user-1:strava"] = &port.ProviderToken{
		Provider:     "strava",
		AccessToken:  "acc",
		RefreshToken: "ref",
		ExpiresAt:    time.Now().Add(time.Hour),
	}
	gw := &mockProviderGateway{
		activities: []port.ProviderActivity{
			{Provider: "strava", ExternalID: "1", Type: "running", StartTime: time.Now().Add(-time.Hour), Duration: 30 * time.Minute},
			{Provider: "strava", ExternalID: "2", Type: "cycling", StartTime: time.Now().Add(-2 * time.Hour), Duration: time.Hour},
		},
	}
	pub := &mockEventPublisher{}

	uc := usecase.NewSyncProviderActivities(gw, tokenRepo, pub)
	out, err := uc.Execute(context.Background(), usecase.SyncProviderActivitiesInput{UserID: "user-1", Provider: "strava"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Count != 2 {
		t.Fatalf("count = %d, want 2", out.Count)
	}
	if len(pub.events) != 2 {
		t.Fatalf("events = %d, want 2", len(pub.events))
	}
	if pub.events[0].Type != "raw.activity.imported" {
		t.Fatalf("event type = %q", pub.events[0].Type)
	}
	payload, ok := pub.events[0].Payload.(usecase.RawActivityImported)
	if !ok {
		t.Fatalf("payload type = %T", pub.events[0].Payload)
	}
	if payload.UserID != "user-1" || payload.Provider != "strava" || payload.ExternalID != "1" {
		t.Fatalf("payload = %+v", payload)
	}
	if gw.refreshCalled {
		t.Fatal("refresh should not be called for a valid token")
	}
}

func TestSyncProviderActivities_RefreshesExpiredToken(t *testing.T) {
	tokenRepo := newMockProviderTokenRepo()
	tokenRepo.tokens["user-1:strava"] = &port.ProviderToken{
		Provider:     "strava",
		AccessToken:  "old",
		RefreshToken: "ref",
		ExpiresAt:    time.Now().Add(-time.Hour), // expired
	}
	gw := &mockProviderGateway{
		refreshed: port.ProviderToken{Provider: "strava", AccessToken: "new", RefreshToken: "ref2", ExpiresAt: time.Now().Add(time.Hour)},
		activities: []port.ProviderActivity{
			{Provider: "strava", ExternalID: "1", Type: "running", StartTime: time.Now().Add(-time.Hour), Duration: 30 * time.Minute},
		},
	}
	pub := &mockEventPublisher{}

	uc := usecase.NewSyncProviderActivities(gw, tokenRepo, pub)
	out, err := uc.Execute(context.Background(), usecase.SyncProviderActivitiesInput{UserID: "user-1", Provider: "strava"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !gw.refreshCalled {
		t.Fatal("expected refresh to be called")
	}
	if gw.usedAccessToken != "new" {
		t.Fatalf("fetch used token %q, want new", gw.usedAccessToken)
	}
	if tokenRepo.saveCalled != 1 {
		t.Fatalf("save called %d times, want 1 (persist refreshed token)", tokenRepo.saveCalled)
	}
	if saved := tokenRepo.tokens["user-1:strava"]; saved.AccessToken != "new" {
		t.Fatalf("persisted token = %q, want new", saved.AccessToken)
	}
	if out.Count != 1 {
		t.Fatalf("count = %d, want 1", out.Count)
	}
}

func TestSyncProviderActivities_NoToken(t *testing.T) {
	tokenRepo := newMockProviderTokenRepo()
	gw := &mockProviderGateway{}
	pub := &mockEventPublisher{}

	uc := usecase.NewSyncProviderActivities(gw, tokenRepo, pub)
	_, err := uc.Execute(context.Background(), usecase.SyncProviderActivitiesInput{UserID: "user-1", Provider: "strava"})
	if !errors.Is(err, usecase.ErrProviderNotConnected) {
		t.Fatalf("expected ErrProviderNotConnected, got %v", err)
	}
}

func TestSyncProviderActivities_FetchError(t *testing.T) {
	tokenRepo := newMockProviderTokenRepo()
	tokenRepo.tokens["user-1:strava"] = &port.ProviderToken{Provider: "strava", AccessToken: "acc", ExpiresAt: time.Now().Add(time.Hour)}
	gw := &mockProviderGateway{fetchErr: errors.New("boom")}
	pub := &mockEventPublisher{}

	uc := usecase.NewSyncProviderActivities(gw, tokenRepo, pub)
	_, err := uc.Execute(context.Background(), usecase.SyncProviderActivitiesInput{UserID: "user-1", Provider: "strava"})
	if err == nil {
		t.Fatal("expected fetch error")
	}
}

func TestSyncProviderActivities_PublishError(t *testing.T) {
	tokenRepo := newMockProviderTokenRepo()
	tokenRepo.tokens["user-1:strava"] = &port.ProviderToken{Provider: "strava", AccessToken: "acc", ExpiresAt: time.Now().Add(time.Hour)}
	gw := &mockProviderGateway{
		activities: []port.ProviderActivity{{Provider: "strava", ExternalID: "1", Type: "running", StartTime: time.Now().Add(-time.Hour), Duration: time.Minute}},
	}
	pub := &mockEventPublisher{err: errors.New("broker down")}

	uc := usecase.NewSyncProviderActivities(gw, tokenRepo, pub)
	_, err := uc.Execute(context.Background(), usecase.SyncProviderActivitiesInput{UserID: "user-1", Provider: "strava"})
	if err == nil {
		t.Fatal("expected publish error to propagate")
	}
}
