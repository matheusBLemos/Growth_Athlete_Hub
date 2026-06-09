package postgres

import (
	"context"
	"database/sql"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
)

var _ port.ProviderTokenRepository = (*ProviderTokenRepository)(nil)

type ProviderTokenRepository struct {
	db *sql.DB
}

func NewProviderTokenRepository(db *sql.DB) *ProviderTokenRepository {
	return &ProviderTokenRepository{db: db}
}

func (r *ProviderTokenRepository) Save(ctx context.Context, userID string, token port.ProviderToken) error {
	_, err := r.db.ExecContext(ctx,
		`INSERT INTO provider_tokens
		   (user_id, provider, access_token, refresh_token, expires_at, scope, athlete_id, created_at, updated_at)
		 VALUES ($1, $2, $3, $4, $5, $6, $7, NOW(), NOW())
		 ON CONFLICT (user_id, provider) DO UPDATE SET
		   access_token  = EXCLUDED.access_token,
		   refresh_token = EXCLUDED.refresh_token,
		   expires_at    = EXCLUDED.expires_at,
		   scope         = EXCLUDED.scope,
		   athlete_id    = EXCLUDED.athlete_id,
		   updated_at    = NOW()`,
		userID, token.Provider, token.AccessToken, token.RefreshToken,
		nullTime(token.ExpiresAt), token.Scope, token.AthleteID,
	)
	return err
}

func (r *ProviderTokenRepository) Find(ctx context.Context, userID, provider string) (*port.ProviderToken, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT provider, access_token, refresh_token, expires_at, scope, athlete_id
		 FROM provider_tokens WHERE user_id = $1 AND provider = $2`,
		userID, provider,
	)

	var t port.ProviderToken
	var expiresAt sql.NullTime
	err := row.Scan(&t.Provider, &t.AccessToken, &t.RefreshToken, &expiresAt, &t.Scope, &t.AthleteID)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if expiresAt.Valid {
		t.ExpiresAt = expiresAt.Time
	}
	return &t, nil
}

// FindUserByAthlete resolve o GAH userID pelo athlete id do provedor, usando o
// índice em (provider, athlete_id). Retorna found=false (sem erro) quando não
// há linha correspondente.
func (r *ProviderTokenRepository) FindUserByAthlete(ctx context.Context, provider, athleteID string) (string, bool, error) {
	row := r.db.QueryRowContext(ctx,
		`SELECT user_id FROM provider_tokens WHERE provider = $1 AND athlete_id = $2`,
		provider, athleteID,
	)

	var userID string
	err := row.Scan(&userID)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return userID, true, nil
}

func nullTime(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}
