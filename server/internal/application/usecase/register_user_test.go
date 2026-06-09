package usecase_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/usecase"
)

func TestRegisterUser_Success(t *testing.T) {
	repo := newMockUserRepo()
	hasher := &mockHasher{}
	pub := &mockEventPublisher{}
	uc := usecase.NewRegisterUser(repo, hasher, pub)

	out, err := uc.Execute(context.Background(), usecase.RegisterUserInput{
		Name:      "Matheus",
		Email:     "matheus@email.com",
		Password:  "s3cret-pass",
		BirthDate: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.ID == "" {
		t.Fatal("expected non-empty user id")
	}
	if repo.saveCalled != 1 {
		t.Fatalf("expected save to be called once, got %d", repo.saveCalled)
	}

	saved := repo.byID[out.ID]
	if saved == nil {
		t.Fatal("expected user persisted")
	}
	if saved.PasswordHash != "hashed:s3cret-pass" {
		t.Fatalf("expected hashed password, got %q", saved.PasswordHash)
	}
	if saved.PasswordHash == "s3cret-pass" {
		t.Fatal("plaintext password must never be stored")
	}
}

func TestRegisterUser_DuplicateEmail(t *testing.T) {
	repo := newMockUserRepo()
	uc := usecase.NewRegisterUser(repo, &mockHasher{}, &mockEventPublisher{})

	in := usecase.RegisterUserInput{
		Name:      "Matheus",
		Email:     "dup@email.com",
		Password:  "s3cret-pass",
		BirthDate: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
	}
	if _, err := uc.Execute(context.Background(), in); err != nil {
		t.Fatalf("first register failed: %v", err)
	}

	_, err := uc.Execute(context.Background(), in)
	if !errors.Is(err, usecase.ErrEmailAlreadyExists) {
		t.Fatalf("expected ErrEmailAlreadyExists, got %v", err)
	}
}

func TestRegisterUser_InvalidEmail(t *testing.T) {
	repo := newMockUserRepo()
	uc := usecase.NewRegisterUser(repo, &mockHasher{}, &mockEventPublisher{})

	_, err := uc.Execute(context.Background(), usecase.RegisterUserInput{
		Name:      "Matheus",
		Email:     "not-an-email",
		Password:  "s3cret-pass",
		BirthDate: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("expected validation error for invalid email")
	}
	if repo.saveCalled != 0 {
		t.Fatal("must not persist on validation failure")
	}
}

func TestRegisterUser_HasherFailurePropagates(t *testing.T) {
	repo := newMockUserRepo()
	hasher := &mockHasher{hashErr: errors.New("boom")}
	uc := usecase.NewRegisterUser(repo, hasher, &mockEventPublisher{})

	_, err := uc.Execute(context.Background(), usecase.RegisterUserInput{
		Name:      "Matheus",
		Email:     "matheus@email.com",
		Password:  "s3cret-pass",
		BirthDate: time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC),
	})
	if err == nil {
		t.Fatal("expected error when hasher fails")
	}
}
