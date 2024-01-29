package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
)

const (
	clientID     = "trips-signals-webapp"
	domain       = "https://localhost:3003/oauth/callback"
	scope        = "openid email"
	responseType = "code"
	grantType    = "authorization_code"
)

type ChallengeResponse struct {
	State     string `json:"state"`
	Challenge string `json:"challenge"`
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

func HandleGenerateChallenge(w http.ResponseWriter, r *http.Request) {
	enableCors(&w)
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	if err := r.ParseForm(); err != nil {
		http.Error(w, "Failed to parse form data", http.StatusBadRequest)
		return
	}

	address := r.FormValue("address")

	formData := url.Values{}
	formData.Add("client_id", clientID)
	formData.Add("domain", domain)
	formData.Add("scope", scope)
	formData.Add("response_type", responseType)
	formData.Add("address", address)

	encodedFormData := formData.Encode()
	reqURL := "https://auth.dev.dimo.zone/auth/web3/generate_challenge"
	resp, err := http.Post(reqURL, "application/x-www-form-urlencoded", strings.NewReader(encodedFormData))
	if err != nil {
		http.Error(w, "Failed to make request to external service", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	log.Printf("body: %s", string(body))

	if err != nil {
		log.Printf("Error reading response body: %v", err)
		http.Error(w, "Error reading external response", http.StatusInternalServerError)
	}

	var apiResp ChallengeResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		log.Printf("Error unmarshalling external response: %v", err)
		http.Error(w, "Error processing response from external service", http.StatusInternalServerError)
		return
	}

	if apiResp.State == "" || apiResp.Challenge == "" {
		log.Printf("State or Challenge is empty")
		http.Error(w, "State or Challenge incomplete from external service", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(apiResp)
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

	state := r.FormValue("state")
	signature := r.FormValue("signature")

	formData := url.Values{}
	formData.Add("client_id", clientID)
	formData.Add("domain", domain)
	formData.Add("grant_type", grantType)
	formData.Add("state", state)
	formData.Add("signature", signature)

	encodedFormData := formData.Encode()

	reqURL := "https://auth.dev.dimo.zone/auth/web3/submit_challenge"
	resp, err := http.Post(reqURL, "application/x-www-form-urlencoded", strings.NewReader(encodedFormData))
	if err != nil {
		http.Error(w, "Failed to make request to external service", http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, "Failed to read response from external service", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

func main() {
	log.Println("Server is starting...")

	http.HandleFunc("/auth/web3/generate_challenge", corsHandler(HandleGenerateChallenge))
	http.HandleFunc("/auth/web3/submit_challenge", corsHandler(HandleSubmitChallenge))

	log.Fatal(http.ListenAndServe(":3003", nil))
}
