package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/dimo-network/trips-web-app/api/internal/config"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

func RequestPriviledgeToken(c *fiber.Ctx, settings *config.Settings, tokenID int64) (*string, error) {
	sessionCookie := c.Cookies("session_id")
	privilegeTokenKey := fmt.Sprintf("privilegeToken_%s", sessionCookie)

	privilegeToken, exists := CacheInstance.Get(privilegeTokenKey)

	if exists {
		privilegeTokenString, ok := privilegeToken.(string)
		if !ok {
			return nil, fmt.Errorf("privilege token value is not valid")
		}
		return &privilegeTokenString, nil
	}

	jwtToken, found := CacheInstance.Get(sessionCookie)
	if !found {
		return nil, fmt.Errorf("JWT token not found in cache")
	}

	accessToken, ok := jwtToken.(string)
	if !ok {
		return nil, fmt.Errorf("JWT token value is not valid")
	}

	privileges := []int{1, 4}
	requestBody := map[string]interface{}{
		"nftContractAddress": settings.PrivilegeNFTContractAddr,
		"privileges":         privileges,
		"tokenID":            tokenID,
	}

	requestBodyBytes, err := json.Marshal(requestBody)
	if err != nil {
		return nil, fmt.Errorf("error marshalling request body")
	}

	req, err := http.NewRequest("POST", settings.TokenExchangeAPIURL, bytes.NewBuffer(requestBodyBytes))
	if err != nil {
		return nil, fmt.Errorf("error creating request to token exchange API")
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", accessToken))
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)

	if err != nil {
		return nil, fmt.Errorf("error making request to token exchange API")
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("error reading response from token exchange API")
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("invalid response from token exchange server: %d,  %s", resp.StatusCode, string(respBody))
	}

	var responseMap map[string]interface{}
	if err := json.Unmarshal(respBody, &responseMap); err != nil {
		log.Error().Err(err).Str("body", string(respBody)).Msg("Error processing response")
		return nil, fmt.Errorf("error processing response from token exchange API")
	}

	token, exists := responseMap["token"]
	if !exists {
		log.Error().Interface("response", responseMap).Msg("Token not found in response")
		return nil, fmt.Errorf("token not found in response from token exchange API")
	}

	privilegeTokenString, ok := token.(string)
	if !ok {
		return nil, fmt.Errorf("token value is not valid")
	}

	CacheInstance.Set(privilegeTokenKey, token, time.Second*30)

	return &privilegeTokenString, nil
}
