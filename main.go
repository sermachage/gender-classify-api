package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
)

// ── External API shape ────────────────────────────────────────────────────────

// genderizeResponse mirrors exactly what the Genderize API sends back.
// Gender is a pointer so we can distinguish between `"gender": null`
// and the field being absent — both mean no prediction.
type genderizeResponse struct {
	Name        string  `json:"name"`
	Gender      *string `json:"gender"` // pointer: nil means null in JSON
	Probability float64 `json:"probability"`
	Count       int     `json:"count"`
}

// ── Our API shapes ────────────────────────────────────────────────────────────

type successData struct {
	Name        string  `json:"name"`
	Gender      string  `json:"gender"`
	Probability float64 `json:"probability"`
	SampleSize  int     `json:"sample_size"`
	IsConfident bool    `json:"is_confident"`
	ProcessedAt string  `json:"processed_at"`
}

type successResponse struct {
	Status string      `json:"status"`
	Data   successData `json:"data"`
}

type errorResponse struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// ── HTTP helpers ──────────────────────────────────────────────────────────────

// writeJSON sets headers and encodes v as JSON with the given status code.
// Every response goes through here so CORS and Content-Type are never missed.
func writeJSON(w http.ResponseWriter, statusCode int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(v) // nolint: errcheck — nothing to do if write fails
}

func writeError(w http.ResponseWriter, statusCode int, message string) {
	writeJSON(w, statusCode, errorResponse{
		Status:  "error",
		Message: message,
	})
}

// ── Genderize client ──────────────────────────────────────────────────────────

// httpClient has a timeout so we never block a goroutine indefinitely.
// We leave room for internal processing within the 500ms budget.
var httpClient = &http.Client{Timeout: 4 * time.Second}

func fetchGenderize(name string) (*genderizeResponse, error) {
	url := fmt.Sprintf("https://api.genderize.io/?name=%s", name)

	resp, err := httpClient.Get(url)
	if err != nil {
		return nil, fmt.Errorf("upstream unreachable: %w", err)
	}
	defer resp.Body.Close()

	var result genderizeResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("invalid upstream response: %w", err)
	}
	return &result, nil
}

// ── Handler ───────────────────────────────────────────────────────────────────

func classifyHandler(w http.ResponseWriter, r *http.Request) {

	// ── 1. Method guard (optional but good practice) ──────────────────────
	if r.Method != http.MethodGet {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}

	// ── 2. Input validation ───────────────────────────────────────────────

	// r.URL.Query() returns a map[string][]string.
	// If the key appears more than once (?name=a&name=b) it means the caller
	// sent a non-singular (array-like) value — treat as 422.
	nameValues, exists := r.URL.Query()["name"]

	if !exists || len(nameValues) == 0 || nameValues[0] == "" {
		// Missing or empty
		writeError(w, http.StatusBadRequest, "name parameter is required")
		return
	}

	if len(nameValues) > 1 {
		// Multiple values supplied — not a plain string
		writeError(w, http.StatusUnprocessableEntity, "name must be a single string value")
		return
	}

	name := nameValues[0]

	// ── 3. Call Genderize ─────────────────────────────────────────────────
	gResp, err := fetchGenderize(name)
	if err != nil {
		log.Printf("genderize error: %v", err)
		writeError(w, http.StatusBadGateway, "failed to reach gender prediction service")
		return
	}

	// ── 4. Genderize edge cases ───────────────────────────────────────────
	// gender == null OR count == 0 means the API has no prediction.
	if gResp.Gender == nil || gResp.Count == 0 {
		writeError(w, http.StatusUnprocessableEntity,
			"No prediction available for the provided name")
		return
	}

	// ── 5. Process & build response ───────────────────────────────────────
	sampleSize := gResp.Count
	isConfident := gResp.Probability >= 0.7 && sampleSize >= 100
	processedAt := time.Now().UTC().Format(time.RFC3339)

	writeJSON(w, http.StatusOK, successResponse{
		Status: "success",
		Data: successData{
			Name:        gResp.Name,
			Gender:      *gResp.Gender, // safe: nil check done above
			Probability: gResp.Probability,
			SampleSize:  sampleSize,
			IsConfident: isConfident,
			ProcessedAt: processedAt,
		},
	})
}

// ── Entry point ───────────────────────────────────────────────────────────────

func main() {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/classify", classifyHandler)

	addr := ":8080"
	log.Printf("server listening on %s", addr)

	server := &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil {
		log.Fatalf("server error: %v", err)
	}
}
