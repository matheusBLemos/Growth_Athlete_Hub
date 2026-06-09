package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"fmt"
	"strings"

	"golang.org/x/crypto/argon2"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
)

var _ port.PasswordHasher = (*Argon2Hasher)(nil)

// ErrInvalidHash indica um hash em formato inesperado/corrompido.
var ErrInvalidHash = errors.New("invalid argon2id hash format")

// ErrMismatchedHash indica que a senha não corresponde ao hash.
var ErrMismatchedHash = errors.New("password does not match hash")

// Parâmetros recomendados para Argon2id (OWASP).
const (
	argonTime    = 1
	argonMemory  = 64 * 1024 // 64 MiB
	argonThreads = 4
	argonKeyLen  = 32
	argonSaltLen = 16
)

// Argon2Hasher implementa port.PasswordHasher usando Argon2id + pepper.
// O pepper é um segredo da aplicação concatenado à senha antes do hashing;
// ao contrário do salt, ele não é armazenado junto ao hash.
type Argon2Hasher struct {
	pepper string
}

func NewArgon2Hasher(pepper string) *Argon2Hasher {
	return &Argon2Hasher{pepper: pepper}
}

func (h *Argon2Hasher) Hash(plain string) (string, error) {
	salt := make([]byte, argonSaltLen)
	if _, err := rand.Read(salt); err != nil {
		return "", fmt.Errorf("generating salt: %w", err)
	}

	key := argon2.IDKey([]byte(plain+h.pepper), salt, argonTime, argonMemory, argonThreads, argonKeyLen)

	// Formato PHC: $argon2id$v=19$m=...,t=...,p=...$salt$hash
	encoded := fmt.Sprintf(
		"$argon2id$v=%d$m=%d,t=%d,p=%d$%s$%s",
		argon2.Version,
		argonMemory, argonTime, argonThreads,
		base64.RawStdEncoding.EncodeToString(salt),
		base64.RawStdEncoding.EncodeToString(key),
	)
	return encoded, nil
}

func (h *Argon2Hasher) Compare(hash, plain string) error {
	salt, key, mem, time, threads, keyLen, err := decodeHash(hash)
	if err != nil {
		return err
	}

	candidate := argon2.IDKey([]byte(plain+h.pepper), salt, time, mem, threads, keyLen)
	if subtle.ConstantTimeCompare(key, candidate) != 1 {
		return ErrMismatchedHash
	}
	return nil
}

func decodeHash(encoded string) (salt, key []byte, mem, time uint32, threads uint8, keyLen uint32, err error) {
	parts := strings.Split(encoded, "$")
	if len(parts) != 6 || parts[1] != "argon2id" {
		return nil, nil, 0, 0, 0, 0, ErrInvalidHash
	}

	var version int
	if _, err = fmt.Sscanf(parts[2], "v=%d", &version); err != nil {
		return nil, nil, 0, 0, 0, 0, ErrInvalidHash
	}
	if version != argon2.Version {
		return nil, nil, 0, 0, 0, 0, ErrInvalidHash
	}

	if _, err = fmt.Sscanf(parts[3], "m=%d,t=%d,p=%d", &mem, &time, &threads); err != nil {
		return nil, nil, 0, 0, 0, 0, ErrInvalidHash
	}

	salt, err = base64.RawStdEncoding.DecodeString(parts[4])
	if err != nil {
		return nil, nil, 0, 0, 0, 0, ErrInvalidHash
	}

	key, err = base64.RawStdEncoding.DecodeString(parts[5])
	if err != nil {
		return nil, nil, 0, 0, 0, 0, ErrInvalidHash
	}

	return salt, key, mem, time, threads, uint32(len(key)), nil
}
