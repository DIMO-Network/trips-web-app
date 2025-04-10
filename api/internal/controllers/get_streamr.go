package controllers

import (
	"github.com/dimo-network/trips-web-app/api/internal/config"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog"
)

type StreamrController struct {
	settings *config.Settings
	logger   *zerolog.Logger
}

func NewStreamrController(settings *config.Settings, logger *zerolog.Logger) StreamrController {
	return StreamrController{settings: settings, logger: logger}
}

func (tc *StreamrController) GetStreamr(c *fiber.Ctx) error {
	ethAddress := c.Locals("ethereum_address").(string)

	vehicles, err := QueryIdentityAPIForVehicles(ethAddress, tc.settings)
	if err != nil {
		tc.logger.Error().Err(err).Msg("Error querying My Vehicles")
		return c.Status(fiber.StatusInternalServerError).SendString("Error querying my vehicles: " + err.Error())
	}

	sharedVehicles, err := QuerySharedVehicles(ethAddress, tc.settings)
	if err != nil {
		tc.logger.Error().Err(err).Msg("Error querying Shared Vehicles")
		return c.Status(fiber.StatusInternalServerError).SendString("Error querying shared vehicles: " + err.Error())
	}
	return c.Render("streamr_live", fiber.Map{
		"Title":          "Streamr Live",
		"Vehicles":       vehicles,
		"SharedVehicles": sharedVehicles,
	})
}
