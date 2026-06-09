package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/usecase"
)

func seedUser(t *testing.T, repo *mockUserRepo, hasher *mockHasher) string {
	t.Helper()
	reg := usecase.NewRegisterUser(repo, hasher, &mockEventPublisher{})
	out, err := reg.Execute(context.Background(), usecase.RegisterUserInput{
		Name:      "Matheus",
		Email:     "matheus@email.com",
		Password:  "s3cret-pass",
		BirthDate: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("seed failed: %v", err)
	}
	return out.ID
}

func TestLoginUser_Success(t *testing.T) {
	repo := newMockUserRepo()
	hasher := &mockHasher{}
	id := seedUser(t, repo, hasher)

	uc := usecase.NewLoginUser(repo, hasher, &mockTokenIssuer{})
	out, err := uc.Execute(context.Background(), usecase.LoginUserInput{
		Email:    "matheus@email.com",
		Password: "s3cret-pass",
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.Token != "token:"+id {
		t.Fatalf("expected token for user %s, got %q", id, out.Token)
	}
}

func TestLoginUser_WrongPassword(t *testing.T) {
	repo := newMockUserRepo()
	hasher := &mockHasher{}
	seedUser(t, repo, hasher)

	uc := usecase.NewLoginUser(repo, hasher, &mockTokenIssuer{})
	_, err := uc.Execute(context.Background(), usecase.LoginUserInput{
		Email:    "matheus@email.com",
		Password: "wrong",
	})
	if !errors.Is(err, usecase.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials, got %v", err)
	}
}

func TestLoginUser_UnknownEmail(t *testing.T) {
	repo := newMockUserRepo()
	uc := usecase.NewLoginUser(repo, &mockHasher{}, &mockTokenIssuer{})

	_, err := uc.Execute(context.Background(), usecase.LoginUserInput{
		Email:    "ghost@email.com",
		Password: "whatever",
	})
	if !errors.Is(err, usecase.ErrInvalidCredentials) {
		t.Fatalf("expected ErrInvalidCredentials (no leak), got %v", err)
	}
}
