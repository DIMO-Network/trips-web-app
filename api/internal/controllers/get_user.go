package controllers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/google/uuid"

	"github.com/dimo-network/trips-web-app/api/internal/config"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

type UserResponse struct {
	Email struct {
		Address string `json:"address"`
	} `json:"email"`
}

func GetEmailFromUsersAPI(c *fiber.Ctx, settings *config.Settings) (string, error) {
	sessionCookie := c.Cookies("session_id")
	jwtToken, found := CacheInstance.Get(sessionCookie)
	if !found {
		return "", fmt.Errorf("JWT token not found in cache")
	}

	accessToken, ok := jwtToken.(string)
	if !ok {
		return "", fmt.Errorf("JWT token value is not valid")
	}

	url := fmt.Sprintf("%s/user", settings.UsersAPIBaseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error().Interface("response", resp).Msgf("Error reading response body: %v", err)
		return "", err
	}

	log.Info().Msgf("API Response Body: %s", string(responseBody))

	var userResponse UserResponse
	if err := json.Unmarshal(responseBody, &userResponse); err != nil {
		log.Error().Str("body", string(responseBody)).Msgf("Error parsing JSON response: %v", err)
		return "", err
	}

	return userResponse.Email.Address, nil
}

type AccountController struct {
	settings *config.Settings
}

func NewAccountController(settings *config.Settings) AccountController {
	return AccountController{settings: settings}
}

func (a *AccountController) MyAccount(c *fiber.Ctx) error {
	sessionCookie := c.Cookies("session_id")
	if sessionCookie == "" {
		fmt.Println("No session_id cookie")
		return c.Render("session_expired", fiber.Map{})
	}

	// check if the session_id is in the cache
	jwtToken, found := CacheInstance.Get(sessionCookie)
	if !found {
		fmt.Println("Session expired")
		return c.Render("session_expired", fiber.Map{})
	}

	ethAddress := c.Locals("ethereum_address").(string)

	vehicles, err := QueryIdentityAPIForVehicles(ethAddress, &a.settings)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error querying identity API: " + err.Error())
	}

	if len(vehicles) == 0 {
		vehicles, err = QuerySharedVehicles(ethAddress, &a.settings)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).SendString("Error querying shared vehicles: " + err.Error())
		}
	}

	if err != nil {
		return c.Render("session_expired", fiber.Map{})
	}

	return c.Render("account", fiber.Map{
		"Token": jwtToken,
		"Privileges": fiber.Map{
			"1": "1: All-time, non-location data",
			"2": "Commands",
			"3": "Current Location",
			"4": "4: All-time location",
			"5": "Verifiable Credentials",
			"6": "Streams",
		},
		"Vehicles": vehicles,
	})
}

func (a *AccountController) LoginWithJWT(c *fiber.Ctx) error {
	return c.Render("login_jwt", fiber.Map{})
}

func (a *AccountController) PostLoginWithJWT(c *fiber.Ctx) error {
	log.Info().Msg("Entered PostLoginWithJWT")

	// Parse JSON body for JWT
	var body struct {
		JWT string `json:"jwt"`
	}
	if err := c.BodyParser(&body); err != nil {
		log.Error().Msg("Error parsing request body")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid request body"})
	}

	jwt := body.JWT
	if jwt == "" {
		log.Error().Msg("JWT token missing from body")
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "JWT token missing"})
	}
	log.Info().Msgf("Received JWT: %s", jwt)

	// Verify JWT and extract Ethereum address
	ethAddr, err := ExtractEthereumAddressFromToken(jwt)
	if err != nil {
		log.Error().Msgf("Invalid JWT: %v", err)
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "Invalid JWT"})
	}
	log.Info().Msgf("Extracted Ethereum address: %s", ethAddr)

	// Set session ID and store JWT in cache
	sessionID := uuid.New().String()
	CacheInstance.Set(sessionID, jwt, 2*time.Hour)
	log.Info().Msgf("Stored JWT in cache with session ID: %s", sessionID)

	// Set session cookie
	c.Cookie(&fiber.Cookie{
		Name:     "session_id",
		Value:    sessionID,
		Expires:  time.Now().Add(2 * time.Hour),
		HTTPOnly: true,
		Secure:   true,
	})
	log.Info().Msg("Session cookie set successfully")

	return c.Redirect("/vehicles/me")
}
