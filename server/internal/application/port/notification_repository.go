package port

import (
	"context"
	"time"
)

// Status de um registro de histórico de notificação.
const (
	// NotificationStatusSent indica que o envio ao dispositivo teve sucesso.
	NotificationStatusSent = "sent"
	// NotificationStatusFailed indica que o envio ao dispositivo falhou.
	NotificationStatusFailed = "failed"
)

// NotificationRecord é um registro de histórico de uma notificação tentada para
// um dispositivo do usuário. Persiste tanto envios bem-sucedidos quanto falhos,
// para auditoria e exibição no app.
type NotificationRecord struct {
	// ID é o identificador único do registro.
	ID string
	// UserID é o destinatário lógico da notificação.
	UserID string
	// InsightID é o insight que originou a notificação (vazio se não aplicável).
	InsightID string
	// Type é o tipo do insight (ex.: "recovery").
	Type string
	// Severity é a severidade do insight (ex.: "warning").
	Severity string
	// Title é o título exibido na notificação.
	Title string
	// Body é o corpo da notificação.
	Body string
	// Status é "sent" ou "failed".
	Status string
	// Error carrega a mensagem de erro quando Status == failed (vazio caso contrário).
	Error string
	// CreatedAt é o instante de criação do registro.
	CreatedAt time.Time
}

// NotificationRepository persiste o histórico de notificações enviadas/falhas. A
// camada de aplicação nunca conhece o backend concreto.
type NotificationRepository interface {
	// Save grava um registro de histórico de notificação.
	Save(ctx context.Context, record NotificationRecord) error
	// ListByUser retorna os registros mais recentes do usuário (limit > 0).
	ListByUser(ctx context.Context, userID string, limit int) ([]NotificationRecord, error)
}
