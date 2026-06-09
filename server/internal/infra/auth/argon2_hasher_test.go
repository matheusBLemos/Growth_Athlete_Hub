package auth_test

import (
	"testing"

	"github.com/Growth-Athlete-Hub/gah-server/internal/infra/auth"
)

func TestArgon2Hasher_HashAndCompare(t *testing.T) {
	h := auth.NewArgon2Hasher("pepper-secret")

	hash, err := h.Hash("s3cret-pass")
	if err != nil {
		t.Fatalf("hash failed: %v", err)
	}
	if hash == "" {
		t.Fatal("expected non-empty hash")
	}
	if hash == "s3cret-pass" {
		t.Fatal("hash must not equal plaintext")
	}

	if err := h.Compare(hash, "s3cret-pass"); err != nil {
		t.Fatalf("expected match, got %v", err)
	}
}

func TestArgon2Hasher_WrongPassword(t *testing.T) {
	h := auth.NewArgon2Hasher("pepper-secret")
	hash, err := h.Hash("s3cret-pass")
	if err != nil {
		t.Fatalf("hash failed: %v", err)
	}
	if err := h.Compare(hash, "wrong"); err == nil {
		t.Fatal("expected mismatch error")
	}
}

func TestArgon2Hasher_SaltMakesHashesDistinct(t *testing.T) {
	h := auth.NewArgon2Hasher("pepper-secret")
	a, _ := h.Hash("same")
	b, _ := h.Hash("same")
	if a == b {
		t.Fatal("expected distinct hashes due to random salt")
	}
}

func TestArgon2Hasher_PepperMatters(t *testing.T) {
	h1 := auth.NewArgon2Hasher("pepper-A")
	h2 := auth.NewArgon2Hasher("pepper-B")

	hash, err := h1.Hash("s3cret-pass")
	if err != nil {
		t.Fatalf("hash failed: %v", err)
	}
	// Mesmo hash, pepper diferente => não deve validar.
	if err := h2.Compare(hash, "s3cret-pass"); err == nil {
		t.Fatal("expected mismatch when pepper differs")
	}
}

func TestArgon2Hasher_MalformedHash(t *testing.T) {
	h := auth.NewArgon2Hasher("pepper-secret")
	if err := h.Compare("not-a-valid-phc-string", "whatever"); err == nil {
		t.Fatal("expected error for malformed hash")
	}
}
