package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

// DelayResponse represents the successful JSON response structure.
type DelayResponse struct {
	Status       string  `json:"status"`
	DelaySeconds float64 `json:"delay_seconds"`
	Message      string  `json:"message"`
}

// ErrorResponse represents the error JSON response structure.
type ErrorResponse struct {
	Error string `json:"error"`
}

func delayHandler(w http.ResponseWriter, r *http.Request) {
	secondsStr := r.PathValue("seconds")
	if secondsStr == "" {
		writeJSONError(w, "missing seconds parameter", http.StatusBadRequest)
		return
	}

	seconds, err := strconv.ParseFloat(secondsStr, 64)
	if err != nil || seconds < 0 {
		writeJSONError(w, "invalid seconds parameter: must be a non-negative number", http.StatusBadRequest)
		return
	}

	const maxDelay = 300.0 // 5 minutes max
	if seconds > maxDelay {
		writeJSONError(w, fmt.Sprintf("delay exceeds maximum allowed limit of %v seconds", maxDelay), http.StatusBadRequest)
		return
	}

	delay := time.Duration(seconds * float64(time.Second))

	select {
	case <-time.After(delay):
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(DelayResponse{
			Status:       "success",
			DelaySeconds: seconds,
			Message:      fmt.Sprintf("Responded after %v second(s)", seconds),
		})
	case <-r.Context().Done():
		http.Error(w, "request cancelled", http.StatusRequestTimeout)
	}
}

func writeJSONError(w http.ResponseWriter, message string, statusCode int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Error: message})
}

func setupRoutes() *http.ServeMux {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /delay/{seconds}", delayHandler)
	return mux
}

func main() {
	// Render requires web services to bind to 0.0.0.0.
	const host = "0.0.0.0"
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	addr := fmt.Sprintf("%s:%s", host, port)
	mux := setupRoutes()

	log.Printf("Starting delay service on http://%s\n", addr)
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("Server error: %v", err)
	}
}
