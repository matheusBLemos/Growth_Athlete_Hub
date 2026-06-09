package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
)

// defaultFCMBaseURL é o endpoint padrão de envio do FCM (legacy HTTP API).
// Sobrescritível para testes com httptest.
const defaultFCMBaseURL = "https://fcm.googleapis.com/fcm/send"

// FCMConfig carrega os parâmetros do provedor de push HTTP (FCM por padrão). A
// BaseURL é sobrescritível para testes; vazia cai no endpoint real do FCM.
type FCMConfig struct {
	// BaseURL é o endpoint de envio (vazio = endpoint real do FCM).
	BaseURL string
	// ServerKey é a chave de servidor usada no header de autorização.
	ServerKey string
}

// FCMNotifier implementa port.Notifier enviando a notificação para um endpoint
// HTTP de push (FCM por padrão) via net/http. Todas as chamadas usam uma BaseURL
// configurável, permitindo testes com httptest sem rede viva.
type FCMNotifier struct {
	cfg        FCMConfig
	httpClient *http.Client
}

var _ port.Notifier = (*FCMNotifier)(nil)

// NewFCMNotifier constrói o notifier aplicando o endpoint padrão para BaseURL vazia.
func NewFCMNotifier(cfg FCMConfig) *FCMNotifier {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = defaultFCMBaseURL
	}
	return &FCMNotifier{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// fcmRequest é o corpo enviado ao endpoint de push (forma FCM legacy).
type fcmRequest struct {
	To           string            `json:"to"`
	Notification fcmNotification   `json:"notification"`
	Data         map[string]string `json:"data,omitempty"`
}

type fcmNotification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

func (n *FCMNotifier) Send(ctx context.Context, notif port.Notification) error {
	payload := fcmRequest{
		To: notif.Token,
		Notification: fcmNotification{
			Title: notif.Title,
			Body:  notif.Body,
		},
		Data: notif.Data,
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal fcm payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, n.cfg.BaseURL, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build fcm request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "key="+n.cfg.ServerKey)

	resp, err := n.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("send fcm request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<16))
		return fmt.Errorf("fcm push failed: status %d: %s", resp.StatusCode, strings.TrimSpace(string(b)))
	}
	return nil
}
