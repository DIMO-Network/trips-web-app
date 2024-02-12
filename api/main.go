package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/DIMO-Network/shared"
	"github.com/dimo-network/trips-web-app/api/internal/config"
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

type Vehicle struct {
	TokenID  int64 `json:"tokenId"`
	Earnings struct {
		TotalTokens string `json:"totalTokens"`
	} `json:"earnings"`
	Definition struct {
		Make  string `json:"make"`
		Model string `json:"model"`
		Year  int    `json:"year"`
	} `json:"definition"`
	AftermarketDevice struct {
		Address      string `json:"address"`
		Serial       string `json:"serial"`
		Manufacturer struct {
			Name string `json:"name"`
		} `json:"manufacturer"`
	} `json:"aftermarketDevice"`
}

func ExtractEthereumAddressFromToken(tokenString string) (string, error) {
	// Parsing the token without validating its signature
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		fmt.Println("Error parsing token:", err)
		return "", fmt.Errorf("error parsing token")
	}

	// Asserting the type of the claims to access the data
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("invalid claims type")
	}

	ethAddress, ok := claims["ethereum_address"].(string)
	if !ok {
		return "", errors.New("ethereum address not found in JWT")
	}

	return ethAddress, nil
}

func AuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// Retrieve the session_id from the request cookie
		sessionCookie := c.Cookies("session_id")

		// Check if the session_id is in the cache
		jwtToken, found := cacheInstance.Get(sessionCookie)
		if !found {
			return c.Status(fiber.StatusUnauthorized).SendString("Unauthorized")
		}

		ethAddress, err := ExtractEthereumAddressFromToken(jwtToken.(string))
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).SendString("Invalid token: " + err.Error())
		}

		c.Locals("ethereum_address", ethAddress)

		return c.Next()
	}
}

