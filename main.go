package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net"
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

// privateIPv4 returns the first non-loopback private IPv4 address of this host.
func privateIPv4() (string, error) {
	addrs, err := net.InterfaceAddrs()
	if err != nil {
		return "", err
	}
	for _, a := range addrs {
		ipNet, ok := a.(*net.IPNet)
		if !ok || ipNet.IP.IsLoopback() {
			continue
		}
		if ip4 := ipNet.IP.To4(); ip4 != nil && ip4.IsPrivate() {
			return ip4.String(), nil
		}
	}
	return "", errors.New("no private IPv4 address found")
}

func main() {
	host := os.Getenv("HOST")
	if host == "" {
		host = "127.0.0.1"
	}
	if host == "auto" {
		ip, err := privateIPv4()
		if err != nil {
			log.Fatalf("Cannot resolve HOST=auto: %v", err)
		}
		host = ip
	}
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
