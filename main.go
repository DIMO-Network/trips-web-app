package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
)

type ChallengeResponse struct {
	State     string `json:"state"`
	Challenge string `json:"challenge"`
}

type VerifyRequest struct {
	ClientID  string `json:"client_id"`
	Domain    string `json:"domain"`
	GrantType string `json:"grant_type"`
	State     string `json:"state"`
	Signature string `json:"signature"`
}

func enableCors(w *http.ResponseWriter) {
	(*w).Header().Set("Access-Control-Allow-Origin", "http://localhost:3000")
	(*w).Header().Set("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
	(*w).Header().Set("Access-Control-Allow-Headers", "Accept, Content-Type, Content-Length, Authorization")
}

func corsHandler(h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		enableCors(&w)
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		h(w, r)
	}
}

func generateRandomString(n int) (string, error) {
	bytes := make([]byte, n)
	_, err := rand.Read(bytes)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

func HandleGenerateChallenge(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Parse the URL-encoded form data
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	// Extract form values
	address := r.FormValue("address")
	// etc

	// Generate state and nonce
	state, err := generateRandomString(16) // 16 bytes = 128 bits
	if err != nil {
		http.Error(w, "Error generating state", http.StatusInternalServerError)
		return
	}

	nonce, err := generateRandomString(32) // 32 bytes = 256 bits
	if err != nil {
		http.Error(w, "Error generating nonce", http.StatusInternalServerError)
		return
	}

	// Construct the challenge message
	challenge := fmt.Sprintf("Please verify ownership of the address %s by signing this random string: %s", address, nonce)

	// Send the JSON response
	resp := ChallengeResponse{
		State:     state,
		Challenge: challenge,
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func HandleSubmitChallenge(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	// Extract form values
	clientID := r.FormValue("client_id")
	domain := r.FormValue("domain")
	grantType := r.FormValue("grant_type")
	state := r.FormValue("state")
	signature := r.FormValue("signature")

	// TODO: Implement actual signature verification logic
	log.Printf("Received challenge submission: clientID=%s, domain=%s, grantType=%s, state=%s, signature=%s\n",
		clientID, domain, grantType, state, signature)

	// signature verification

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Challenge submission received"))
}

func main() {
	log.Println("Server is starting...")

	http.HandleFunc("/auth/web3/generate_challenge", corsHandler(HandleGenerateChallenge))
	http.HandleFunc("/auth/web3/submit_challenge", corsHandler(HandleSubmitChallenge))

	log.Fatal(http.ListenAndServe(":3003", nil))
}
