package controllers

import (
	"github.com/dimo-network/trips-web-app/api/internal/config"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

type StreamrController struct {
	settings *config.Settings
}

func NewStreamrController(settings *config.Settings) StreamrController {
	return StreamrController{settings: settings}
}

func (tc *StreamrController) GetStreamr(c *fiber.Ctx) error {
	ethAddress := c.Locals("ethereum_address").(string)

	vehicles, err := QueryIdentityAPIForVehicles(ethAddress, tc.settings)
	if err != nil {
		log.Printf("Error querying My Vehicles: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Error querying my vehicles: " + err.Error())
	}

	sharedVehicles, err := QuerySharedVehicles(ethAddress, tc.settings)
	if err != nil {
		log.Printf("Error querying Shared Vehicles: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Error querying shared vehicles: " + err.Error())
	}
	return c.Render("streamr_live", fiber.Map{
		"Title":          "Streamr Live",
		"Vehicles":       vehicles,
		"SharedVehicles": sharedVehicles,
	})
}
