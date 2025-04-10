package controllers

import (
	"fmt"
	"github.com/google/uuid"
	"time"

	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/patrickmn/go-cache"
	"github.com/pkg/errors"
)

var CacheInstance = cache.New(cache.NoExpiration, 10*time.Hour)

type ChallengeResponse struct {
	State     string `json:"state"`
	Challenge string `json:"challenge"`
}

func ExtractEthereumAddressFromToken(tokenString string) (string, error) {
	// Parsing the token without validating its signature
	token, _, err := new(jwt.Parser).ParseUnverified(tokenString, jwt.MapClaims{})
	if err != nil {
		fmt.Println("Error parsing token:", err)
		return "", fmt.Errorf("error parsing token")
	}

	// Asserting the type of the claims to access the data
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", errors.New("invalid claims type")
	}

	ethAddress, ok := claims["ethereum_address"].(string)
	if !ok {
		return "", errors.New("ethereum address not found in JWT")
	}

	return ethAddress, nil
}

func AuthMiddleware() fiber.Handler {
	return func(c *fiber.Ctx) error {
		// check if session_id cookie exists
		sessionCookie := c.Cookies("session_id")
		if sessionCookie == "" {
			fmt.Println("No session_id cookie")
			return c.Render("session_expired", fiber.Map{})
		}

		// check if the session_id is in the cache
		jwtToken, found := CacheInstance.Get(sessionCookie)
		if !found {
			fmt.Println("Session expired")
			return c.Render("session_expired", fiber.Map{})
		}

		// check if main auth jwt token has expired here, if not show same render of session expired etc.
		ethAddress, err := ExtractEthereumAddressFromToken(jwtToken.(string))
		if err != nil {
			fmt.Println("Error extracting ethereum address from token:", err)
			return c.Render("session_expired", fiber.Map{})
		}

		c.Locals("ethereum_address", ethAddress)

		return c.Next()
	}
}

// JwtRequest represents the expected POST JSON payload.
type JwtRequest struct {
	Jwt string `json:"jwt"`
}

// PersistJwtHandler handles the POST request containing the JWT for our session
func PersistJwtHandler(c *fiber.Ctx) error {
	var req JwtRequest

	// Parse the JSON body into our request struct.
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request payload",
		})
	}
	sessionID := uuid.New().String()
	CacheInstance.Set(sessionID, req.Jwt, 2*time.Hour)

	// Return the session_id as JSON.
	return c.JSON(fiber.Map{
		"session_id": sessionID,
	})
}
