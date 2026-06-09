package port

import "context"

// Device representa um dispositivo registrado de um usuário para recebimento de
// notificações push, identificado pelo seu registration token.
type Device struct {
	// UserID é o dono do dispositivo.
	UserID string
	// Token é o registration token do dispositivo (FCM/APNs).
	Token string
	// Platform identifica a plataforma (ex.: "android", "ios", "web").
	Platform string
}

// DeviceRepository persiste os dispositivos de notificação por usuário. A
// camada de aplicação nunca conhece o backend concreto.
type DeviceRepository interface {
	// Save grava (ou atualiza, upsert por token) o dispositivo do usuário.
	Save(ctx context.Context, userID, token, platform string) error
	// FindByUser retorna todos os dispositivos registrados do usuário (vazio se nenhum).
	FindByUser(ctx context.Context, userID string) ([]Device, error)
}
