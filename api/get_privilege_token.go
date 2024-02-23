package main

import (
	"bytes"
	"encoding/json"
	"github.com/dimo-network/trips-web-app/api/internal/config"
	"github.com/gofiber/fiber/v2"
	"github.com/patrickmn/go-cache"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
)

func HandleTokenExchange(c *fiber.Ctx, settings *config.Settings) error {

	ethAddress := c.Locals("ethereum_address").(string)
	vehicles, err := queryIdentityAPIForVehicles(ethAddress, settings)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Failed to query vehicles")
	}
	if len(vehicles) == 0 {
		return c.Status(fiber.StatusInternalServerError).SendString("No vehicles found")
	}
	tokenId := vehicles[0].TokenID

	log.Info().Msg("HandleTokenExchange called")

	sessionCookie := c.Cookies("session_id")

	jwtToken, found := CacheInstance.Get(sessionCookie)
	if !found {
		return c.Status(fiber.StatusUnauthorized).SendString("Unauthorized: No session found")
	}

	idToken, ok := jwtToken.(string)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).SendString("Internal Error: Token format is invalid")
	}

	log.Info().Msgf("JWT being sent: %s", idToken)

	nftContractAddress := "0xbA5738a18d83D41847dfFbDC6101d37C69c9B0cF"
	privileges := []int{4}
	requestBody := map[string]interface{}{
		"nftContractAddress": nftContractAddress,
		"privileges":         privileges,
		"tokenId":            tokenId,
	}

	requestBodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error marshaling request body")
	}

	log.Info().Msgf("Request body being sent: %s", string(requestBodyBytes))

	req, err := http.NewRequest("POST", settings.TokenExchangeAPIURL, bytes.NewBuffer(requestBodyBytes))
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error creating new request")
	}

	req.Header.Set("Authorization", "Bearer "+idToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error sending request to token exchange API")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error reading response from token exchange API")
	}

	var responseMap map[string]interface{}
	if err := json.Unmarshal(respBody, &responseMap); err != nil {
		log.Error().Err(err).Msg("Error processing response")
		return c.Status(fiber.StatusInternalServerError).SendString("Error processing response")
	}

	token, exists := responseMap["token"]
	if !exists {
		return c.Status(fiber.StatusInternalServerError).SendString("Token not found in response from token exchange API")
	}

	// privilege token storage
	privilegeTokenKey := "privilegeToken_" + sessionCookie
	CacheInstance.Set(privilegeTokenKey, token, cache.DefaultExpiration)

	log.Info().Msgf("Token exchange successful: %s", token)
	return c.JSON(fiber.Map{"token": token})
}
