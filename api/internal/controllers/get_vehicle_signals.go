package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"

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

	log.Info().Msgf("Sending FetchAvailableSignals query: %s", graphqlQuery)

	privilegeToken, err := RequestPriviledgeToken(c, settings, tokenID)
	if err != nil {
		log.Error().Err(err).Msg("Error obtaining privilege token")
		return nil, errors.Wrap(err, "error getting privilege token")
	}

	resp, err := makeGraphQLRequest(settings.TelemetryAPIURL, graphqlQuery, privilegeToken)
	if err != nil {
		log.Error().Err(err).Msg("Error making request to Telemetry API for available signals")
		return nil, err
	}

	log.Info().Msgf("Received response for FetchAvailableSignals: %s", string(resp))

	if err := json.Unmarshal(resp, &availableSignals); err != nil {
		log.Error().Err(err).Msg("Error parsing available signals response")
		return nil, errors.Wrap(err, "error parsing available signals response")
	}

	log.Info().Msgf("Parsed available signals: %v", availableSignals.Data.AvailableSignals)
	return availableSignals.Data.AvailableSignals, nil
}

// FetchLatestSignalValues retrieves the latest timestamp and value for each available signal
func FetchLatestSignalValues(tokenID int64, signalNames []string, settings *config.Settings, c *fiber.Ctx) (SignalEntries, error) {
	var latestSignalData struct {
		Data struct {
			SignalsLatest map[string]struct {
				Timestamp string      `json:"timestamp"`
				Value     interface{} `json:"value"`
			} `json:"signalsLatest"`
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

	log.Info().Msgf("Sending FetchLatestSignalValues query: %s", graphqlQuery)

	privilegeToken, err := RequestPriviledgeToken(c, settings, tokenID)
	if err != nil {
		log.Error().Err(err).Msg("Error obtaining privilege token")
		return nil, errors.Wrap(err, "error getting privilege token")
	}

	resp, err := makeGraphQLRequest(settings.TelemetryAPIURL, graphqlQuery, privilegeToken)
	if err != nil {
		log.Error().Err(err).Msg("Error making request to Telemetry API for latest signal values")
		return nil, err
	}

	log.Info().Msgf("Received response for FetchLatestSignalValues: %s", string(resp))

	if err := json.Unmarshal(resp, &latestSignalData); err != nil {
		log.Error().Err(err).Msg("Error parsing latest signal values response")
		return nil, errors.Wrap(err, "error parsing latest signal values response")
	}

	for signalName, signalData := range latestSignalData.Data.SignalsLatest {
		entry := SignalEntry{
			SignalName: signalName,
			Value:      signalData.Value,
			Timestamp:  signalData.Timestamp,
		}
		entries = append(entries, entry)
		log.Info().Msgf("Parsed signal entry: %v", entry)
	}

	log.Info().Msgf("Final parsed signal entries: %v", entries)
	return entries, nil
}

func (v *VehiclesController) HandleGetHistoricalData(c *fiber.Ctx) error {
	tokenID, err := strconv.ParseInt(c.Params("tokenid"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid token ID",
		})
	}

	signalName := c.Query("signalName")
	if signalName == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Signal name is required",
		})
	}

	// Calculate start and end times for the past 7 days
	endTime := time.Now().UTC()
	startTime := endTime.AddDate(0, 0, -7)

	// Fetch historical data with a 24-hour interval
	entries, err := FetchHistoricalSignalValues(tokenID, signalName, startTime.Format(time.RFC3339), endTime.Format(time.RFC3339), v.settings, c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch historical signal values",
		})
	}

	return c.JSON(entries)
}

func FetchHistoricalSignalValues(tokenID int64, signalName string, startTime, endTime string, settings *config.Settings, c *fiber.Ctx) ([]SignalEntry, error) {
	var historicalData struct {
		Data struct {
			Signals []map[string]interface{} `json:"signals"`
		} `json:"data"`
	}

	graphqlQuery := fmt.Sprintf(`{
		signals(tokenId: %d, interval: "24h", from: "%s", to: "%s") {
			%s(agg: MAX)
		}
	}`, tokenID, startTime, endTime, signalName)

	log.Info().Msgf("Sending FetchHistoricalSignalValues query: %s", graphqlQuery)

	privilegeToken, err := RequestPriviledgeToken(c, settings, tokenID)
	if err != nil {
		log.Error().Err(err).Msg("Error obtaining privilege token")
		return nil, errors.Wrap(err, "error getting privilege token")
	}

	resp, err := makeGraphQLRequest(settings.TelemetryAPIURL, graphqlQuery, privilegeToken)
	if err != nil {
		log.Error().Err(err).Msg("Error making request to Telemetry API for historical values")
		return nil, err
	}

	log.Info().Msgf("Received response for FetchHistoricalSignalValues: %s", string(resp))

	if err := json.Unmarshal(resp, &historicalData); err != nil {
		log.Error().Err(err).Msg("Error parsing historical values response")
		return nil, errors.Wrap(err, "error parsing historical values response")
	}

	entries := []SignalEntry{}
	for _, signal := range historicalData.Data.Signals {
		if value, ok := signal[signalName]; ok {
			entry := SignalEntry{
				SignalName: signalName,
				Value:      value,
				Timestamp:  "", // No specific timestamp with aggregation
			}
			entries = append(entries, entry)
			log.Info().Msgf("Parsed historical signal entry: %v", entry)
		} else {
			log.Warn().Msgf("Signal data missing for: %s", signalName)
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
