package handler

import (
	"errors"

	"github.com/gofiber/fiber/v2"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
	"github.com/Growth-Athlete-Hub/gah-server/internal/application/usecase"
)

// StravaWebhookEventType é o Type publicado quando um evento de webhook da
// Strava é recebido. O módulo de Processamento (ou um worker) consome este
// evento para disparar a importação/sync da atividade afetada.
const StravaWebhookEventType = "strava.webhook.activity"

// StravaHandler expõe os endpoints HTTP do conector Strava: início da conexão
// OAuth, callback, sync manual e webhook (verificação + recepção de eventos).
type StravaHandler struct {
	connect   *usecase.ConnectProvider
	sync      *usecase.SyncProviderActivities
	publisher port.EventPublisher
	// issuer assina/valida o state OAuth, vinculando-o ao userID autenticado.
	issuer             port.TokenIssuer
	webhookVerifyToken string
}

func NewStravaHandler(
	connect *usecase.ConnectProvider,
	sync *usecase.SyncProviderActivities,
	publisher port.EventPublisher,
	issuer port.TokenIssuer,
	webhookVerifyToken string,
) *StravaHandler {
	return &StravaHandler{
		connect:            connect,
		sync:               sync,
		publisher:          publisher,
		issuer:             issuer,
		webhookVerifyToken: webhookVerifyToken,
	}
}

// Connect (auth-protected) gera um state assinado vinculado ao userID e
// redireciona (302) para a URL de autorização da Strava.
func (h *StravaHandler) Connect(c *fiber.Ctx) error {
	userID := userIDFromCtx(c)
	if userID == "" {
		return writeError(c, fiber.StatusUnauthorized, "unauthorized")
	}

	state, err := h.issuer.Issue(userID)
	if err != nil {
		return writeError(c, fiber.StatusInternalServerError, "internal error")
	}

	return c.Redirect(h.connect.AuthURL(state), fiber.StatusFound)
}

// Callback (público) valida o state assinado, troca o code por um token e o
// persiste para o usuário. A Strava redireciona o navegador para cá.
func (h *StravaHandler) Callback(c *fiber.Ctx) error {
	if errParam := c.Query("error"); errParam != "" {
		return writeError(c, fiber.StatusBadRequest, "authorization denied: "+errParam)
	}

	code := c.Query("code")
	if code == "" {
		return writeError(c, fiber.StatusBadRequest, "missing code")
	}

	state := c.Query("state")
	userID, err := h.issuer.Parse(state)
	if err != nil || userID == "" {
		return writeError(c, fiber.StatusBadRequest, "invalid state")
	}

	if err := h.connect.HandleCallback(c.UserContext(), usecase.HandleCallbackInput{UserID: userID, Code: code}); err != nil {
		return writeError(c, fiber.StatusBadGateway, "failed to connect provider")
	}

	// Dispara um sync inicial best-effort; falhas não invalidam a conexão.
	_, _ = h.sync.Execute(c.UserContext(), usecase.SyncProviderActivitiesInput{UserID: userID, Provider: h.connect.Provider()})

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "connected", "provider": h.connect.Provider()})
}

// Sync (auth-protected) dispara a sincronização das atividades do usuário atual.
func (h *StravaHandler) Sync(c *fiber.Ctx) error {
	userID := userIDFromCtx(c)
	if userID == "" {
		return writeError(c, fiber.StatusUnauthorized, "unauthorized")
	}

	out, err := h.sync.Execute(c.UserContext(), usecase.SyncProviderActivitiesInput{UserID: userID, Provider: h.connect.Provider()})
	if err != nil {
		if errors.Is(err, usecase.ErrProviderNotConnected) {
			return writeError(c, fiber.StatusBadRequest, "strava not connected")
		}
		return writeError(c, fiber.StatusBadGateway, "failed to sync activities")
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"imported": out.Count})
}

// WebhookVerify (público) responde à validação de subscription da Strava:
// ecoa hub.challenge quando hub.verify_token confere.
func (h *StravaHandler) WebhookVerify(c *fiber.Ctx) error {
	if c.Query("hub.verify_token") != h.webhookVerifyToken {
		return writeError(c, fiber.StatusForbidden, "invalid verify token")
	}
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"hub.challenge": c.Query("hub.challenge")})
}

// stravaWebhookEvent é o payload de evento da Strava (push de webhook).
type stravaWebhookEvent struct {
	ObjectType string `json:"object_type"`
	AspectType string `json:"aspect_type"`
	ObjectID   int64  `json:"object_id"`
	OwnerID    int64  `json:"owner_id"`
	EventTime  int64  `json:"event_time"`
}

// WebhookEvent (público) recebe os eventos da Strava. Para atividades novas,
// publica um evento bruto que aciona a importação/sync downstream.
func (h *StravaHandler) WebhookEvent(c *fiber.Ctx) error {
	var ev stravaWebhookEvent
	if err := c.BodyParser(&ev); err != nil {
		// A Strava espera 200 para não reenviar; ignoramos payloads inválidos.
		return c.SendStatus(fiber.StatusOK)
	}

	if ev.ObjectType == "activity" && (ev.AspectType == "create" || ev.AspectType == "update") {
		_ = h.publisher.Publish(c.UserContext(), port.Event{
			Type: StravaWebhookEventType,
			Payload: fiber.Map{
				"provider":    "strava",
				"object_type": ev.ObjectType,
				"aspect_type": ev.AspectType,
				"object_id":   ev.ObjectID,
				"owner_id":    ev.OwnerID,
				"event_time":  ev.EventTime,
			},
		})
	}

	return c.SendStatus(fiber.StatusOK)
}
