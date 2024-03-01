package controllers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"

	"github.com/dimo-network/trips-web-app/api/internal/config"
	"github.com/gofiber/fiber/v2"
	geojson "github.com/paulmach/go.geojson"
	"github.com/pkg/errors"
	"github.com/rs/zerolog/log"
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

var TripIDToTokenIDMap = make(map[string]int64)

type LocationData struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
}

func QueryTripsAPI(tokenID int64, settings *config.Settings, c *fiber.Ctx) ([]Trip, error) {

	var tripsResponse TripsResponse

	privilegeToken, err := RequestPriviledgeToken(c, settings, tokenID)

	if err != nil {
		return []Trip{}, errors.Wrap(err, "error getting privilege token")
	}

	url := fmt.Sprintf("%s/vehicle/%d/trips", settings.TripsAPIBaseURL, tokenID)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *privilegeToken))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read the raw response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error().Interface("response", resp).Msgf("Error reading response body: %v", err)
		return nil, err
	}

	// Dynamically parse the JSON response
	if err := json.Unmarshal(responseBody, &tripsResponse); err != nil {
		log.Error().Str("body", string(responseBody)).Msgf("Error parsing JSON response: %v", err)
		return nil, err
	}

	sort.Slice(tripsResponse.Trips, func(i, j int) bool {
		return tripsResponse.Trips[i].End.Time > tripsResponse.Trips[j].End.Time
	})

	// 20 latest trips
	latestTrips := tripsResponse.Trips
	if len(latestTrips) > 20 {
		latestTrips = latestTrips[:20]
	}

	for _, trip := range latestTrips {
		TripIDToTokenIDMap[trip.ID] = tokenID
		log.Info().Msgf("Trip ID: %s", trip.ID)
	}

	return latestTrips, nil
}

func queryDeviceDataHistory(tokenID int64, startTime string, endTime string, settings *config.Settings, c *fiber.Ctx) ([]LocationData, error) {

	privilegeToken, err := RequestPriviledgeToken(c, settings, tokenID)

	if err != nil {
		return []LocationData{}, errors.Wrap(err, "error getting privilege token")
	}

	ddURL := fmt.Sprintf("%s/vehicle/%d/history?startDate=%s&endDate=%s", settings.DeviceDataAPIURL, tokenID, url.QueryEscape(startTime), url.QueryEscape(endTime))

	req, err := http.NewRequest("GET", ddURL, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *privilegeToken))

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Read the raw response body
	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Dynamically parse the JSON response
	var result map[string]interface{}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return nil, err
	}

	// Extract the hits array
	hits := result["hits"].(map[string]interface{})["hits"].([]interface{})

	// Sort the hits based on the timestamp
	sort.SliceStable(hits, func(i, j int) bool {
		iTimestamp := hits[i].(map[string]interface{})["_source"].(map[string]interface{})["data"].(map[string]interface{})["timestamp"].(string)
		jTimestamp := hits[j].(map[string]interface{})["_source"].(map[string]interface{})["data"].(map[string]interface{})["timestamp"].(string)
		return iTimestamp < jTimestamp
	})

	// Convert sorted hits to LocationData
	locations := extractLocationData(hits)

	return locations, nil
}

func HandleMapDataForTrip(c *fiber.Ctx, settings *config.Settings, tripID, startTime, endTime string) error {
	tokenID, exists := TripIDToTokenIDMap[tripID]
	if !exists {
		return c.Status(fiber.StatusNotFound).SendString("Trip not found")
	}

	log.Info().Msgf("HandleMapDataForTrip: TripID: %s, StartTime: %s, EndTime: %s, TokenID: %d", tripID, startTime, endTime, tokenID)

	// Fetch historical data for the specific trip
	locations, err := queryDeviceDataHistory(tokenID, startTime, endTime, settings, c)

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch historical data: " + err.Error()})
	}

	// Convert the historical data to GeoJSON
	geoJSON := convertToGeoJSON(locations, tripID, startTime, endTime)

	geoJSONData, err := json.Marshal(geoJSON)
	if err != nil {
		log.Error().Msgf("Error with GeoJSON: %v", err)
	} else {
		log.Info().Msgf("GeoJSON data: %s", string(geoJSONData))
	}
	return c.JSON(geoJSON)
}

func extractLocationData(hits []interface{}) []LocationData {
	locations := make([]LocationData, len(hits))
	for i, hit := range hits {
		hitMap := hit.(map[string]interface{})
		data := hitMap["_source"].(map[string]interface{})["data"].(map[string]interface{})
		locData := LocationData{
			Latitude:  data["latitude"].(float64),
			Longitude: data["longitude"].(float64),
		}
		locations[i] = locData
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
