package handler

import (
	"strconv"

	"github.com/gofiber/fiber/v2"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
)

// defaultNotificationLimit é o número de registros retornados quando o cliente
// não informa ?limit.
const defaultNotificationLimit = 50

// maxNotificationLimit limita o tamanho da página para proteger o backend.
const maxNotificationLimit = 200

// NotificationHandler expõe o histórico de notificações do usuário autenticado.
type NotificationHandler struct {
	history port.NotificationRepository
}

func NewNotificationHandler(history port.NotificationRepository) *NotificationHandler {
	return &NotificationHandler{history: history}
}

type notificationDTO struct {
	ID        string `json:"id"`
	InsightID string `json:"insight_id"`
	Type      string `json:"type"`
	Severity  string `json:"severity"`
	Title     string `json:"title"`
	Body      string `json:"body"`
	Status    string `json:"status"`
	Error     string `json:"error,omitempty"`
	CreatedAt string `json:"created_at"`
}

// List retorna o histórico de notificações do usuário autenticado, do mais
// recente para o mais antigo.
func (h *NotificationHandler) List(c *fiber.Ctx) error {
	userID := userIDFromCtx(c)

	limit := defaultNotificationLimit
	if v := c.Query("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			return writeError(c, fiber.StatusBadRequest, "invalid 'limit' query param")
		}
		limit = n
	}
	if limit > maxNotificationLimit {
		limit = maxNotificationLimit
	}

	records, err := h.history.ListByUser(c.UserContext(), userID, limit)
	if err != nil {
		return writeError(c, fiber.StatusInternalServerError, "internal error")
	}

	out := make([]notificationDTO, 0, len(records))
	for _, r := range records {
		out = append(out, notificationDTO{
			ID:        r.ID,
			InsightID: r.InsightID,
			Type:      r.Type,
			Severity:  r.Severity,
			Title:     r.Title,
			Body:      r.Body,
			Status:    r.Status,
			Error:     r.Error,
			CreatedAt: r.CreatedAt.UTC().Format("2006-01-02T15:04:05Z07:00"),
		})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"notifications": out,
		"count":         len(out),
	})
}
