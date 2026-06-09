package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
)

// defaultFCMBaseURL é o endpoint base do FCM HTTP v1. O caminho completo de
// envio é {base}/v1/projects/{projectID}/messages:send. Sobrescritível para
// testes com httptest.
const defaultFCMBaseURL = "https://fcm.googleapis.com"

// fcmScope é o OAuth2 scope exigido pela API FCM HTTP v1.
const fcmScope = "https://www.googleapis.com/auth/firebase.messaging"

// FCMConfig carrega os parâmetros do provedor de push FCM HTTP v1. A BaseURL e o
// TokenSource são sobrescritíveis para testes (httptest + token estático),
// evitando qualquer chamada real ao Google.
type FCMConfig struct {
	// BaseURL é o endpoint base do FCM (vazio = endpoint real).
	BaseURL string
	// ProjectID é o ID do projeto Firebase/GCP (compõe a URL de envio).
	ProjectID string
	// TokenSource fornece o bearer token OAuth2. Em produção vem da
	// service-account; em testes, um oauth2.StaticTokenSource.
	TokenSource oauth2.TokenSource
}

// FCMNotifier implementa port.Notifier usando a API FCM HTTP v1. A autenticação
// é feita via bearer token OAuth2 (service-account), e a BaseURL/TokenSource são
// injetáveis para testes sem rede viva nem credenciais reais.
type FCMNotifier struct {
	cfg        FCMConfig
	httpClient *http.Client
}

var _ port.Notifier = (*FCMNotifier)(nil)

// NewFCMNotifier constrói o notifier aplicando o endpoint padrão para BaseURL
// vazia. O TokenSource deve ser fornecido (use NewServiceAccountTokenSource em
// produção ou um oauth2.StaticTokenSource em testes).
func NewFCMNotifier(cfg FCMConfig) *FCMNotifier {
	if strings.TrimSpace(cfg.BaseURL) == "" {
		cfg.BaseURL = defaultFCMBaseURL
	}
	cfg.BaseURL = strings.TrimRight(cfg.BaseURL, "/")
	return &FCMNotifier{
		cfg:        cfg,
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

// NewServiceAccountTokenSource cria um oauth2.TokenSource a partir do JSON da
// service-account no caminho informado, com o scope do FCM. Usado na wiring de
// produção (cmd/worker).
func NewServiceAccountTokenSource(ctx context.Context, credentialsFile string) (oauth2.TokenSource, error) {
	data, err := os.ReadFile(credentialsFile)
	if err != nil {
		return nil, fmt.Errorf("read fcm credentials file: %w", err)
	}
	jwtCfg, err := google.JWTConfigFromJSON(data, fcmScope)
	if err != nil {
		return nil, fmt.Errorf("parse fcm service account: %w", err)
	}
	return jwtCfg.TokenSource(ctx), nil
}

// fcmV1Request é o envelope da API FCM HTTP v1: {"message": {...}}.
type fcmV1Request struct {
	Message fcmV1Message `json:"message"`
}

type fcmV1Message struct {
	Token        string            `json:"token"`
	Notification fcmV1Notification `json:"notification"`
	Data         map[string]string `json:"data,omitempty"`
}

type fcmV1Notification struct {
	Title string `json:"title"`
	Body  string `json:"body"`
}

func (n *FCMNotifier) Send(ctx context.Context, notif port.Notification) error {
	payload := fcmV1Request{
		Message: fcmV1Message{
			Token: notif.Token,
			Notification: fcmV1Notification{
				Title: notif.Title,
				Body:  notif.Body,
			},
			Data: notif.Data,
		},
	}

	body, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("marshal fcm payload: %w", err)
	}

	if n.cfg.TokenSource == nil {
		return fmt.Errorf("fcm notifier: nil token source")
	}
	tok, err := n.cfg.TokenSource.Token()
	if err != nil {
		return fmt.Errorf("obtain fcm access token: %w", err)
	}

	url := fmt.Sprintf("%s/v1/projects/%s/messages:send", n.cfg.BaseURL, n.cfg.ProjectID)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("build fcm request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+tok.AccessToken)

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
