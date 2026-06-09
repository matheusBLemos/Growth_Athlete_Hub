// Package notifications contém os adaptadores e consumidores do módulo de
// Notificações do GAH: implementações de port.Notifier (log-stub, FCM/HTTP) e o
// handler que consome insight.generated e dispara as notificações push.
package notifications

import (
	"context"
	"log"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
)

// LogNotifier é a implementação padrão segura de port.Notifier: apenas registra
// a notificação no log, sem nenhuma chamada externa. É usada quando nenhum
// provedor de push está configurado.
type LogNotifier struct{}

var _ port.Notifier = (*LogNotifier)(nil)

func NewLogNotifier() *LogNotifier {
	return &LogNotifier{}
}

func (n *LogNotifier) Send(_ context.Context, notif port.Notification) error {
	log.Printf("notifications(log): user=%s token=%s title=%q body=%q data=%v",
		notif.UserID, notif.Token, notif.Title, notif.Body, notif.Data)
	return nil
}
