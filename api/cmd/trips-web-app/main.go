package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"strconv"

	"golang.org/x/sync/errgroup"

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
	logger := zerolog.New(os.Stdout).Level(zerolog.InfoLevel).With().
		Timestamp().
		Str("app", "trips-sandbox-app").
		Logger()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

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

	ac := controllers.NewAccountController(&settings, &logger)
	vc := controllers.NewVehiclesController(&settings, &logger)
	tc := controllers.NewTripsController(&settings, &logger)
	st := controllers.NewStreamrController(&settings, &logger)
	sc := controllers.NewSettingsController(&settings, &logger)

	app := fiber.New(fiber.Config{
		ErrorHandler:          ErrorHandler,
		Views:                 engine,
		ReadBufferSize:        16000,
		DisableStartupMessage: true,
	})
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "https://localdev.dimo.org:3008", // localhost development
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowCredentials: true,
	}))

	// View routes (protected)
	app.Get("/account", controllers.AuthMiddleware(), ac.MyAccount)
	app.Get("/vehicles/me", controllers.AuthMiddleware(), vc.HandleGetVehicles)
	app.Get("/vehicles/:tokenid/signals", controllers.AuthMiddleware(), vc.HandleVehicleSignals)
	app.Get("/vehicles/:tokenid/history", vc.HandleGetHistoricalData)

	app.Get("/vehicles/:tokenid/trips", controllers.AuthMiddleware(), tc.HandleTripsList)
	app.Get("/give-feedback", controllers.AuthMiddleware(), vc.HandleGiveFeedback(&settings))
	app.Get("/streamr", controllers.AuthMiddleware(), st.GetStreamr)

	// API routes called via Javascript fetch
	app.Get("/api/trip/:tripID", controllers.AuthMiddleware(), func(c *fiber.Ctx) error {
		tripID := c.Params("tripID")
		startTime := c.Query("start")
		endTime := c.Query("end")

		logger.Debug().Msgf("Received request for tripID: %s, startTime: %s, endTime: %s", tripID, startTime, endTime)

		// Retrieve the estimated start location from the query string
		var estimatedStart *controllers.LatLon
		if estimatedStartStr := c.Query("estimatedStart"); estimatedStartStr != "" {
			logger.Debug().Msgf("Estimated start location: %s", estimatedStartStr)
			if err := json.Unmarshal([]byte(estimatedStartStr), &estimatedStart); err != nil {
				log.Error().Err(err).Msg("Invalid estimated start location")
				logger.Err(err).Msgf(
					"Invalid estimated start location: %s",
					estimatedStartStr,
				)
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid estimated start location"})
			}
		} else {
			logger.Debug().Msgf("No estimatedStart received for tripID: %s", tripID)
		}

		return tc.HandleMapDataForTrip(c, &settings, tripID, startTime, endTime, estimatedStart)
	})
	// used by /web frontend in lit for the login
	app.Get("/v1/public/settings", sc.GetPublicSettings)

	// Public Routes
	app.Post("/auth/web3/generate_challenge", func(c *fiber.Ctx) error {
		return controllers.HandleGenerateChallenge(c, &settings)
	})
	app.Post("/auth/web3/submit_challenge", func(c *fiber.Ctx) error {
		return controllers.HandleSubmitChallenge(c, &settings)
	})
	app.Post("/auth/start_session", controllers.PersistJwtHandler)

	app.Post("/api/generate-token/:tokenID", controllers.AuthMiddleware(), func(c *fiber.Ctx) error {
		tokenID, err := strconv.ParseInt(c.Params("tokenID"), 10, 64)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid token ID"})
		}

		token, err := controllers.RequestPriviledgeToken(c, &settings, tokenID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to generate privilege token", "details": err.Error()})
		}

		return c.JSON(fiber.Map{"token": *token})
	})

	// to hold any static css or js
	app.Static("/static", "./static", fiber.Static{
		Compress:      true,
		Download:      false,
		Index:         "",
		CacheDuration: 0,
		MaxAge:        0,
	})

	// used load react compiled login app
	app.Get("/", loadStaticIndex)

	// host the compiled frontend for the web3 login, which should be built to the dist folder
	staticConfig := fiber.Static{
		Compress: true,
		MaxAge:   0,
		Index:    "index.html",
	}
	app.Static("/", "./dist", staticConfig)

	app.Get("/health", healthCheck)

	group, gCtx := errgroup.WithContext(ctx)

	log.Info().Msgf("Starting server on port %s", settings.Port)
	runFiber(gCtx, app, ":"+settings.Port, group, settings.UseDevCerts)

	if err := group.Wait(); err != nil {
		logger.Fatal().Err(err).Msg("Server failed.")
	}
	logger.Info().Msg("Server stopped.")
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

func runFiber(ctx context.Context, fiberApp *fiber.App, addr string, group *errgroup.Group, useTLS bool) {
	group.Go(func() error {
		if useTLS {
			if err := fiberApp.ListenTLS("localdev.dimo.org"+addr, "../web/.mkcert/cert.pem", "../web/.mkcert/dev.pem"); err != nil {
				return fmt.Errorf("failed to start server: %w", err)
			}
		} else {
			if err := fiberApp.Listen(addr); err != nil {
				return fmt.Errorf("failed to start server: %w", err)
			}
		}
		return nil
	})
	group.Go(func() error {
		<-ctx.Done()
		if err := fiberApp.Shutdown(); err != nil {
			return fmt.Errorf("failed to shutdown server: %w", err)
		}
		return nil
	})
}
