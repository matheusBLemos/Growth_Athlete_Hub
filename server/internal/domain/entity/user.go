package entity

import (
	"crypto/rand"
	"fmt"
	"strings"
	"time"
)

type User struct {
	ID        string
	Name      string
	Email     string
	BirthDate time.Time
	CreatedAt time.Time
}

func NewUser(name, email string, birthDate time.Time) (*User, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil, ErrEmptyName
	}
	if !isValidEmail(email) {
		return nil, ErrInvalidEmail
	}
	if birthDate.IsZero() {
		return nil, ErrBirthDateFuture
	}
	if birthDate.After(time.Now()) {
		return nil, ErrBirthDateFuture
	}

	return &User{
		ID:        generateID(),
		Name:      name,
		Email:     strings.ToLower(strings.TrimSpace(email)),
		BirthDate: birthDate,
		CreatedAt: time.Now(),
	}, nil
}

func isValidEmail(email string) bool {
	if email == "" || strings.Contains(email, " ") {
		return false
	}
	parts := strings.SplitN(email, "@", 2)
	if len(parts) != 2 {
		return false
	}
	local, domain := parts[0], parts[1]
	if local == "" || domain == "" {
		return false
	}
	if !strings.Contains(domain, ".") {
		return false
	}
	return true
}

func generateID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		panic("crypto/rand failed: " + err.Error())
	}
	return fmt.Sprintf("%x", b)
}
