package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
)

var _ port.TokenIssuer = (*JWTIssuer)(nil)

// ErrInvalidToken indica um token ausente, expirado, malformado ou com assinatura inválida.
var ErrInvalidToken = errors.New("invalid token")

// JWTIssuer emite e valida tokens JWT assinados com HS256.
type JWTIssuer struct {
	secret []byte
	ttl    time.Duration
}

func NewJWTIssuer(secret string, ttl time.Duration) *JWTIssuer {
	return &JWTIssuer{
		secret: []byte(secret),
		ttl:    ttl,
	}
}

func (j *JWTIssuer) Issue(userID string) (string, error) {
	now := time.Now()
	claims := jwt.RegisteredClaims{
		Subject:   userID,
		IssuedAt:  jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(j.ttl)),
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(j.secret)
}

func (j *JWTIssuer) Parse(tokenStr string) (string, error) {
	claims := &jwt.RegisteredClaims{}
	token, err := jwt.ParseWithClaims(tokenStr, claims, func(t *jwt.Token) (any, error) {
		// Garante que só aceitamos HS256 (evita ataque de troca de algoritmo).
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return j.secret, nil
	})
	if err != nil || !token.Valid {
		return "", ErrInvalidToken
	}
	if claims.Subject == "" {
		return "", ErrInvalidToken
	}
	return claims.Subject, nil
}
