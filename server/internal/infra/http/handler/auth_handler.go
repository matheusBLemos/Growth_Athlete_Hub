package handler

import (
	"errors"
	"time"

	"github.com/gofiber/fiber/v2"

	"github.com/Growth-Athlete-Hub/gah-server/internal/application/usecase"
	"github.com/Growth-Athlete-Hub/gah-server/internal/domain/entity"
)

type AuthHandler struct {
	registerUser *usecase.RegisterUser
	loginUser    *usecase.LoginUser
}

func NewAuthHandler(register *usecase.RegisterUser, login *usecase.LoginUser) *AuthHandler {
	return &AuthHandler{
		registerUser: register,
		loginUser:    login,
	}
}

type registerUserRequest struct {
	Name      string `json:"name"`
	Email     string `json:"email"`
	Password  string `json:"password"`
	BirthDate string `json:"birth_date"`
}

func (h *AuthHandler) Register(c *fiber.Ctx) error {
	var req registerUserRequest
	if err := c.BodyParser(&req); err != nil {
		return writeError(c, fiber.StatusBadRequest, "invalid request body")
	}

	birthDate, err := time.Parse(time.RFC3339, req.BirthDate)
	if err != nil {
		return writeError(c, fiber.StatusBadRequest, "invalid birth_date format, use RFC3339")
	}

	output, err := h.registerUser.Execute(c.UserContext(), usecase.RegisterUserInput{
		Name:      req.Name,
		Email:     req.Email,
		Password:  req.Password,
		BirthDate: birthDate,
	})
	if err != nil {
		if errors.Is(err, entity.ErrEmptyName) ||
			errors.Is(err, entity.ErrInvalidEmail) ||
			errors.Is(err, entity.ErrBirthDateFuture) ||
			errors.Is(err, entity.ErrEmptyPasswordHash) {
			return writeError(c, fiber.StatusUnprocessableEntity, err.Error())
		}
		if errors.Is(err, usecase.ErrEmailAlreadyExists) {
			return writeError(c, fiber.StatusConflict, err.Error())
		}
		return writeError(c, fiber.StatusInternalServerError, "internal error")
	}

	return c.Status(fiber.StatusCreated).JSON(fiber.Map{"id": output.ID})
}

type loginUserRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
	var req loginUserRequest
	if err := c.BodyParser(&req); err != nil {
		return writeError(c, fiber.StatusBadRequest, "invalid request body")
	}

	output, err := h.loginUser.Execute(c.UserContext(), usecase.LoginUserInput{
		Email:    req.Email,
		Password: req.Password,
	})
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidCredentials) {
			return writeError(c, fiber.StatusUnauthorized, err.Error())
		}
		return writeError(c, fiber.StatusInternalServerError, "internal error")
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"token": output.Token})
}
