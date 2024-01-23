package main

import (
	"encoding/json"
	"log"
	"net/http"
)

type ChallengeResponse struct {
	State     string `json:"state"`
	Challenge string `json:"challenge"`
}

type requestBody struct {
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

// HandleGenerateChallenge generates a challenge for the user to sign
func HandleGenerateChallenge(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	resp := ChallengeResponse{
		State:     "exampleState",
		Challenge: "Please sign this message to log in.",
	}

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(resp)
	if err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

// HandleSubmitChallenge verifies the challenge signed by the user
func HandleSubmitChallenge(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var body requestBody

	err := json.NewDecoder(r.Body).Decode(&body)
	if err != nil {
		http.Error(w, "Bad request", http.StatusBadRequest)
		return
	}

	// TODO: signature verification

	log.Printf("Received challenge submission: %+v\n", body)
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Challenge submission received"))
}

func main() {
	log.Println("Server is starting...")

	http.HandleFunc("/auth/web3/generate_challenge", HandleGenerateChallenge)
	http.HandleFunc("/auth/web3/submit_challenge", HandleSubmitChallenge)

	// Start the server
	err := http.ListenAndServe(":3003", nil)
	if err != nil {
		log.Fatal("Server failed to start:", err)
	}
}
