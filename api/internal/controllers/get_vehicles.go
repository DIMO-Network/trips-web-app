package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
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

func HandleGiveFeedback(settings *config.Settings) fiber.Handler {
	return func(c *fiber.Ctx) error {
		ethAddress, ok := c.Locals("ethereum_address").(string)
		if !ok {
			// Handle the case where ethereum_address is not set or not a string
			// For example, return an error or set a default value
			return c.Status(fiber.StatusBadRequest).SendString("Ethereum address not provided")
		}
		email, err := GetEmailFromUsersAPI(c, settings)
		if err != nil {
			log.Error().Err(err).Msg("Error querying User API for email")
			return c.Status(fiber.StatusInternalServerError).SendString("Error querying user data: " + err.Error())
		}

		var deviceType string
		vehicles, err := QueryIdentityAPIForVehicles(ethAddress, settings)
		if err != nil {
			log.Error().Err(err).Msg("Error querying My Vehicles")
			return c.Status(fiber.StatusInternalServerError).SendString("Error querying my vehicles: " + err.Error())
		}

		if len(vehicles) > 0 {
			aftermarketDevice := vehicles[0].AftermarketDevice
			if aftermarketDevice.Address != "" && aftermarketDevice.Serial != "" && aftermarketDevice.Manufacturer.Name != "" {
				deviceType = fmt.Sprintf("%s: %s", aftermarketDevice.Manufacturer.Name, aftermarketDevice.Serial)
			}
		}

		feedbackURL := fmt.Sprintf("https://formcrafts.com/a/74047?field59=%s&field55=%s&field56=%s&field57=%s",
			url.QueryEscape(ethAddress), url.QueryEscape(email), url.QueryEscape("sample-web-app "+time.Now().Format("2006-01-02 15:04:05")), url.QueryEscape(deviceType))

		return c.Redirect(feedbackURL, http.StatusFound) // 302 status code
	}
}

func HandleGetVehicles(c *fiber.Ctx, settings *config.Settings) error {
	ethAddress := c.Locals("ethereum_address").(string)

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

	return c.Render("vehicles", fiber.Map{
		"Title":          "My Vehicles",
		"Vehicles":       vehicles,
		"SharedVehicles": sharedVehicles,
		"EthAddress":     ethAddress,
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
