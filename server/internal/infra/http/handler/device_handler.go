package handler

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/port"
)

// DeviceHandler expõe o registro de dispositivos de push do usuário autenticado.
type DeviceHandler struct {
	devices port.DeviceRepository
}

func NewDeviceHandler(devices port.DeviceRepository) *DeviceHandler {
	return &DeviceHandler{devices: devices}
}

type registerDeviceRequest struct {
	Token    string `json:"token"`
	Platform string `json:"platform"`
}

// Register grava o registration token de push do dispositivo para o usuário
// autenticado (upsert por token).
func (h *DeviceHandler) Register(c *fiber.Ctx) error {
	userID := userIDFromCtx(c)

	var req registerDeviceRequest
	if err := c.BodyParser(&req); err != nil {
		return writeError(c, fiber.StatusBadRequest, "invalid request body")
	}

	if strings.TrimSpace(req.Token) == "" {
		return writeError(c, fiber.StatusUnprocessableEntity, "token is required")
	}

	if err := h.devices.Save(c.Context(), userID, req.Token, req.Platform); err != nil {
		return writeError(c, fiber.StatusInternalServerError, "internal error")
	}

	return c.SendStatus(fiber.StatusNoContent)
}
