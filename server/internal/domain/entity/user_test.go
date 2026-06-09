package entity_test

import (
	"testing"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
)

func TestNewUser_Valid(t *testing.T) {
	u, err := entity.NewUser("Matheus", "matheus@email.com", time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if u.ID == "" {
		t.Fatal("expected non-empty ID")
	}
	if u.Name != "Matheus" {
		t.Fatalf("expected name Matheus, got %s", u.Name)
	}
	if u.Email != "matheus@email.com" {
		t.Fatalf("expected email matheus@email.com, got %s", u.Email)
	}
}

func TestNewUser_EmptyName(t *testing.T) {
	_, err := entity.NewUser("", "matheus@email.com", time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != entity.ErrEmptyName {
		t.Fatalf("expected ErrEmptyName, got %v", err)
	}
}

func TestNewUser_InvalidEmail(t *testing.T) {
	cases := []string{"", "invalid", "missing@", "@domain.com", "no spaces@email.com"}
	for _, email := range cases {
		t.Run(email, func(t *testing.T) {
			_, err := entity.NewUser("Matheus", email, time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
			if err != entity.ErrInvalidEmail {
				t.Fatalf("expected ErrInvalidEmail for %q, got %v", email, err)
			}
		})
	}
}

func TestNewUser_BirthDateInFuture(t *testing.T) {
	future := time.Now().AddDate(1, 0, 0)
	_, err := entity.NewUser("Matheus", "matheus@email.com", future)
	if err != entity.ErrBirthDateFuture {
		t.Fatalf("expected ErrBirthDateFuture, got %v", err)
	}
}

func TestNewUserWithCredentials_Valid(t *testing.T) {
	u, err := entity.NewUserWithCredentials("Matheus", "matheus@email.com", "argon2id-hash", time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if u.PasswordHash != "argon2id-hash" {
		t.Fatalf("expected password hash to be set, got %q", u.PasswordHash)
	}
}

func TestNewUserWithCredentials_EmptyHash(t *testing.T) {
	_, err := entity.NewUserWithCredentials("Matheus", "matheus@email.com", "  ", time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != entity.ErrEmptyPasswordHash {
		t.Fatalf("expected ErrEmptyPasswordHash, got %v", err)
	}
}

func TestNewUserWithCredentials_PropagatesValidation(t *testing.T) {
	_, err := entity.NewUserWithCredentials("", "matheus@email.com", "hash", time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != entity.ErrEmptyName {
		t.Fatalf("expected ErrEmptyName, got %v", err)
	}
}

func TestNewUser_CreatedAtIsSet(t *testing.T) {
	before := time.Now().Add(-time.Second)
	u, err := entity.NewUser("Matheus", "matheus@email.com", time.Date(2000, 1, 1, 0, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if u.CreatedAt.Before(before) {
		t.Fatal("expected CreatedAt to be recent")
	}
}
