package usecase

import (
	"context"
	"fmt"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
)

// RawActivityImportedEventType é o Type publicado para cada atividade canônica
// importada de um provedor externo. O módulo de Processamento consome este
// evento para deduplicar, persistir e agregar.
const RawActivityImportedEventType = "raw.activity.imported"

// defaultLookback é a janela usada quando não há marca de "última sincronização".
const defaultLookback = 30 * 24 * time.Hour

// RawActivityImported é o payload do evento raw.activity.imported. É a forma
// canônica de uma atividade externa, enriquecida com userID e provider.
type RawActivityImported struct {
	UserID         string    `json:"user_id"`
	Provider       string    `json:"provider"`
	ExternalID     string    `json:"external_id"`
	Type           string    `json:"type"`
	StartTime      time.Time `json:"start_time"`
	DurationNs     int64     `json:"duration_ns"`
	AvgHeartRate   int       `json:"avg_heart_rate"`
	DistanceMeters float64   `json:"distance_meters"`
	Name           string    `json:"name"`
}

type SyncProviderActivitiesInput struct {
	UserID   string
	Provider string
	// Since opcional; quando zero, usa uma janela de lookback padrão.
	Since time.Time
}

type SyncProviderActivitiesOutput struct {
	Count int
}

// SyncProviderActivities carrega o token do usuário, renova-o se expirado,
// busca as atividades no provedor e publica um evento por atividade. Não
// persiste atividades — isso é responsabilidade do módulo de Processamento.
type SyncProviderActivities struct {
	gateway   port.ProviderGateway
	tokenRepo port.ProviderTokenRepository
	publisher port.EventPublisher
}

func NewSyncProviderActivities(gw port.ProviderGateway, tokenRepo port.ProviderTokenRepository, pub port.EventPublisher) *SyncProviderActivities {
	return &SyncProviderActivities{gateway: gw, tokenRepo: tokenRepo, publisher: pub}
}

func (uc *SyncProviderActivities) Execute(ctx context.Context, input SyncProviderActivitiesInput) (*SyncProviderActivitiesOutput, error) {
	provider := input.Provider
	if provider == "" {
		provider = uc.gateway.Provider()
	}

	token, err := uc.tokenRepo.Find(ctx, input.UserID, provider)
	if err != nil {
		return nil, fmt.Errorf("find provider token: %w", err)
	}
	if token == nil {
		return nil, ErrProviderNotConnected
	}

	if token.Expired(time.Now()) {
		refreshed, err := uc.gateway.Refresh(ctx, token.RefreshToken)
		if err != nil {
			return nil, fmt.Errorf("refresh provider token: %w", err)
		}
		if refreshed.Provider == "" {
			refreshed.Provider = provider
		}
		if err := uc.tokenRepo.Save(ctx, input.UserID, refreshed); err != nil {
			return nil, fmt.Errorf("persist refreshed token: %w", err)
		}
		token = &refreshed
	}

	since := input.Since
	if since.IsZero() {
		since = time.Now().Add(-defaultLookback)
	}

	activities, err := uc.gateway.FetchActivities(ctx, token.AccessToken, since)
	if err != nil {
		return nil, fmt.Errorf("fetch activities: %w", err)
	}

	for _, a := range activities {
		payload := RawActivityImported{
			UserID:         input.UserID,
			Provider:       a.Provider,
			ExternalID:     a.ExternalID,
			Type:           a.Type,
			StartTime:      a.StartTime,
			DurationNs:     a.Duration.Nanoseconds(),
			AvgHeartRate:   a.AvgHeartRate,
			DistanceMeters: a.DistanceMeters,
			Name:           a.Name,
		}
		if payload.Provider == "" {
			payload.Provider = provider
		}
		if err := uc.publisher.Publish(ctx, port.Event{Type: RawActivityImportedEventType, Payload: payload}); err != nil {
			return nil, fmt.Errorf("publish %s: %w", RawActivityImportedEventType, err)
		}
	}

	return &SyncProviderActivitiesOutput{Count: len(activities)}, nil
}
