package main

import (
	"fmt"
	"os"
	"strconv"

	"github.com/DIMO-Network/shared"
	"github.com/dimo-network/trips-web-app/api/internal/config"
	"github.com/dimo-network/trips-web-app/api/internal/controllers"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/template/handlebars/v2"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func ErrorHandler(ctx *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	message := "Internal Server Error"

	var e *fiber.Error
	if errors.As(err, &e) {
		code = e.Code
		message = e.Message
	}

	log.Error().Err(err).Int("code", code).Str("path", ctx.Path()).Msg("Error occurred")

	return ctx.Status(code).JSON(fiber.Map{
		"error":   true,
		"message": message,
	})
}

func main() {
	log.Logger = log.Output(zerolog.ConsoleWriter{Out: os.Stderr})

	fmt.Print("Server is starting...")

	settings, err := shared.LoadConfig[config.Settings]("settings.prod.yaml")
	if err != nil {
		log.Fatal().Err(err).Msg("could not load settings")
	}

	level, err := zerolog.ParseLevel(settings.LogLevel)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not parse LOG_LEVEL: %s", settings.LogLevel)
	}
	zerolog.SetGlobalLevel(level)

	engine := handlebars.New("./views", ".hbs")

	app := fiber.New(fiber.Config{
		ErrorHandler: ErrorHandler,
		Views:        engine,
	})

	app.Use(cors.New())

	// Protected route
	app.Get("/vehicles/me", controllers.AuthMiddleware(), func(c *fiber.Ctx) error {

		return controllers.HandleVehiclesAndFeedbackData(c, &settings)
	})

	app.Get("/account", controllers.AuthMiddleware(), func(c *fiber.Ctx) error {
		sessionCookie := c.Cookies("session_id")
		if sessionCookie == "" {
			fmt.Println("No session_id cookie")
			return c.Render("session_expired", fiber.Map{})
		}

		// check if the session_id is in the cache
		jwtToken, found := controllers.CacheInstance.Get(sessionCookie)
		if !found {
			fmt.Println("Session expired")
			return c.Render("session_expired", fiber.Map{})
		}

		ethAddress := c.Locals("ethereum_address").(string)

		vehicles, err := controllers.QueryIdentityAPIForVehicles(ethAddress, &settings)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Error querying identity API: " + err.Error())
		}

		if len(vehicles) == 0 {
			vehicles, err = controllers.QuerySharedVehicles(ethAddress, &settings)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).SendString("Error querying shared vehicles: " + err.Error())
			}
		}

		privilegeToken, err := controllers.RequestPriviledgeToken(c, &settings, vehicles[0].TokenID)

		if err != nil {
			return c.Render("session_expired", fiber.Map{})
		}

		return c.Render("account", fiber.Map{
			"Token":          jwtToken,
			"PrivilegeToken": privilegeToken,
			"Privileges": fiber.Map{
				"1": "1: All-time, non-location data",
				"4": "4: All-time location",
			},
		})
	})

	// Device status route
	app.Get("/vehicles/:tokenid/status", controllers.AuthMiddleware(), func(c *fiber.Ctx) error {
		tokenID, err := strconv.ParseInt(c.Params("tokenid"), 10, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid token ID",
			})
		}

		rawDeviceStatus, err := controllers.QueryDeviceDataAPI(tokenID, &settings, c)
		if err != nil {
			log.Error().Err(err).Msg("Failed to query device data API")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to fetch device status",
			})
		}

		deviceStatus := controllers.ProcessRawDeviceStatus(rawDeviceStatus)

		return c.Render("vehicle_status", fiber.Map{
			"TokenID":             tokenID,
			"DeviceStatusEntries": deviceStatus,
			"Privileges":          []any{},
		})
	})

	// Device trips route
	app.Get("/vehicles/:tokenid/trips", controllers.AuthMiddleware(), func(c *fiber.Ctx) error {
		tokenID, err := strconv.ParseInt(c.Params("tokenid"), 10, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
				"error": "Invalid token ID",
			})
		}

		trips, err := controllers.QueryTripsAPI(tokenID, &settings, c)
		if err != nil {
			log.Error().Err(err).Msg("Failed to query trips API")
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "Failed to fetch trips",
			})
		}

		return c.Render("vehicle_trips", fiber.Map{
			"TokenID": tokenID,
			"Trips":   trips,
		})
	})

	app.Get("/api/trip/:tripID", controllers.AuthMiddleware(), func(c *fiber.Ctx) error {
		tripID := c.Params("tripID")
		startTime := c.Query("start")
		endTime := c.Query("end")

		return controllers.HandleMapDataForTrip(c, &settings, tripID, startTime, endTime)
	})

	// Public Routes
	app.Post("/auth/web3/generate_challenge", func(c *fiber.Ctx) error {
		return controllers.HandleGenerateChallenge(c, &settings)
	})
	app.Post("/auth/web3/submit_challenge", func(c *fiber.Ctx) error {
		return controllers.HandleSubmitChallenge(c, &settings)
	})

	app.Get("/", loadStaticIndex)

	// host the compiled frontend for the web3 login, which should be built to the dist folder
	staticConfig := fiber.Static{
		Compress: true,
		MaxAge:   0,
		Index:    "index.html",
	}
	app.Static("/", "./dist", staticConfig)

	app.Get("/health", healthCheck)

	log.Info().Msgf("Starting server on port %s", settings.Port)
	if err := app.Listen(":" + settings.Port); err != nil {
		log.Fatal().Err(err).Msg("Server failed to start")
	}
}

func healthCheck(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"code":    200,
		"message": "server is up",
	})
}

func loadStaticIndex(ctx *fiber.Ctx) error {
	dat, err := os.ReadFile("dist/index.html")
	if err != nil {
		return err
	}
	ctx.Set("Content-Type", "text/html; charset=utf-8")
	return ctx.Status(fiber.StatusOK).Send(dat)
}
