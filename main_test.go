package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestDelayHandler_ValidDelay(t *testing.T) {
	mux := setupRoutes()

	start := time.Now()
	req := httptest.NewRequest(http.MethodGet, "/delay/0.1", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)
	elapsed := time.Since(start)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	if elapsed < 90*time.Millisecond {
		t.Errorf("expected elapsed time to be at least ~100ms, got %v", elapsed)
	}

	var resp DelayResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}

	if resp.Status != "success" || resp.DelaySeconds != 0.1 {
		t.Errorf("unexpected response content: %+v", resp)
	}
}

func TestDelayHandler_ZeroDelay(t *testing.T) {
	mux := setupRoutes()

	req := httptest.NewRequest(http.MethodGet, "/delay/0", nil)
	rr := httptest.NewRecorder()

	mux.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200 OK, got %d. Body: %s", rr.Code, rr.Body.String())
	}

	var resp DelayResponse
	if err := json.Unmarshal(rr.Body.Bytes(), &resp); err != nil {
		t.Fatalf("failed to decode response JSON: %v", err)
	}

	if resp.Status != "success" || resp.DelaySeconds != 0 {
		t.Errorf("unexpected response content: %+v", resp)
	}
}

func TestDelayHandler_InvalidDelay(t *testing.T) {
	mux := setupRoutes()

	tests := []struct {
		name string
		path string
	}{
		{"negative number", "/delay/-1"},
		{"non-numeric string", "/delay/abc"},
		{"exceeds max delay", "/delay/301"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rr := httptest.NewRecorder()

			mux.ServeHTTP(rr, req)

			if rr.Code != http.StatusBadRequest {
				t.Errorf("expected status 400 Bad Request, got %d. Body: %s", rr.Code, rr.Body.String())
			}

			var errResp ErrorResponse
			if err := json.Unmarshal(rr.Body.Bytes(), &errResp); err != nil {
				t.Fatalf("failed to decode error JSON: %v", err)
			}
			if errResp.Error == "" {
				t.Errorf("expected error message in response, got empty")
			}
		})
	}
}

func TestDelayHandler_ClientCancellation(t *testing.T) {
	mux := setupRoutes()

	ctx, cancel := context.WithCancel(context.Background())
	req := httptest.NewRequest(http.MethodGet, "/delay/2", nil).WithContext(ctx)
	rr := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		mux.ServeHTTP(rr, req)
		close(done)
	}()

	// Cancel after 50ms
	time.Sleep(50 * time.Millisecond)
	cancel()

	select {
	case <-done:
		if rr.Code != http.StatusRequestTimeout {
			t.Errorf("expected status 408 Request Timeout on cancellation, got %d", rr.Code)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("handler did not return promptly after context cancellation")
	}
}
