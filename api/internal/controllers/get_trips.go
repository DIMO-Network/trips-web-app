package controllers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"sort"
	"strconv"
	"time"

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

type TelemetryAPIResponse struct {
	Data struct {
		Signals struct {
			CurrentLocationLongitude []signal `json:"currentLocationLongitude"`
			CurrentLocationLatitude  []signal `json:"currentLocationLatitude"`
			Speed                    []signal `json:"speed"`
		} `json:"signals"`
	} `json:"data"`

	Errors []struct {
		Message string   `json:"message"`
		Path    []string `json:"path"`
	} `json:"errors"`
}

type signal struct {
	Timestamp time.Time `json:"timestamp"`
	Value     float64   `json:"value"`
}

type locationInfo struct {
	speed *signal
	lat   *signal
	long  *signal
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

func queryTelemetryData(tokenID int64, startTime string, endTime string, settings *config.Settings, c *fiber.Ctx) ([]LocationData, error) {
	graphqlQuery := fmt.Sprintf(` 
	{
	  signals(
		tokenID: %d
		interval: "30s"
		from: "%s"
		to: "%s"
	  ) {
		timestamp
		speed(agg: {type: MAX})
		powertrainRange(agg: {type: RAND})
	  }
	}`, tokenID, startTime, endTime)

	requestPayload := GraphQLRequest{Query: graphqlQuery}
	payloadBytes, err := json.Marshal(requestPayload)
	if err != nil {
		return nil, err
	}

	privilegeToken, err := RequestPriviledgeToken(c, settings, tokenID)
	if err != nil {
		return nil, errors.Wrap(err, "error getting privilege token")
	}

	req, err := http.NewRequest("POST", settings.TelemetryAPIURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", *privilegeToken))

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

	var respData TelemetryAPIResponse
	if err := json.Unmarshal(body, &respData); err != nil {
		return nil, err
	}
	if len(respData.Errors) > 0 {
		log.Error().Interface("errors", respData.Errors).Msg("Error in telemetry API response")
	}
	log.Info().Interface("response", respData).Msg("Telemetry API response")

	// Create a map to store location information by timestamp
	tsMap := make(map[time.Time]*locationInfo)

	// Determine the maximum length of the signals arrays
	maxLen := len(respData.Data.Signals.CurrentLocationLongitude)
	if len(respData.Data.Signals.CurrentLocationLatitude) > maxLen {
		maxLen = len(respData.Data.Signals.CurrentLocationLatitude)
	}
	if len(respData.Data.Signals.Speed) > maxLen {
		maxLen = len(respData.Data.Signals.Speed)
	}

	// Populate the map with data
	for i := 0; i < maxLen; i++ {
		if i < len(respData.Data.Signals.CurrentLocationLongitude) {
			signal := respData.Data.Signals.CurrentLocationLongitude[i]
			if tsMap[signal.Timestamp] == nil {
				tsMap[signal.Timestamp] = &locationInfo{}
			}
			tsMap[signal.Timestamp].long = &signal
		}

		if i < len(respData.Data.Signals.CurrentLocationLatitude) {
			signal := respData.Data.Signals.CurrentLocationLatitude[i]
			if tsMap[signal.Timestamp] == nil {
				tsMap[signal.Timestamp] = &locationInfo{}
			}
			tsMap[signal.Timestamp].lat = &signal
		}

		if i < len(respData.Data.Signals.Speed) {
			signal := respData.Data.Signals.Speed[i]
			if tsMap[signal.Timestamp] == nil {
				tsMap[signal.Timestamp] = &locationInfo{}
			}
			tsMap[signal.Timestamp].speed = &signal
		}
	}

	// Extract sorted timestamps
	keys := make([]time.Time, 0, len(tsMap))
	for key := range tsMap {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		return keys[i].Before(keys[j])
	})

	// Extract location data based on the map
	locations := make([]LocationData, 0, len(keys))
	for _, tsKey := range keys {
		locInfo := tsMap[tsKey]
		if locInfo.lat == nil || locInfo.long == nil || locInfo.speed == nil {
			continue
		}
		loc := LocationData{
			Timestamp: tsKey.String(),
			Latitude:  locInfo.lat.Value,
			Longitude: locInfo.long.Value,
			Speed:     locInfo.speed.Value,
		}
		locations = append(locations, loc)
	}

	// locations := make([]LocationData, 0, len(respData.Data.Signals.CurrentLocationLongitude))
	// for _, tsKey := range keys {
	// 	loc := LocationData{Timestamp: tsKey}
	// 	locInfo := tsMap[tsKey]
	// 	if locInfo.lat != nil && locInfo.long != nil {
	// 		loc.Latitude = locInfo.lat.Value
	// 		loc.Longitude = locInfo.lat.Value
	// 	}
	// 	if locInfo.speed != nil{
	// 		loc.Speed =  locInfo.speed.Value
	// 	}
	// 	locations = append(locations, loc) // len 10 partial data
	// }
	return locations, nil
}

func HandleMapDataForTrip(c *fiber.Ctx, settings *config.Settings, tripID, startTime, endTime string) error {
	tokenID, exists := TripIDToTokenIDMap[tripID]
	if !exists {
		log.Error().Msgf("Trip not found for tripID: %s", tripID) // Log trip not found
		return c.Status(fiber.StatusNotFound).SendString("Trip not found")
	}

	log.Info().Msgf("Fetching map data for TripID: %s, StartTime: %s, EndTime: %s, TokenID: %d", tripID, startTime, endTime, tokenID)

	locations, err := queryTelemetryData(tokenID, startTime, endTime, settings, c)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch historical data")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch historical data: " + err.Error()})
	}

	if len(locations) == 0 {
		log.Warn().Msg("No location data received")
	}

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
