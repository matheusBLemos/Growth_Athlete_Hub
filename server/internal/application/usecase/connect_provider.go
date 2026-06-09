package usecase

import (
	"context"
	"fmt"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
)

type HandleCallbackInput struct {
	// UserID é o usuário autenticado, recuperado do state OAuth assinado.
	UserID string
	// Code é o authorization code retornado pelo provedor.
	Code string
}

// ConnectProvider cobre o início da conexão OAuth (AuthURL) e o tratamento do
// callback: troca o código por um token e o persiste para o usuário.
type ConnectProvider struct {
	gateway   port.ProviderGateway
	tokenRepo port.ProviderTokenRepository
}

func NewConnectProvider(gw port.ProviderGateway, tokenRepo port.ProviderTokenRepository) *ConnectProvider {
	return &ConnectProvider{gateway: gw, tokenRepo: tokenRepo}
}

// AuthURL retorna a URL de autorização do provedor com o state embutido.
func (uc *ConnectProvider) AuthURL(state string) string {
	return uc.gateway.AuthURL(state)
}

// Provider retorna o identificador do provedor associado a este use case.
func (uc *ConnectProvider) Provider() string {
	return uc.gateway.Provider()
}

// HandleCallback troca o code por um token e o persiste para o usuário.
func (uc *ConnectProvider) HandleCallback(ctx context.Context, input HandleCallbackInput) error {
	token, err := uc.gateway.ExchangeCode(ctx, input.Code)
	if err != nil {
		return fmt.Errorf("exchange code: %w", err)
	}
	if token.Provider == "" {
		token.Provider = uc.gateway.Provider()
	}
	if err := uc.tokenRepo.Save(ctx, input.UserID, token); err != nil {
		return fmt.Errorf("persist provider token: %w", err)
	}
	return nil
}
