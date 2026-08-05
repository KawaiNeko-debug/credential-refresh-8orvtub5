package main

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"credential-refresher/internal/refreshapi"
)

func TestPostResultWithRetrySendsOneResultAndRetries(t *testing.T) {
	var attempts atomic.Int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var request refreshapi.ResultRequest
		if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(request.Results) != 1 || request.Results[0].CustomerCode != "ACCOUNT1" {
			t.Fatalf("unexpected request: %+v", request)
		}
		if attempts.Add(1) < 3 {
			http.Error(w, "temporary", http.StatusServiceUnavailable)
			return
		}
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	err := postResultWithRetry(context.Background(), server.Client(), server.URL, "job-1", 0, "token", refreshapi.AccountResult{
		CustomerCode: "ACCOUNT1",
		Success:      true,
	}, 3, time.Millisecond)
	if err != nil {
		t.Fatalf("post result: %v", err)
	}
	if attempts.Load() != 3 {
		t.Fatalf("attempts = %d, want 3", attempts.Load())
	}
}
