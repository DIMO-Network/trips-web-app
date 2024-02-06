package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/DIMO-Network/shared"
	"github.com/dimo-network/trips-web-app-new/api/api/internal/config"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/template/handlebars/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

var cacheInstance = cache.New(cache.DefaultExpiration, 10*time.Minute)

type ChallengeResponse struct {
	State     string `json:"state"`
	Challenge string `json:"challenge"`
}

type GraphQLRequest struct {
	Query string `json:"query"`
}

type VehicleResponse struct {
	Data struct {
		Vehicles struct {
			Nodes []Vehicle `json:"nodes"`
		} `json:"vehicles"`
	} `json:"data"`
}

type Vehicle struct {
	ID    string `json:"id"`
	Make  string `json:"make"`
	Model string `json:"model"`
	Year  int    `json:"year"`
}

const EthereumAddressKey = "ethereum_address"

func HandleGetVehicles(c *fiber.Ctx, settings *config.Settings) error {
	// Retrieve the user's eth address from c.Locals
	ethAddress, ok := c.Locals(EthereumAddressKey).(string)
	if !ok || ethAddress == "" {
		return c.Status(fiber.StatusBadRequest).SendString("Ethereum address is required")
	}

	// Query identity-api
	vehicles, err := queryIdentityAPIForVehicles(ethAddress, settings)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error querying identity API: " + err.Error())
	}

	return c.Render("vehicles", fiber.Map{
		"Title":    "My Vehicles",
		"Vehicles": vehicles,
	})
}

func queryIdentityAPIForVehicles(ethAddress string, settings *config.Settings) ([]Vehicle, error) {
	// GraphQL query
	graphqlQuery := `{
        vehicles(first: 10, filterBy: { owner: "` + ethAddress + `" }) {
            nodes {
                tokenId,
                earnings {
                    totalTokens
                },
                definition {
                    make,
                    model,
                    year
                },
                aftermarketDevice {
                    address,
                    serial,
                    manufacturer {
                        name
                    }
                }
            }
        }
    }`

	// GraphQL request
	requestPayload := GraphQLRequest{Query: graphqlQuery}
	payloadBytes, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, err
	}

	// POST request
	req, err := http.NewRequest("POST", settings.IdentityAPIURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var response VehicleResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, err
	}

	return response.Data.Vehicles.Nodes, nil
}

func setupRoutes(app *fiber.App, settings *config.Settings) {
	app.Post("/auth/web3/generate_challenge", func(c *fiber.Ctx) error {
		return HandleGenerateChallenge(c, settings)
	})
	app.Post("/auth/web3/submit_challenge", func(c *fiber.Ctx) error {
		return HandleSubmitChallenge(c, settings)
	})
	app.Get("/vehicles/me", AuthMiddleware(cacheInstance), ExtractEthereumAddress, func(c *fiber.Ctx) error {
		return HandleGetVehicles(c, settings)
	})
}

func AuthMiddleware(cache *cache.Cache) fiber.Handler {
	return func(c *fiber.Ctx) error {
		sessionCookie := c.Cookies("session_id")
		token, found := cache.Get(sessionCookie)
		if !found {
			return c.Status(fiber.StatusUnauthorized).SendString("Unauthorized")
		}

		// Parse the token to get the claims
		jwtToken, err := jwt.Parse(token.(string), nil)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Error parsing JWT")
		}

		// Set the jwt token in c.Locals
		c.Locals("user", jwtToken)
		return c.Next()
	}
}

func ExtractEthereumAddress(c *fiber.Ctx) error {
	// Retrieve the jwt token from c.Locals
	token := c.Locals("user").(*jwt.Token)
	claims := token.Claims.(jwt.MapClaims)
	ethereumAddress, ok := claims["ethereum_address"].(string)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).SendString("Ethereum address not found in JWT")
	}

	c.Locals("ethereum_address", ethereumAddress)
	return c.Next()
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

	// Check the HTTP status code here
	if resp.StatusCode >= 300 {
		return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Received non-success status code: %d", resp.StatusCode))
	}

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to read response from external service")
	}

	var responseMap map[string]interface{}
	if err := json.Unmarshal(respBody, &responseMap); err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error processing response")
	}

	log.Info().Msgf("Response from submit challenge: %+v", responseMap) //debugging

	token, exists := responseMap["access_token"]
	if !exists {
		return c.Status(fiber.StatusInternalServerError).SendString("Token not found in response")
	}

	sessionID := uuid.New().String()
	cacheInstance.Set(sessionID, token, 2*time.Hour)

	cookie := new(fiber.Cookie)
	cookie.Name = "session_id"
	cookie.Value = sessionID
	cookie.Expires = time.Now().Add(2 * time.Hour)
	cookie.HTTPOnly = true

	c.Cookie(cookie)

	return c.JSON(fiber.Map{"message": "Challenge accepted and session started!", "access_token": token})
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

	engine := handlebars.New("../views", ".hbs")

	app := fiber.New(fiber.Config{
		ErrorHandler: ErrorHandler,
		Views:        engine,
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:3000",
		AllowMethods: "GET,POST,HEAD,PUT,DELETE,PATCH",
		AllowHeaders: "Accept, Content-Type, Content-Length, Authorization",
	}))

	/*
		handleGetVehiclesWithSettings := func(c *fiber.Ctx) error {
			return HandleGetVehicles(c, &settings)
		}
	*/

	app.Get("/", func(c *fiber.Ctx) error {
		return c.Render("vehicles", fiber.Map{
			"Title": "My Vehicles",
		})
	})

	//app.Get("/vehicles/me", AuthMiddleware(cacheInstance), ExtractEthereumAddress, handleGetVehiclesWithSettings)
	setupRoutes(app, &settings)

	log.Info().Msgf("Starting server on port %s", settings.Port)
	if err := app.Listen(":" + settings.Port); err != nil {
		log.Fatal().Err(err).Msg("Server failed to start")
	}
}
