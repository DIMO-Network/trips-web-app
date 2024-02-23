package main

import (
	"encoding/json"
	"fmt"
	"github.com/dimo-network/trips-web-app/api/internal/config"
	"github.com/gofiber/fiber/v2"
	geojson "github.com/paulmach/go.geojson"
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

var tripIDToTokenIDMap = make(map[string]int64)

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
		tripIDToTokenIDMap[trip.ID] = tokenID
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

func convertToGeoJSON(locations []LocationData, tripID string, tripStart string, tripEnd string) *geojson.FeatureCollection {
	coords := make([][]float64, 0, len(locations))

	for _, loc := range locations {
		// Append each location as a coordinate pair in the coords slice
		coords = append(coords, []float64{loc.Longitude, loc.Latitude})
	}

	feature := geojson.NewLineStringFeature(coords)

	feature.Properties = map[string]interface{}{
		"type":         "LineString",
		"trip_id":      tripID,
		"trip_start":   tripStart,
		"trip_end":     tripEnd,
		"privacy_zone": 1,
		"color":        "black",
		"point-color":  "black",
	}

	// Create a feature collection and add the LineString feature to it
	fc := geojson.NewFeatureCollection()
	fc.AddFeature(feature)

	return fc
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
	tokenID, exists := tripIDToTokenIDMap[tripID]
	if !exists {
		return c.Status(fiber.StatusNotFound).SendString("Trip not found")
	}

	// Fetch historical data for the specific trip
	locations, err := queryDeviceDataHistory(tokenID, startTime, endTime, settings, c)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch historical data: " + err.Error()})
	}

	// Convert the historical data to GeoJSON
	geoJSON := convertToGeoJSON(locations, tripID, startTime, endTime)
	return c.JSON(geoJSON)
}
