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
	ID    string
	Start TripPoint
	End   TripPoint
}

type TripPoint struct {
	Time              string
	Location          LatLon
	EstimatedLocation *LatLon
}

// LatLon represents latitude and longitude coordinates.
type LatLon struct {
	Latitude  float64
	Longitude float64
}

type TimeEntry struct {
	Time string `json:"time"`
}

type TripsResponse struct {
	Trips []Trip `json:"trips"`
}

var TripIDToTokenIDMap = make(map[string]int64)

type LocationData struct {
	Longitude *float64
	Latitude  *float64
	Speed     *float64
	Timestamp string
}

type TelemetryAPIResponse struct {
	Data struct {
		Signals []Signal `json:"signals"`
	} `json:"data"`

	Errors []struct {
		Message string   `json:"message"`
		Path    []string `json:"path"`
	} `json:"errors"`
}

type Signal struct {
	Timestamp                time.Time `json:"timestamp"`
	CurrentLocationLongitude *float64  `json:"currentLocationLongitude"`
	CurrentLocationLatitude  *float64  `json:"currentLocationLatitude"`
	Speed                    *float64  `json:"speed"`
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
		speed(agg: MAX)
		currentLocationLatitude(agg: AVG)
		currentLocationLongitude(agg: AVG)
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

	locations := make([]LocationData, 0, len(respData.Data.Signals))
	for _, signal := range respData.Data.Signals {
		loc := LocationData{
			Timestamp: signal.Timestamp.String(),
			Latitude:  signal.CurrentLocationLatitude,
			Longitude: signal.CurrentLocationLongitude,
			Speed:     signal.Speed,
		}
		locations = append(locations, loc)
	}

	return locations, nil
}

func HandleMapDataForTrip(c *fiber.Ctx, settings *config.Settings, tripID, startTime, endTime string, estimatedStart *LatLon) error {
	tokenID, exists := TripIDToTokenIDMap[tripID]
	if !exists {
		log.Error().Msgf("Trip not found for tripID: %s", tripID)
		return c.Status(fiber.StatusNotFound).SendString("Trip not found")
	}

	log.Info().Msgf("Fetching map data for TripID: %s, StartTime: %s, EndTime: %s, TokenID: %d", tripID, startTime, endTime, tokenID)

	// Fetch telemetry data
	locations, err := queryTelemetryData(tokenID, startTime, endTime, settings, c)
	if err != nil {
		log.Error().Err(err).Msg("Failed to fetch historical data")
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to fetch historical data: " + err.Error()})
	}

	if len(locations) == 0 {
		log.Warn().Msg("No location data received")
	}

	geoJSON := convertToGeoJSON(locations, estimatedStart)
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

func convertToGeoJSON(locations []LocationData, estimatedStart *LatLon) *geojson.FeatureCollection {
	featureCollection := geojson.NewFeatureCollection()

	// Add the estimated start location if it exists
	if estimatedStart != nil {
		estimatedStartFeature := geojson.NewPointFeature([]float64{estimatedStart.Longitude, estimatedStart.Latitude})
		estimatedStartFeature.Properties["point_type"] = "estimated_start"
		estimatedStartFeature.Properties["privacy_zone"] = 1
		estimatedStartFeature.Properties["color"] = "black"
		featureCollection.AddFeature(estimatedStartFeature)
	}

	// Iterate through the locations and add each as a point feature
	for i, loc := range locations {
		if loc.Longitude != nil && loc.Latitude != nil {
			point := geojson.NewPointFeature([]float64{*loc.Longitude, *loc.Latitude})
			if loc.Speed != nil {
				point.Properties["speed"] = *loc.Speed
			}
			point.Properties["timestamp"] = loc.Timestamp
			point.Properties["privacy_zone"] = 1
			point.Properties["color"] = "black"

			// Mark the last point as the end point
			if i == len(locations)-1 {
				point.Properties["point_type"] = "end"
				point.Properties["privacy_zone"] = 1
				point.Properties["color"] = "red"
			}

			featureCollection.AddFeature(point)
		}
	}

	return featureCollection
}

func calculateSpeedGradient(locations []LocationData) []string {
	colors := make([]string, len(locations))
	for i, loc := range locations {
		if loc.Speed != nil {
			colors[i] = getSpeedColor(*loc.Speed)
		} else {
			colors[i] = "black" // Default color if speed data is missing
		}
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
