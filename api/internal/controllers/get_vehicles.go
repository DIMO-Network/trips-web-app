package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/rs/zerolog/log"

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

func HandleVehiclesAndFeedbackData(c *fiber.Ctx, settings *config.Settings) error {
	ethAddress := c.Locals("ethereum_address").(string)
	log.Info().Msgf("EthAddress: %s", ethAddress) //troubleshooting

	vehicles, err := QueryIdentityAPIForVehicles(ethAddress, settings)
	if err != nil {
		log.Printf("Error querying My Vehicles: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Error querying my vehicles: " + err.Error())
	}

	sharedVehicles, err := QuerySharedVehicles(ethAddress, settings)
	if err != nil {
		log.Printf("Error querying Shared Vehicles: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Error querying shared vehicles: " + err.Error())
	}

	log.Info().Interface("Vehicles", vehicles).Msg("Fetched Vehicles")

	var deviceType string
	if len(vehicles) > 0 {
		aftermarketDevice := vehicles[0].AftermarketDevice
		if aftermarketDevice.Address != "" && aftermarketDevice.Serial != "" && aftermarketDevice.Manufacturer.Name != "" {
			deviceType = fmt.Sprintf("%s: %s", aftermarketDevice.Manufacturer.Name, aftermarketDevice.Serial)
			log.Info().Msgf("DeviceType: %s", deviceType)
		} else {
			log.Info().Msg("Vehicle 0 AftermarketDevice or its fields are empty")
		}
	} else {
		log.Info().Msg("No vehicles found")
	}

	email, err := QueryUsersAPI(c, settings)
	if err != nil {
		log.Printf("Error querying User API: %v", err)
		return c.Status(fiber.StatusInternalServerError).SendString("Error querying user data: " + err.Error())
	}
	log.Info().Msgf("Email: %s", email)

	build := "sample-web-app " + time.Now().Format("2006-01-02 15:04:05")

	return c.Render("vehicles", fiber.Map{
		"Title":          "My Vehicles",
		"Vehicles":       vehicles,
		"SharedVehicles": sharedVehicles,
		"EthAddress":     ethAddress,
		"Email":          email,
		"Build":          build,
		"DeviceType":     deviceType,
	})
}

func QueryIdentityAPIForVehicles(ethAddress string, settings *config.Settings) ([]Vehicle, error) {
	// GraphQL query
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

func QuerySharedVehicles(ethAddress string, settings *config.Settings) ([]Vehicle, error) {
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
	vehicles = append(vehicles, vehicleResponse.Data.Vehicles.Nodes...)

	return vehicles, nil
}
