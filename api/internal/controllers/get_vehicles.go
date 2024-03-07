package controllers

import (
	"bytes"
	"encoding/json"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"

	"github.com/dimo-network/trips-web-app/api/internal/config"
	"github.com/gofiber/fiber/v2"
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

	vehicles, err := queryMyVehicles(ethAddress, settings)
	if err != nil {
		log.Printf("Error querying My Vehicles: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Error querying my vehicles: " + err.Error())
	}

	sharedVehicles, err := querySharedVehicles(ethAddress, settings)
	if err != nil {
		log.Printf("Error querying Shared Vehicles: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Error querying shared vehicles: " + err.Error())
	}

	return c.Render("vehicles", fiber.Map{
		"Title":          "My Vehicles",
		"Vehicles":       vehicles,
		"SharedVehicles": sharedVehicles,
	})
}

func queryMyVehicles(ethAddress string, settings *config.Settings) ([]Vehicle, error) {
	graphqlQuery := `{
        vehicles(first: 50, filterBy: { owner: "` + ethAddress + `" }) {
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

	return fetchVehiclesWithQuery(graphqlQuery, settings)
}

func querySharedVehicles(ethAddress string, settings *config.Settings) ([]Vehicle, error) {
	graphqlQuery := `{
        vehicles(first: 50, filterBy: {privileged: "` + ethAddress + `" }) {
            nodes {
                tokenId,
                name,
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

	return fetchVehiclesWithQuery(graphqlQuery, settings)
}

func fetchVehiclesWithQuery(query string, settings *config.Settings) ([]Vehicle, error) {
	// GraphQL request
	requestPayload := GraphQLRequest{Query: query}
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
		vehicles = append(vehicles, v)
	}

	return vehicles, nil
}
