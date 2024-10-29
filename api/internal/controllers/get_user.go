package controllers

import (
	"encoding/json"
	"fmt"
	"github.com/google/uuid"
	"io"
	"net/http"
	"time"

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
	settings config.Settings
}

func NewAccountController(settings config.Settings) AccountController {
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
	// get the jwt from the form post
	jwt := c.FormValue("jwt")

	ethAddr, err := ExtractEthereumAddressFromToken(jwt)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).Render("login_jwt", fiber.Map{"error": "No ethereum address found in ethAddr " + err.Error()})
	}
	fmt.Println("processed login with JWT, eth addr: ", ethAddr)

	// set the session cookie stuff with the jwt
	//jwt ethAddr storage
	sessionID := uuid.New().String()
	CacheInstance.Set(sessionID, jwt, 2*time.Hour)

	cookie := new(fiber.Cookie)
	cookie.Name = "session_id"
	cookie.Value = sessionID
	cookie.Expires = time.Now().Add(2 * time.Hour)
	cookie.HTTPOnly = true

	c.Cookie(cookie)

	return c.Redirect("/account")
}
