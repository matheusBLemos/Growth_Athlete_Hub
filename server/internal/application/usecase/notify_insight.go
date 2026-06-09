package usecase

import (
	"context"
	"errors"
	"fmt"
	"log"
	"strings"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
)

// NotifyInsight é a use case do módulo de Notificações: dado um insight gerado
// (decodificado do evento insight.generated), localiza os dispositivos do
// usuário e dispara uma notificação push para cada um.
//
// Estratégia de falha multi-dispositivo: a falha de envio para um dispositivo
// NÃO aborta os demais — todos são tentados e os erros são acumulados. Se algum
// envio falhou, Execute retorna um erro (via errors.Join), o que faz o handler
// nack-ar a mensagem para redelivery/dead-letter. Usuário sem dispositivos é um
// no-op de sucesso (ack idempotente).
type NotifyInsight struct {
	devices  port.DeviceRepository
	notifier port.Notifier
	// history persiste o histórico de envios. Opcional: nil desativa a
	// persistência (mantém compatibilidade com wiring/testes existentes).
	history port.NotificationRepository
}

func NewNotifyInsight(devices port.DeviceRepository, notifier port.Notifier) *NotifyInsight {
	return &NotifyInsight{devices: devices, notifier: notifier}
}

// WithHistory injeta (opcionalmente) o repositório de histórico. Retorna a
// própria use case para encadeamento na wiring. history nil é um no-op.
func (uc *NotifyInsight) WithHistory(history port.NotificationRepository) *NotifyInsight {
	uc.history = history
	return uc
}

func (uc *NotifyInsight) Execute(ctx context.Context, insight InsightGenerated) error {
	devices, err := uc.devices.FindByUser(ctx, insight.UserID)
	if err != nil {
		return fmt.Errorf("find devices for user %s: %w", insight.UserID, err)
	}
	if len(devices) == 0 {
		// Usuário sem dispositivos registrados: nada a fazer (ack).
		return nil
	}

	title := insightTitle(insight)
	data := map[string]string{
		"insight_id": insight.InsightID,
		"type":       insight.Type,
		"severity":   insight.Severity,
	}

	var errs []error
	for _, d := range devices {
		notif := port.Notification{
			UserID: insight.UserID,
			Token:  d.Token,
			Title:  title,
			Body:   insight.Message,
			Data:   data,
		}
		sendErr := uc.notifier.Send(ctx, notif)
		if sendErr != nil {
			// Não aborta os demais: acumula e segue.
			log.Printf("notify_insight: send to device (user=%s insight=%s): %v", insight.UserID, insight.InsightID, sendErr)
			errs = append(errs, sendErr)
		}
		uc.recordHistory(ctx, insight, title, sendErr)
	}

	if len(errs) > 0 {
		return fmt.Errorf("notify insight %s: %w", insight.InsightID, errors.Join(errs...))
	}
	return nil
}

// recordHistory persiste um registro de histórico do envio (sent/failed). Uma
// falha de escrita NÃO aborta a entrega: apenas loga. No-op quando history nil.
func (uc *NotifyInsight) recordHistory(ctx context.Context, insight InsightGenerated, title string, sendErr error) {
	if uc.history == nil {
		return
	}
	rec := port.NotificationRecord{
		UserID:    insight.UserID,
		InsightID: insight.InsightID,
		Type:      insight.Type,
		Severity:  insight.Severity,
		Title:     title,
		Body:      insight.Message,
		Status:    port.NotificationStatusSent,
	}
	if sendErr != nil {
		rec.Status = port.NotificationStatusFailed
		rec.Error = sendErr.Error()
	}
	if err := uc.history.Save(ctx, rec); err != nil {
		log.Printf("notify_insight: persist history (user=%s insight=%s): %v", insight.UserID, insight.InsightID, err)
	}
}

// insightTitle deriva um título curto a partir do tipo/severidade do insight.
func insightTitle(insight InsightGenerated) string {
	typ := strings.TrimSpace(insight.Type)
	if typ == "" {
		typ = "Insight"
	}
	sev := strings.TrimSpace(insight.Severity)
	if sev == "" {
		return fmt.Sprintf("Novo insight: %s", typ)
	}
	return fmt.Sprintf("%s: %s", strings.ToUpper(sev[:1])+sev[1:], typ)
}
