package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/application/usecase"
)

func TestHandleOAuthCallback_Success(t *testing.T) {
	tokenRepo := newMockProviderTokenRepo()
	gw := &mockProviderGateway{
		exchanged: port.ProviderToken{Provider: "strava", AccessToken: "acc", RefreshToken: "ref", ExpiresAt: time.Now().Add(time.Hour)},
	}

	uc := usecase.NewConnectProvider(gw, tokenRepo)
	err := uc.HandleCallback(context.Background(), usecase.HandleCallbackInput{UserID: "user-1", Code: "code-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tokenRepo.saveCalled != 1 {
		t.Fatalf("save called %d, want 1", tokenRepo.saveCalled)
	}
	saved := tokenRepo.tokens["user-1:strava"]
	if saved == nil || saved.AccessToken != "acc" {
		t.Fatalf("token not persisted correctly: %+v", saved)
	}
}

func TestHandleOAuthCallback_ExchangeError(t *testing.T) {
	tokenRepo := newMockProviderTokenRepo()
	gw := &mockProviderGateway{exchangeErr: errors.New("bad code")}

	uc := usecase.NewConnectProvider(gw, tokenRepo)
	err := uc.HandleCallback(context.Background(), usecase.HandleCallbackInput{UserID: "user-1", Code: "x"})
	if err == nil {
		t.Fatal("expected exchange error")
	}
	if tokenRepo.saveCalled != 0 {
		t.Fatal("token should not be saved on exchange error")
	}
}

func TestConnectProvider_AuthURL(t *testing.T) {
	gw := &mockProviderGateway{}
	uc := usecase.NewConnectProvider(gw, newMockProviderTokenRepo())
	if got := uc.AuthURL("state-x"); got != "https://provider.test/authorize?state=state-x" {
		t.Fatalf("auth url = %q", got)
	}
}
