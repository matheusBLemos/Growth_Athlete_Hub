package port

import (
	"context"
	"time"
)

// ProviderToken representa as credenciais OAuth2 emitidas por um provedor
// externo (ex.: Strava). É um value object de transporte entre a camada de
// conectores e a persistência — não é uma entidade de domínio.
type ProviderToken struct {
	// Provider identifica o provedor externo (ex.: "strava").
	Provider string
	// AccessToken é o bearer token usado nas chamadas à API do provedor.
	AccessToken string
	// RefreshToken é usado para renovar o AccessToken quando ele expira.
	RefreshToken string
	// ExpiresAt é o instante de expiração do AccessToken.
	ExpiresAt time.Time
	// Scope são os escopos OAuth concedidos pelo provedor.
	Scope string
	// AthleteID é o identificador do atleta/usuário no provedor externo.
	AthleteID string
}

// Expired informa se o AccessToken já expirou (com uma margem de segurança
// para evitar usar um token prestes a expirar).
func (t ProviderToken) Expired(now time.Time) bool {
	if t.ExpiresAt.IsZero() {
		return false
	}
	const skew = 60 * time.Second
	return now.Add(skew).After(t.ExpiresAt)
}

// ProviderActivity é a forma canônica de uma atividade importada de um provedor
// externo, mapeada para os campos que o domínio do GAH conhece. A normalização
// acontece no adaptador do conector; o módulo de processamento (consumidor dos
// eventos) é quem persiste/deduplica.
type ProviderActivity struct {
	// Provider identifica o provedor de origem (ex.: "strava").
	Provider string
	// ExternalID é o ID da atividade no provedor (carregado como external_id).
	ExternalID string
	// Type é o tipo de atividade já mapeado para os tipos canônicos do GAH.
	Type string
	// StartTime é o instante de início da atividade.
	StartTime time.Time
	// Duration é a duração da atividade.
	Duration time.Duration
	// AvgHeartRate é a frequência cardíaca média (0 quando ausente).
	AvgHeartRate int
	// DistanceMeters é a distância percorrida em metros (0 quando ausente).
	DistanceMeters float64
	// Name é o título da atividade no provedor (informativo).
	Name string
}

// ProviderGateway abstrai a integração com um provedor externo de atividades,
// cobrindo OAuth2 (autorização, troca de código e refresh) e o consumo da API.
// É intencionalmente agnóstico ao provedor concreto.
type ProviderGateway interface {
	// AuthURL constrói a URL de autorização OAuth2, embutindo o state opaco.
	AuthURL(state string) string
	// ExchangeCode troca o authorization code por um ProviderToken.
	ExchangeCode(ctx context.Context, code string) (ProviderToken, error)
	// Refresh renova o token usando o refresh token.
	Refresh(ctx context.Context, refreshToken string) (ProviderToken, error)
	// FetchActivities busca as atividades do atleta a partir de `since`.
	FetchActivities(ctx context.Context, accessToken string, since time.Time) ([]ProviderActivity, error)
	// Provider retorna o identificador do provedor (ex.: "strava").
	Provider() string
}
