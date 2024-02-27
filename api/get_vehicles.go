package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/dimo-network/trips-web-app/api/internal/config"
	"github.com/gofiber/fiber/v2"
	"github.com/rs/zerolog/log"
)

type GraphQLRequest struct {
	Query string `json:"query"`
}

type Vehicle struct {
	TokenID  int64 `json:"tokenId"`
	Earnings struct {
		TotalTokens string `json:"totalTokens"`
	} `json:"earnings"`
	Definition struct {
		Make  string `json:"make"`
		Model string `json:"model"`
		Year  int    `json:"year"`
	} `json:"definition"`
	AftermarketDevice struct {
		Address      string `json:"address"`
		Serial       string `json:"serial"`
		Manufacturer struct {
			Name string `json:"name"`
		} `json:"manufacturer"`
	} `json:"aftermarketDevice"`
	DeviceStatusEntries []DeviceDataEntry `json:"deviceStatusEntries"`
	Trips               []Trip            `json:"trips"`
}

func HandleGetVehicles(c *fiber.Ctx, settings *config.Settings) error {
	ethAddress := c.Locals("ethereum_address").(string)

	vehicles, err := queryIdentityAPIForVehicles(ethAddress, settings)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).SendString("Error querying identity API: " + err.Error())
	}

	for i := range vehicles {
		// fetch raw status
		rawStatus, err := queryDeviceDataAPI(vehicles[i].TokenID, settings, c)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get raw device status")
			return c.Status(fiber.StatusInternalServerError).SendString(fmt.Sprintf("Failed to get raw device status for vehicle with TokenID: %d", vehicles[i].TokenID))
		}
		vehicles[i].DeviceStatusEntries = processRawDeviceStatus(rawStatus)

		// fetch trips for each vehicle
		trips, err := queryTripsAPI(vehicles[i].TokenID, settings, c)
		if err != nil {
			log.Error().Err(err).Msg("Failed to get trips for vehicle")
			continue
		}
		vehicles[i].Trips = trips
	}

	return c.Render("vehicles", fiber.Map{
		"Title":    "My Vehicles",
		"Vehicles": vehicles,
	})
}

func queryIdentityAPIForVehicles(ethAddress string, settings *config.Settings) ([]Vehicle, error) {
	// GraphQL query
	graphqlQuery := `{
        vehicles(first: 10, filterBy: { owner: "` + ethAddress + `" }) {
            nodes {
                tokenId,
                earnings {
                    totalTokens
                },
                definition {
                    make,
                    model,
                    year
                },
                aftermarketDevice {
                    address,
                    serial,
                    manufacturer {
                        name
                    }
                }
            }
        }
    }`

	// GraphQL request
	requestPayload := GraphQLRequest{Query: graphqlQuery}
	payloadBytes, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, err
	}

	// POST request
	req, err := http.NewRequest("POST", settings.IdentityAPIURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var vehicleResponse struct {
		Data struct {
			Vehicles struct {
				Nodes []Vehicle `json:"nodes"`
			} `json:"vehicles"`
		} `json:"data"`
	}

	if err := json.Unmarshal(body, &vehicleResponse); err != nil {
		return nil, err
	}

	vehicles := make([]Vehicle, 0, len(vehicleResponse.Data.Vehicles.Nodes))
	for _, v := range vehicleResponse.Data.Vehicles.Nodes {
		vehicles = append(vehicles, Vehicle{
			TokenID:           v.TokenID,
			Earnings:          v.Earnings,
			Definition:        v.Definition,
			AftermarketDevice: v.AftermarketDevice,
		})
	}

	return vehicles, nil
}
