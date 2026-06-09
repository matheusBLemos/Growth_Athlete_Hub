package port

import "context"

// Notification é o envelope de uma notificação push a ser entregue a um
// dispositivo específico. Carrega o token de destino, o conteúdo e dados
// estruturados opcionais (ex.: insight_id para deep-link no app).
type Notification struct {
	// UserID é o destinatário lógico (auditoria/log; não usado no envio em si).
	UserID string
	// Token é o registration token do dispositivo de destino (FCM/APNs).
	Token string
	// Title é o título curto exibido na notificação.
	Title string
	// Body é o corpo da notificação.
	Body string
	// Data carrega pares chave-valor extras entregues ao app (deep-link, ids).
	Data map[string]string
}

// Notifier é o contrato para envio de notificações push. A camada de aplicação
// nunca conhece o provedor concreto (FCM, log-stub, etc.).
type Notifier interface {
	// Send entrega a notificação ao dispositivo de n.Token. Um erro indica
	// falha no envio daquele dispositivo específico.
	Send(ctx context.Context, n Notification) error
}
