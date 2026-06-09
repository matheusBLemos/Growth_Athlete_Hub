package usecase

import "errors"

var (
	ErrDuplicateActivity    = errors.New("activity with this external ID already exists")
	ErrEmailAlreadyExists   = errors.New("email already registered")
	ErrInvalidCredentials   = errors.New("invalid email or password")
	ErrProviderNotConnected = errors.New("provider not connected for this user")
)
