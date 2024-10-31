package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"

	"github.com/dimo-network/trips-web-app/api/internal/config"
	"github.com/gofiber/fiber/v2"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
)

type SignalEntry struct {
	SignalName string
	Value      interface{}
	Timestamp  string
}

type SignalEntries []SignalEntry

// FetchAvailableSignals retrieves a list of available signals for a given vehicle
func FetchAvailableSignals(tokenID int64, settings *config.Settings, c *fiber.Ctx) ([]string, error) {
	var availableSignals struct {
		Data struct {
			AvailableSignals []string `json:"availableSignals"`
		} `json:"data"`
	}

	graphqlQuery := fmt.Sprintf(`{
		availableSignals(tokenId: %d)
	}`, tokenID)

	privilegeToken, err := RequestPriviledgeToken(c, settings, tokenID)
	if err != nil {
		return nil, errors.Wrap(err, "error getting privilege token")
	}

	resp, err := makeGraphQLRequest(settings.TelemetryAPIURL, graphqlQuery, privilegeToken)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(resp, &availableSignals); err != nil {
		return nil, errors.Wrap(err, "error parsing available signals response")
	}

	return availableSignals.Data.AvailableSignals, nil
}

// FetchLatestSignalValues retrieves the latest timestamp and value for each available signal
func FetchLatestSignalValues(tokenID int64, signalNames []string, settings *config.Settings, c *fiber.Ctx) (SignalEntries, error) {
	var latestSignalData struct {
		Data map[string]map[string]struct {
			Timestamp string      `json:"timestamp"`
			Value     interface{} `json:"value"`
		} `json:"data"`
	}

	entries := SignalEntries{}
	signalsQuery := ""
	for _, signal := range signalNames {
		signalsQuery += fmt.Sprintf("%s { timestamp value } ", signal)
	}

	graphqlQuery := fmt.Sprintf(`{
		signalsLatest(tokenId: %d) {
			%s
		}
	}`, tokenID, signalsQuery)

	privilegeToken, err := RequestPriviledgeToken(c, settings, tokenID)
	if err != nil {
		return nil, errors.Wrap(err, "error getting privilege token")
	}

	resp, err := makeGraphQLRequest(settings.TelemetryAPIURL, graphqlQuery, privilegeToken)
	if err != nil {
		return nil, err
	}

	if err := json.Unmarshal(resp, &latestSignalData); err != nil {
		return nil, errors.Wrap(err, "error parsing latest signal values response")
	}

	for signalName, signalData := range latestSignalData.Data {
		if data, ok := signalData[signalName]; ok {
			entries = append(entries, SignalEntry{
				SignalName: signalName,
				Value:      data.Value,
				Timestamp:  data.Timestamp,
			})
		}
	}

	return entries, nil
}

func makeGraphQLRequest(url, graphqlQuery string, privilegeToken *string) ([]byte, error) {
	requestPayload := map[string]interface{}{
		"query": graphqlQuery,
	}
	payloadBytes, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", url, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *privilegeToken))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	return io.ReadAll(resp.Body)
}

func (v *VehiclesController) HandleVehicleTelemetry(c *fiber.Ctx) error {
	tokenID, err := strconv.ParseInt(c.Params("tokenid"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid token ID",
		})
	}

	// Fetch available signals
	signalNames, err := FetchAvailableSignals(tokenID, &v.settings, c)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch available signals")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch available signals",
		})
	}

	// Fetch latest values for each signal
	telemetrySignals, err := FetchLatestSignalValues(tokenID, signalNames, &v.settings, c)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch latest signal values")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch latest signal values",
		})
	}

	return c.Render("vehicle_signals", fiber.Map{
		"TokenID":       tokenID,
		"SignalEntries": telemetrySignals,
		"Privileges":    []any{},
	})
}
