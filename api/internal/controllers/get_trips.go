package controllers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strconv"

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
	Speed     float64 `json:"speed"`
	Timestamp string  `json:"timestamp"`
}

var SpeedGradient = []struct {
	Threshold float64
	Color     string
}{
	{10, "blue"},
	{30, "green"},
	{50, "yellow"},
	{70, "orange"},
	{90, "red"},
}

type TripsController struct {
	settings config.Settings
}

func NewTripsController(settings config.Settings) TripsController {
	return TripsController{settings: settings}
}

func (t *TripsController) HandleTripsList(c *fiber.Ctx) error {
	tokenID, err := strconv.ParseInt(c.Params("tokenid"), 10, 64)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid token ID",
		})
	}

	trips, err := QueryTripsAPI(tokenID, &t.settings, c)
	if err != nil {
		log.Error().Err(err).Msg("Failed to query trips API")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to fetch trips",
		})
	}

	return c.Render("vehicle_trips", fiber.Map{
		"TokenID": tokenID,
		"Trips":   trips,
	})
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

	responseBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var result map[string]interface{}
	if err := json.Unmarshal(responseBody, &result); err != nil {
		return nil, err
	}

	hitsInterface, ok := result["hits"].(map[string]interface{})["hits"]
	if !ok {
		return nil, errors.New("unexpected data structure for hits")
	}

	hits, ok := hitsInterface.([]interface{})
	if !ok {
		return nil, errors.New("unexpected data structure for hits array")
	}

	locations, err := extractLocationData(hits)
	if err != nil {
		return nil, err
	}

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
	speedGradient := calculateSpeedGradient(locations)

	geoJSONData, err := json.Marshal(geoJSON)
	if err != nil {
		log.Error().Msgf("Error with GeoJSON: %v", err)
	} else {
		log.Info().Msgf("GeoJSON data: %s", string(geoJSONData))
	}

	response := map[string]interface{}{
		"geojson":       geoJSON,
		"speedGradient": speedGradient,
	}

	return c.JSON(response)
}

func extractLocationData(hits []interface{}) ([]LocationData, error) {
	// Sort the hits based on the timestamp
	sort.SliceStable(hits, func(i, j int) bool {
		iData := hits[i].(map[string]interface{})["_source"].(map[string]interface{})["data"].(map[string]interface{})
		jData := hits[j].(map[string]interface{})["_source"].(map[string]interface{})["data"].(map[string]interface{})
		iTimestamp, iOk := iData["timestamp"].(string)
		jTimestamp, jOk := jData["timestamp"].(string)
		return iOk && jOk && iTimestamp < jTimestamp
	})

	locations := make([]LocationData, 0, len(hits))
	for _, hitInterface := range hits {
		hitMap, _ := hitInterface.(map[string]interface{})
		source, _ := hitMap["_source"].(map[string]interface{})
		data, _ := source["data"].(map[string]interface{})

		latitude, _ := data["latitude"].(float64)
		longitude, _ := data["longitude"].(float64)
		speed, _ := data["speed"].(float64)
		timestamp, _ := data["timestamp"].(string)

		locData := LocationData{
			Latitude:  latitude,
			Longitude: longitude,
			Speed:     speed,
			Timestamp: timestamp,
		}
		locations = append(locations, locData)
	}

	return locations, nil
}

func convertToGeoJSON(locations []LocationData, tripID string, tripStart string, tripEnd string) *geojson.FeatureCollection {
	featureCollection := geojson.NewFeatureCollection()

	for _, loc := range locations {
		// Create a new point feature with the current location's coordinates
		point := geojson.NewPointFeature([]float64{loc.Longitude, loc.Latitude})

		// Add properties to the point feature, including speed and timestamp
		point.Properties["speed"] = loc.Speed
		point.Properties["timestamp"] = loc.Timestamp

		// Add additional properties as needed
		point.Properties["trip_id"] = tripID
		point.Properties["trip_start"] = tripStart
		point.Properties["trip_end"] = tripEnd
		point.Properties["privacy_zone"] = 1
		point.Properties["color"] = "black"
		point.Properties["point-color"] = "black"

		// Append the point feature to the feature collection
		featureCollection.AddFeature(point)
	}

	return featureCollection
}

func calculateSpeedGradient(locations []LocationData) []string {
	colors := make([]string, len(locations))
	for i, loc := range locations {
		colors[i] = getSpeedColor(loc.Speed)
	}
	return colors
}

func getSpeedColor(speed float64) string {
	for _, sg := range SpeedGradient {
		if speed <= sg.Threshold {
			return sg.Color
		}
	}
	return "black"
}
