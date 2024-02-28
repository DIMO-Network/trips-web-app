package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dimo-network/trips-web-app/api/internal/config"
	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"github.com/rs/zerolog/log"
)

type ChallengeRequest struct {
	Address string `json:"address"`
}

type SignatureRequest struct {
	State     string `json:"state"`
	Signature string `json:"signature"`
}

func HandleGenerateChallenge(c *fiber.Ctx, settings *config.Settings) error {
	var challengeReq ChallengeRequest
	if err := c.BodyParser(&challengeReq); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid request body")
	}

	address := challengeReq.Address

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

	log.Info().Msgf("Response from generate challenge: %+v", apiResp)

	return c.JSON(apiResp)
}

func HandleSubmitChallenge(c *fiber.Ctx, settings *config.Settings) error {
	var signatureReq SignatureRequest
	if err := c.BodyParser(&signatureReq); err != nil {
		return c.Status(fiber.StatusBadRequest).SendString("Invalid request body")
	}

	log.Info().Msgf("State: %s, Signature: %s", signatureReq.State, signatureReq.Signature)

	formData := url.Values{}
	formData.Add("client_id", settings.ClientID)
	formData.Add("domain", settings.Domain)
	formData.Add("grant_type", settings.GrantType)
	formData.Add("state", signatureReq.State)
	formData.Add("signature", signatureReq.Signature)

	log.Info().Msgf("Response from submit challenge: %+v", formData)

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

	token, exists := responseMap["id_token"]
	if !exists {
		return c.Status(fiber.StatusInternalServerError).SendString("Token not found in response")
	}

	//jwt token storage
	sessionID := uuid.New().String()
	CacheInstance.Set(sessionID, token, 2*time.Hour)

	cookie := new(fiber.Cookie)
	cookie.Name = "session_id"
	cookie.Value = sessionID
	cookie.Expires = time.Now().Add(2 * time.Hour)
	cookie.HTTPOnly = true
	cookie.Domain = "localhost"

	c.Cookie(cookie)

	return c.JSON(fiber.Map{"message": "Challenge accepted and session started!", "id_token": token})
}
