package main

import (
	"encoding/json"
	"fmt"
	"github.com/dimo-network/trips-web-app/api/internal/config"
	"github.com/gofiber/fiber/v2"
	"github.com/pkg/errors"
	"net/http"
	"reflect"
)

type RawDeviceStatus struct {
	DTC                       map[string]interface{} `json:"dtc"`
	MAF                       map[string]interface{} `json:"maf"`
	VIN                       map[string]interface{} `json:"vin"`
	Cell                      map[string]interface{} `json:"cell"`
	HDOP                      map[string]interface{} `json:"hdop"`
	NSAT                      map[string]interface{} `json:"nsat"`
	WiFi                      map[string]interface{} `json:"wifi"`
	Speed                     map[string]interface{} `json:"speed"`
	Device                    map[string]interface{} `json:"device"`
	RunTime                   map[string]interface{} `json:"runTime"`
	Altitude                  map[string]interface{} `json:"altitude"`
	Timestamp                 map[string]interface{} `json:"timestamp"`
	EngineLoad                map[string]interface{} `json:"engineLoad"`
	IntakeTemp                map[string]interface{} `json:"intakeTemp"`
	CoolantTemp               map[string]interface{} `json:"coolantTemp"`
	EngineSpeed               map[string]interface{} `json:"engineSpeed"`
	ThrottlePosition          map[string]interface{} `json:"throttlePosition"`
	LongTermFuelTrim1         map[string]interface{} `json:"longTermFuelTrim1"`
	BarometricPressure        map[string]interface{} `json:"barometricPressure"`
	ShortTermFuelTrim1        map[string]interface{} `json:"shortTermFuelTrim1"`
	AcceleratorPedalPositionD map[string]interface{} `json:"acceleratorPedalPositionD"`
	AcceleratorPedalPositionE map[string]interface{} `json:"acceleratorPedalPositionE"`
}

type DeviceDataEntry struct {
	SignalName string
	Value      interface{}
	Timestamp  string
	Source     string
}

type DeviceStatusEntries []DeviceDataEntry

func processRawDeviceStatus(rawDeviceStatus RawDeviceStatus) DeviceStatusEntries {
	var entries DeviceStatusEntries

	v := reflect.ValueOf(rawDeviceStatus)
	for i := 0; i < v.NumField(); i++ {
		field := v.Field(i)
		name := v.Type().Field(i).Name

		if data, ok := field.Interface().(map[string]interface{}); ok {
			if value, exists := data["value"]; exists {
				// Check if value is a nested map and process each entry
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

func queryDeviceDataAPI(tokenID int64, settings *config.Settings, c *fiber.Ctx) (RawDeviceStatus, error) {
	var rawDeviceStatus RawDeviceStatus

	sessionCookie := c.Cookies("session_id")
	privilegeTokenKey := "privilegeToken_" + sessionCookie

	// Retrieve the privilege token from the cache
	token, found := CacheInstance.Get(privilegeTokenKey)
	if !found {
		return rawDeviceStatus, errors.New("privilege token not found in cache")
	}

	url := fmt.Sprintf("%s/vehicle/%d/status-raw", settings.DeviceDataAPIBaseURL, tokenID)

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
