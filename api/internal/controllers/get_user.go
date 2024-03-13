package controllers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/dimo-network/trips-web-app/api/internal/config"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

type UserResponse struct {
	Email struct {
		Address string `json:"address"`
	} `json:"email"`
}

func QueryUsersAPI(c *fiber.Ctx, settings *config.Settings) (string, error) {
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
