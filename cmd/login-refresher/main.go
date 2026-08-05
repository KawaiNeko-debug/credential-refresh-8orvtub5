package main

import (
	"bytes"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"credential-refresher/internal/login"
	"credential-refresher/internal/refreshapi"
)

func main() {
	managerURL := flag.String("manager-url", envOr("REFRESH_MANAGER_URL", ""), "external Manager URL")
	jobID := flag.String("job-id", envOr("REFRESH_JOB_ID", ""), "credential refresh job id")
	groupIndex := flag.Int("group-index", envIntOr("REFRESH_GROUP_INDEX", 0), "credential refresh group index")
	token := flag.String("token", envOr("REFRESH_MANAGER_TOKEN", ""), "external Manager bearer token")
	chromePath := flag.String("chrome", envOr("REFRESH_BROWSER_PATH", ""), "Chrome/Edge executable path")
	proxy := flag.String("proxy", envOr("REFRESH_PROXY", ""), "optional browser proxy")
	concurrency := flag.Int("concurrency", envIntOr("REFRESH_CONCURRENCY", 0), "browser concurrency")
	timeout := flag.Duration("timeout", envDurationOr("REFRESH_LOGIN_TIMEOUT", 5*time.Minute), "single login timeout")
	headless := flag.Bool("headless", envBoolOr("REFRESH_HEADLESS", true), "run browser in headless mode")
	flag.Parse()

	if strings.TrimSpace(*managerURL) == "" || strings.TrimSpace(*jobID) == "" || strings.TrimSpace(*token) == "" {
		log.Fatal("manager-url, job-id and token are required")
	}
	target, err := login.TargetFromMarker(os.Getenv("TARGET_MARKER"))
	if err != nil {
		log.Fatal(err)
	}

	client := &http.Client{Timeout: envDurationOr("REFRESH_MANAGER_TIMEOUT", 5*time.Minute)}
	ctx := context.Background()
	group, err := fetchGroup(ctx, client, *managerURL, *jobID, *groupIndex, *token)
	if err != nil {
		log.Fatalf("fetch group: %v", err)
	}
	workers := *concurrency
	if workers <= 0 {
		workers = group.BrowsersPerRunner
	}
	if workers <= 0 {
		workers = 1
	}

	runner := login.NewRunner(login.Config{
		Target:       target,
		Proxy:        *proxy,
		ChromePath:   *chromePath,
		Timeout:      *timeout,
		QueueTimeout: time.Duration(len(group.Accounts)+workers) * *timeout,
		Headless:     *headless,
		Workers:      workers,
		Logger: func(event login.LogEvent) {
			log.Printf("%s %s %s %s", event.Level, event.Type, event.CustomerCode, event.Message)
		},
	})
	results := refreshAccounts(ctx, runner, group.Accounts, workers)
	if err := postResults(ctx, client, *managerURL, *jobID, *groupIndex, *token, results); err != nil {
		log.Fatalf("post results: %v", err)
	}
	status, message := completionStatus(results)
	if err := completeGroup(ctx, client, *managerURL, *jobID, *groupIndex, *token, status, message); err != nil {
		log.Fatalf("complete group: %v", err)
	}
	log.Printf("group %d complete: %d accounts", *groupIndex, len(results))
}

func refreshAccounts(ctx context.Context, runner *login.Runner, accounts []refreshapi.JobAccount, workers int) []refreshapi.AccountResult {
	if workers <= 0 {
		workers = 1
	}
	results := make([]refreshapi.AccountResult, len(accounts))
	var wg sync.WaitGroup
	sem := make(chan struct{}, workers)
	for index, account := range accounts {
		index, account := index, account
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			code := refreshapi.NormalizeCustomerCode(account.CustomerCode)
			if code == "" {
				results[index] = refreshapi.AccountResult{Success: false, Message: "customerCode is required"}
				return
			}
			log.Printf("refreshing %s", code)
			result, err := runner.Login(ctx, code, account.Password)
			if err != nil {
				log.Printf("refresh %s failed: %v", code, err)
				results[index] = refreshapi.AccountResult{CustomerCode: code, Success: false, Message: err.Error()}
				return
			}
			voucher := result.CanUseVoucher
			results[index] = refreshapi.AccountResult{
				CustomerCode:      result.CustomerCode,
				Success:           true,
				Ticket:            result.Ticket,
				PrimarySession:    result.PrimarySession,
				GroupSession:      result.GroupSession,
				MobileAccessToken: result.MobileAccessToken,
				CanUseVoucher:     &voucher,
				Message:           "ok",
			}
		}()
	}
	wg.Wait()
	return results
}

func completionStatus(results []refreshapi.AccountResult) (string, string) {
	for _, result := range results {
		if !result.Success {
			return refreshapi.GroupStatusFailed, "completed with failures"
		}
	}
	return refreshapi.GroupStatusCompleted, "completed"
}

func fetchGroup(ctx context.Context, client *http.Client, baseURL, jobID string, groupIndex int, token string) (refreshapi.GroupAccountsResponse, error) {
	var out refreshapi.GroupAccountsResponse
	endpoint := fmt.Sprintf("%s/api/v1/external/login-refresh-jobs/%s/groups/%d", strings.TrimRight(baseURL, "/"), jobID, groupIndex)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return out, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	body, status, err := doJSON(client, req)
	if err != nil {
		return out, err
	}
	if status < 200 || status >= 300 {
		return out, fmt.Errorf("manager returned %d: %s", status, string(body))
	}
	if err := json.Unmarshal(body, &out); err != nil {
		return out, err
	}
	return out, nil
}

func postResults(ctx context.Context, client *http.Client, baseURL, jobID string, groupIndex int, token string, results []refreshapi.AccountResult) error {
	body, err := json.Marshal(refreshapi.ResultRequest{Results: results})
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/api/v1/external/login-refresh-jobs/%s/groups/%d/results", strings.TrimRight(baseURL, "/"), jobID, groupIndex)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	responseBody, status, err := doJSON(client, req)
	if err != nil {
		return err
	}
	if status < 200 || status >= 300 {
		return fmt.Errorf("manager returned %d: %s", status, string(responseBody))
	}
	return nil
}

func completeGroup(ctx context.Context, client *http.Client, baseURL, jobID string, groupIndex int, token, status, message string) error {
	body, err := json.Marshal(refreshapi.GroupCompleteRequest{Status: status, Message: message})
	if err != nil {
		return err
	}
	endpoint := fmt.Sprintf("%s/api/v1/external/login-refresh-jobs/%s/groups/%d/complete", strings.TrimRight(baseURL, "/"), jobID, groupIndex)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", "application/json")
	responseBody, statusCode, err := doJSON(client, req)
	if err != nil {
		return err
	}
	if statusCode < 200 || statusCode >= 300 {
		return fmt.Errorf("manager returned %d: %s", statusCode, string(responseBody))
	}
	return nil
}

func doJSON(client *http.Client, req *http.Request) ([]byte, int, error) {
	resp, err := client.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	return body, resp.StatusCode, err
}

func envOr(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}

func envDurationOr(key string, fallback time.Duration) time.Duration {
	value := envOr(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envIntOr(key string, fallback int) int {
	value := envOr(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
}

func envBoolOr(key string, fallback bool) bool {
	value := envOr(key, "")
	if value == "" {
		return fallback
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return fallback
	}
	return parsed
}
