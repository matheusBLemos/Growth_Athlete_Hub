package auth_test

import (
	"testing"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/auth"
)

func TestJWTIssuer_RoundTrip(t *testing.T) {
	issuer := auth.NewJWTIssuer("super-secret", time.Hour)

	token, err := issuer.Issue("user-123")
	if err != nil {
		t.Fatalf("issue failed: %v", err)
	}
	if token == "" {
		t.Fatal("expected non-empty token")
	}

	userID, err := issuer.Parse(token)
	if err != nil {
		t.Fatalf("parse failed: %v", err)
	}
	if userID != "user-123" {
		t.Fatalf("expected user-123, got %q", userID)
	}
}

func TestJWTIssuer_Expired(t *testing.T) {
	issuer := auth.NewJWTIssuer("super-secret", -time.Minute)

	token, err := issuer.Issue("user-123")
	if err != nil {
		t.Fatalf("issue failed: %v", err)
	}

	if _, err := issuer.Parse(token); err == nil {
		t.Fatal("expected error for expired token")
	}
}

func TestJWTIssuer_InvalidSignature(t *testing.T) {
	signer := auth.NewJWTIssuer("secret-A", time.Hour)
	verifier := auth.NewJWTIssuer("secret-B", time.Hour)

	token, err := signer.Issue("user-123")
	if err != nil {
		t.Fatalf("issue failed: %v", err)
	}

	if _, err := verifier.Parse(token); err == nil {
		t.Fatal("expected error for invalid signature")
	}
}

func TestJWTIssuer_Garbage(t *testing.T) {
	issuer := auth.NewJWTIssuer("secret", time.Hour)
	if _, err := issuer.Parse("not.a.jwt"); err == nil {
		t.Fatal("expected error for garbage token")
	}
}
