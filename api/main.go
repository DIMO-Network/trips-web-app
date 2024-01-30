package main

import (
	"encoding/json"
	"fmt"
	"github.com/DIMO-Network/shared"
	"github.com/dimo-network/trips-web-app-new/api/api/internal/config"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
)

type ChallengeResponse struct {
	State     string `json:"state"`
	Challenge string `json:"challenge"`
}

func setupRoutes(app *fiber.App, settings *config.Settings) {
	app.Post("/auth/web3/generate_challenge", func(c *fiber.Ctx) error {
		return HandleGenerateChallenge(c, settings)
	})
	app.Post("/auth/web3/submit_challenge", func(c *fiber.Ctx) error {
		return HandleSubmitChallenge(c, settings)
	})
}

func HandleGenerateChallenge(c *fiber.Ctx, settings *config.Settings) error {
	address := c.FormValue("address")

	formData := url.Values{}
	formData.Add("client_id", settings.ClientID)
	formData.Add("domain", settings.Domain)
	formData.Add("scope", settings.Scope)
	formData.Add("response_type", settings.ResponseType)
	formData.Add("address", address)

	encodedFormData := formData.Encode()
	reqURL := settings.AuthURL

	resp, err := http.Post(reqURL, "application/x-www-form-urlencoded", strings.NewReader(encodedFormData))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to make request to external service")
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error reading external response")
	}

	var apiResp ChallengeResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error processing response from external service")
	}

	if apiResp.State == "" || apiResp.Challenge == "" {
		return c.Status(fiber.StatusInternalServerError).SendString("State or Challenge incomplete from external service")
	}

	return c.JSON(apiResp)
}

func HandleSubmitChallenge(c *fiber.Ctx, settings *config.Settings) error {
	state := c.FormValue("state")
	signature := c.FormValue("signature")

	log.Info().Msgf("State: %s, Signature: %s", state, signature)

	formData := url.Values{}
	formData.Add("client_id", settings.ClientID)
	formData.Add("domain", settings.Domain)
	formData.Add("grant_type", settings.GrantType)
	formData.Add("state", state)
	formData.Add("signature", signature)

	encodedFormData := formData.Encode()
	reqURL := settings.SubmitChallengeURL

	resp, err := http.Post(reqURL, "application/x-www-form-urlencoded", strings.NewReader(encodedFormData))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to make request to external service")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to read response from external service")
	}

	c.Set("Content-Type", "application/json")
	return c.Send(respBody)
}

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

	settings, err := shared.LoadConfig[config.Settings]("settings.yaml")
	if err != nil {
		log.Fatal().Err(err).Msg("could not load settings")
	}

	level, err := zerolog.ParseLevel(settings.LogLevel)
	if err != nil {
		log.Fatal().Err(err).Msgf("could not parse LOG_LEVEL: %s", settings.LogLevel)
	}
	zerolog.SetGlobalLevel(level)

	app := fiber.New(fiber.Config{
		ErrorHandler: ErrorHandler,
	})

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("Hello, Fiber is running!")
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:3000",
		AllowMethods: "GET,POST,HEAD,PUT,DELETE,PATCH",
		AllowHeaders: "Accept, Content-Type, Content-Length, Authorization",
	}))

	setupRoutes(app, &settings)

	log.Info().Msgf("Starting server on port %s", settings.Port)
	if err := app.Listen(":" + settings.Port); err != nil {
		log.Fatal().Err(err).Msg("Server failed to start")
	}
}
