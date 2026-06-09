package port

import "context"

// ProviderTokenRepository persiste os tokens OAuth dos provedores externos por
// usuário e provedor. A camada de aplicação nunca conhece o backend concreto.
type ProviderTokenRepository interface {
	// Save grava (ou atualiza) o token do usuário para o provedor contido em token.Provider.
	Save(ctx context.Context, userID string, token ProviderToken) error
	// Find retorna o token do usuário para o provedor, ou (nil, nil) se ausente.
	Find(ctx context.Context, userID, provider string) (*ProviderToken, error)
	// FindUserByAthlete resolve o GAH userID a partir do athlete id de um provedor
	// (ex.: o owner_id de um webhook da Strava). found=false (sem erro) quando não
	// há vínculo conhecido para aquele (provider, athleteID).
	FindUserByAthlete(ctx context.Context, provider, athleteID string) (userID string, found bool, err error)
}
