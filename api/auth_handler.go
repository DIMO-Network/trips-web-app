package main

import (
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/pkg/errors"
)

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
		// Retrieve the session_id from the request cookie
		sessionCookie := c.Cookies("session_id")

		// Check if the session_id is in the cache
		jwtToken, found := CacheInstance.Get(sessionCookie)
		if !found {
			return c.Status(fiber.StatusUnauthorized).SendString("Unauthorized")
		}

		ethAddress, err := ExtractEthereumAddressFromToken(jwtToken.(string))
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).SendString("Invalid token: " + err.Error())
		}

		c.Locals("ethereum_address", ethAddress)

		return c.Next()
	}
}