func HandleGetVehicles(c *fiber.Ctx, settings *config.Settings) error {
	ethAddress := c.Locals("ethereum_address")

	vehicles, err := queryIdentityAPIForVehicles(ethAddress.(string), settings)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error querying identity API: " + err.Error())
	}

	// Debugging: Print the vehicles to check
	fmt.Printf("Vehicles: %+v\n", vehicles)

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

	log.Info().Msgf("this is the request query: %s", graphqlQuery)

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

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	fmt.Printf("Response body: %s\n", string(body))

	var vehicleResponse struct {
		Data struct {
			Vehicles struct {
				Nodes []Vehicle `json:"nodes"`
			} `json:"vehicles"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &vehicleResponse); err != nil {
		return nil, err
	}

	// Create a slice of Vehicles with the flattened structure for the template
	vehicles := make([]Vehicle, 0, len(vehicleResponse.Data.Vehicles.Nodes))
	for _, v := range vehicleResponse.Data.Vehicles.Nodes {
		vehicles = append(vehicles, Vehicle{
			TokenID:           v.TokenID,
			Earnings:          v.Earnings,
			Definition:        v.Definition,
			AftermarketDevice: v.AftermarketDevice,
		})
	}

	return vehicles, nil
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
	cookie.Domain = "localhost"

	c.Cookie(cookie)

	return c.JSON(fiber.Map{"message": "Challenge accepted and session started!", "access_token": token})
}

func HandleTokenExchange(c *fiber.Ctx, settings *config.Settings) error {

	ethAddress := c.Locals("ethereum_address").(string)
	vehicles, err := queryIdentityAPIForVehicles(ethAddress, settings)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query vehicles")
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to query vehicles")
	}
	if len(vehicles) == 0 {
		log.Error().Msg("No vehicles found for the given Ethereum address")
		return c.Status(fiber.StatusInternalServerError).SendString("No vehicles found")
	}
	tokenId := vehicles[0].TokenID

	log.Info().Msg("HandleTokenExchange called")

	sessionCookie := c.Cookies("session_id")
	log.Info().Msgf("Session Cookie: %s", sessionCookie)

	jwtToken, found := cacheInstance.Get(sessionCookie)
	if !found {
		log.Error().Msg("No session found")
		return c.Status(fiber.StatusUnauthorized).SendString("Unauthorized: No session found")
	}

	accessToken, ok := jwtToken.(string)
	if !ok {
		log.Error().Msg("Token format is invalid")
		return c.Status(fiber.StatusInternalServerError).SendString("Internal Error: Token format is invalid")
	}

	log.Info().Msgf("JWT being sent: %s", accessToken)

	nftContractAddress := "0x90C4D6113Ec88dd4BDf12f26DB2b3998fd13A144"
	privileges := []int{4} // example?
	requestBody := map[string]interface{}{
		"nftContractAddress": nftContractAddress,
		"privileges":         privileges,
		"tokenId":            tokenId,
	}

	requestBodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		log.Error().Err(err).Msg("Error marshaling request body")
		return c.Status(fiber.StatusInternalServerError).SendString("Error marshaling request body")
	}

	log.Info().Msgf("Request body being sent: %s", string(requestBodyBytes))

	req, err := http.NewRequest("POST", settings.TokenExchangeAPIURL, bytes.NewBuffer(requestBodyBytes))
	if err != nil {
		log.Error().Err(err).Msg("Error creating new request")
		return c.Status(fiber.StatusInternalServerError).SendString("Error creating new request")
	}

	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	//log raw req
	log.Info().Msgf("Request URL: %s", req.URL.String())
	log.Info().Msgf("Request Headers: %v", req.Header)
	log.Info().Msgf("Request Body: %s", string(requestBodyBytes))

	client := &http.Client{}
	resp, err := client.Do(req)
	log.Info().Msgf("Token Exchange API URL: %s", settings.TokenExchangeAPIURL)

	if err != nil {
		log.Error().Err(err).Msg("Error sending request to token exchange API")
		return c.Status(fiber.StatusInternalServerError).SendString("Error sending request to token exchange API")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error().Err(err).Msg("Error reading response from token exchange API")
		return c.Status(fiber.StatusInternalServerError).SendString("Error reading response from token exchange API")
	}

	log.Info().Msgf("Response body from token exchange API: %s", string(respBody))

	var responseMap map[string]interface{}
	if err := json.Unmarshal(respBody, &responseMap); err != nil {
		log.Error().Err(err).Msg("Error processing response")
		return c.Status(fiber.StatusInternalServerError).SendString("Error processing response")
	}

	log.Info().Msgf("Response body from token exchange API: %s", string(respBody))

	token, exists := responseMap["token"]
	if !exists {
		log.Error().Msg("Token not found in response from token exchange API")
		return c.Status(fiber.StatusInternalServerError).SendString("Token not found in response from token exchange API")
	}

	log.Info().Msgf("Token exchange successful: %s", token)
	return c.JSON(fiber.Map{"token": token})
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
		AllowOrigins:     "http://localhost:3000",
		AllowMethods:     "GET,POST,HEAD,PUT,DELETE,PATCH",
		AllowHeaders:     "Accept, Content-Type, Content-Length, Authorization",
		AllowCredentials: true,
	}))

	// Protected route
	app.Get("/api/vehicles/me", AuthMiddleware(), func(c *fiber.Ctx) error {
		return HandleGetVehicles(c, &settings)
	})

	// Public Routes
	app.Post("/auth/web3/generate_challenge", func(c *fiber.Ctx) error {
		return HandleGenerateChallenge(c, &settings)
	})
	app.Post("/auth/web3/submit_challenge", func(c *fiber.Ctx) error {
		return HandleSubmitChallenge(c, &settings)
	})

	app.Post("/api/token_exchange", AuthMiddleware(), func(c *fiber.Ctx) error {
		return HandleTokenExchange(c, &settings)
	})

	app.Get("/", func(c *fiber.Ctx) error {
		return c.SendString("can you see this")
	})

	log.Info().Msgf("Starting server on port %s", settings.Port)
	if err := app.Listen(":" + settings.Port); err != nil {
		log.Fatal().Err(err).Msg("Server failed to start")
	}
}
