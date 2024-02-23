package main

import (
	"encoding/json"
	"fmt"
	"github.com/dimo-network/trips-web-app/api/internal/config"
	"github.com/gofiber/fiber/v2"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
	"net/http"
	"net/url"
)

type Trip struct {
	ID    string    `json:"id"`
	Start TimeEntry `json:"start"`
	End   TimeEntry `json:"end"`
}

type TimeEntry struct {
	Time string `json:"time"`
}

type TripsResponse struct {
	Trips []Trip `json:"trips"`
}

type GeoJSONFeatureCollection struct {
	Type     string           `json:"type"`
	Features []GeoJSONFeature `json:"features"`
}

type GeoJSONFeature struct {
	Type     string          `json:"type"`
	Geometry GeoJSONGeometry `json:"geometry"`
}

type GeoJSONGeometry struct {
	Type        string      `json:"type"`
	Coordinates [][]float64 `json:"coordinates"`
}

type HistoryResponse struct {
	Hits struct {
		Hits []struct {
			Source struct {
				Data LocationData `json:"data"`
			} `json:"_source"`
		} `json:"hits"`
	} `json:"hits"`
}

type LocationData struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

func queryTripsAPI(tokenID int64, settings *config.Settings, c *fiber.Ctx) ([]Trip, error) {
	var tripsResponse TripsResponse

	sessionCookie := c.Cookies("session_id")
	privilegeTokenKey := "privilegeToken_" + sessionCookie

	// Retrieve the privilege token from the cache
	token, found := CacheInstance.Get(privilegeTokenKey)
	if !found {
		return nil, errors.New("privilege token not found in cache")
	}

	url := fmt.Sprintf("%s/vehicle/%d/trips", settings.TripsAPIBaseURL, tokenID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.(string))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&tripsResponse); err != nil {
		return nil, err
	}

	// Log each trip ID
	for _, trip := range tripsResponse.Trips {
		log.Info().Msgf("Trip ID: %s", trip.ID)
	}

	return tripsResponse.Trips, nil
}

func extractLocationData(historyData HistoryResponse) []LocationData {
	var locations []LocationData
	for _, hit := range historyData.Hits.Hits {
		locData := LocationData{
			Latitude:  hit.Source.Data.Latitude,
			Longitude: hit.Source.Data.Longitude,
		}
		locations = append(locations, locData)
	}
	return locations
}

func convertToGeoJSON(locations []LocationData) GeoJSONFeatureCollection {
	var coordinates [][]float64
	for _, loc := range locations {
		coordinates = append(coordinates, []float64{loc.Longitude, loc.Latitude})
	}

	geoJSON := GeoJSONFeatureCollection{
		Type: "FeatureCollection",
		Features: []GeoJSONFeature{
			{
				Type: "Feature",
				Geometry: GeoJSONGeometry{
					Type:        "LineString",
					Coordinates: coordinates,
				},
			},
		},
	}

	return geoJSON
}

func queryDeviceDataHistory(tokenID int64, startTime string, endTime string, settings *config.Settings, c *fiber.Ctx) ([]LocationData, error) {
	var historyResponse HistoryResponse

	sessionCookie := c.Cookies("session_id")
	privilegeTokenKey := "privilegeToken_" + sessionCookie

	// Retrieve the privilege token from the cache
	token, found := CacheInstance.Get(privilegeTokenKey)
	if !found {
		return nil, errors.New("privilege token not found in cache")
	}

	ddUrl := fmt.Sprintf("%s/v1/vehicle/%d/history?start=%s&end=%s", settings.DeviceDataAPIBaseURL, tokenID, url.QueryEscape(startTime), url.QueryEscape(endTime))

	req, err := http.NewRequest("GET", ddUrl, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token.(string))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if err := json.NewDecoder(resp.Body).Decode(&historyResponse); err != nil {
		return nil, err
	}

	locations := extractLocationData(historyResponse)
	return locations, nil
}

func handleMapDataForTrip(c *fiber.Ctx, settings *config.Settings, tripID, startTime, endTime string) error {
	ethAddress := c.Locals("ethereum_address").(string)
	vehicles, err := queryIdentityAPIForVehicles(ethAddress, settings)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if len(vehicles) == 0 {
		return c.Status(fiber.StatusNotFound).SendString("No vehicles found")
	}

	var tokenID int64
	var tripFound = false
	for _, vehicle := range vehicles {
		if tripFound {
			break
		}

		trips, err := queryTripsAPI(vehicle.TokenID, settings, c)
		if err != nil {
			continue
		}

		for _, trip := range trips {
			if trip.ID == tripID {
				tokenID = vehicle.TokenID
				tripFound = true
				break
			}
		}
	}

	if !tripFound {
		return c.Status(fiber.StatusNotFound).SendString("Trip not found")
	}

	// Fetch historical data for the specific trip
	locations, err := queryDeviceDataHistory(tokenID, startTime, endTime, settings, c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch historical data: " + err.Error()})
	}

	// Convert the historical data to GeoJSON
	geoJSON := convertToGeoJSON(locations)
	return c.JSON(geoJSON)
}
