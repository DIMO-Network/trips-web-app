package controllers

import (
	"github.com/dimo-network/trips-web-app/api/internal/config"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

type SettingsController struct {
	settings *config.Settings
	logger   *zerolog.Logger
}

func NewSettingsController(settings *config.Settings, logger *zerolog.Logger) *SettingsController {
	return &SettingsController{
		settings: settings,
		logger:   logger,
	}
}

// GetSettings
// @Summary Get private configuration parameters
// @Description Get config params for frontend app
// @Tags Settings
// @Produce json
// @Success 200
// @Security     BearerAuth
// @Router /v1/settings [get]
func (v *SettingsController) GetSettings(c *fiber.Ctx) error {

	payload := SettingsResponse{
		Environment: v.settings.Environment,
	}

	return c.JSON(payload)
}

func (v *SettingsController) GetPublicSettings(c *fiber.Ctx) error {
	payload := PublicSettingsResponse{
		ClientID: v.settings.ClientID,
		LoginURL: v.settings.LoginURL.String(),
	}
	return c.JSON(payload)
}

type SettingsResponse struct {
	Environment string `json:"environment"`
}

type PublicSettingsResponse struct {
	ClientID string `json:"clientId"`
	LoginURL string `json:"loginUrl"`
}
