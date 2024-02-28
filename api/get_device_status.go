package main

import (
	"encoding/json"
	"fmt"
	"github.com/rs/zerolog/log"
	"net/http"

	"github.com/dimo-network/trips-web-app/api/internal/config"
	"github.com/gofiber/fiber/v2"
	"github.com/pkg/errors"
)

type DeviceDataEntry struct {
	SignalName string
	Value      interface{}
	Timestamp  string
	Source     string
}

type DeviceStatusEntries []DeviceDataEntry

func processRawDeviceStatus(rawDeviceStatus map[string]interface{}) DeviceStatusEntries {
	var entries DeviceStatusEntries

	for name, field := range rawDeviceStatus {
		if data, ok := field.(map[string]interface{}); ok {
			if value, exists := data["value"]; exists {
				switch valueTyped := value.(type) {
				case map[string]interface{}:
					for k, v := range valueTyped {
						entries = append(entries, DeviceDataEntry{
							SignalName: fmt.Sprintf("%s.%s", name, k),
							Value:      fmt.Sprintf("%v", v),
							Timestamp:  fmt.Sprintf("%v", data["timestamp"]),
							Source:     fmt.Sprintf("%v", data["source"]),
						})
					}
				default:
					entries = append(entries, DeviceDataEntry{
						SignalName: name,
						Value:      fmt.Sprintf("%v", value),
						Timestamp:  fmt.Sprintf("%v", data["timestamp"]),
						Source:     fmt.Sprintf("%v", data["source"]),
					})
				}
			} else {
				entries = append(entries, DeviceDataEntry{
					SignalName: name,
					Value:      "",
					Timestamp:  fmt.Sprintf("%v", data["timestamp"]),
					Source:     fmt.Sprintf("%v", data["source"]),
				})
			}
		}
	}
	return entries
}

func queryDeviceDataAPI(tokenID int64, settings *config.Settings, c *fiber.Ctx) (map[string]interface{}, error) {
	var rawDeviceStatus map[string]interface{}

	sessionCookie := c.Cookies("session_id")
	privilegeTokenKey := "privilegeToken_" + sessionCookie

	// Retrieve the privilege token from the cache
	token, found := CacheInstance.Get(privilegeTokenKey)
	if !found {
		return rawDeviceStatus, errors.New("privilege token not found in cache")
	}

	url := fmt.Sprintf("%s/vehicle/%d/status-raw", settings.DeviceDataAPIURL, tokenID)
	log.Debug().Msgf("Request URL: %s", url)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return rawDeviceStatus, err
	}
	req.Header.Set("Authorization", "Bearer "+token.(string))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return rawDeviceStatus, err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&rawDeviceStatus); err != nil {
		return rawDeviceStatus, err
	}

	return rawDeviceStatus, nil
}
