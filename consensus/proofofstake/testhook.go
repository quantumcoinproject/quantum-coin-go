package proofofstake

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"

	"github.com/quantumcoinproject/quantum-coin-go/defaults"
	"github.com/quantumcoinproject/quantum-coin-go/log"
)

// TestHookCheck queries one or more test-hook endpoints for the given block number.
// The configured endpoint string may be empty, a single URL, or a comma-separated
// list of URLs. Endpoints are tried in order; if at least one endpoint responds
// with body "1", the check succeeds and nil is returned. If every endpoint fails
// (network error, non-200, or non-"1" body), the last error is returned.
func TestHookCheck(blockNumber uint64) error {
	endpoints := parseTestHookEndpoints(defaults.GetTestHookEndpoint())
	if endpoints == nil {
		return nil
	}
	if len(endpoints) == 0 {
		return nil
	}

	var lastErr error
	for _, endpoint := range endpoints {
		if err := checkSingleTestHook(endpoint, blockNumber); err != nil {
			lastErr = err
			log.Info("TestHookCheck: endpoint failed, trying next if available",
				"endpoint", endpoint, "blockNumber", blockNumber, "error", err)
			continue
		}
		log.Info("TestHookCheck: hook return 1", "endpoint", endpoint, "blockNumber", blockNumber)
		return nil
	}

	if lastErr == nil {
		lastErr = errors.New("test hook: no endpoints succeeded")
	}
	return lastErr
}

// parseTestHookEndpoints splits the raw config value on commas, trims whitespace,
// and drops empty entries. Returns nil if no usable endpoints remain.
func parseTestHookEndpoints(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}

	parts := strings.Split(raw, ",")
	endpoints := make([]string, 0, len(parts))
	for _, p := range parts {
		if trimmed := strings.TrimSpace(p); trimmed != "" {
			endpoints = append(endpoints, trimmed)
		}
	}
	if len(endpoints) == 0 {
		return nil
	}
	return endpoints
}

// validateTestHookEndpoint ensures the endpoint is a syntactically valid http(s) URL.
func validateTestHookEndpoint(endpoint string) error {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return fmt.Errorf("test hook endpoint is not a valid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return fmt.Errorf("test hook endpoint must be http or https, got scheme %q", parsed.Scheme)
	}
	return nil
}

// buildTestHookURL composes the per-block hook URL from a base endpoint.
func buildTestHookURL(endpoint string, blockNumber uint64) string {
	base := strings.TrimSuffix(endpoint, "/")
	return fmt.Sprintf("%s/block/%d", base, blockNumber)
}

// fetchTestHookResponse performs the HTTP GET and returns the trimmed body on success.
func fetchTestHookResponse(hookURL string) (string, error) {
	log.Debug("TestHookCheck: fetching endpoint URL", "url", hookURL)
	resp, err := http.Get(hookURL)
	if err != nil {
		return "", fmt.Errorf("test hook request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("test hook: failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("test hook returned status %d: %s", resp.StatusCode, string(body))
	}

	return strings.TrimSpace(string(body)), nil
}

// checkSingleTestHook validates the endpoint, calls it, and returns nil only when
// the endpoint responds with body "1".
func checkSingleTestHook(endpoint string, blockNumber uint64) error {
	if err := validateTestHookEndpoint(endpoint); err != nil {
		log.Error("TestHookCheck: invalid endpoint", "endpoint", endpoint, "error", err)
		return err
	}

	hookURL := buildTestHookURL(endpoint, blockNumber)
	log.Debug("TestHookCheck: calling hook", "url", hookURL, "blockNumber", blockNumber)

	response, err := fetchTestHookResponse(hookURL)
	if err != nil {
		log.Error("TestHookCheck: request error", "url", hookURL, "error", err)
		return err
	}

	if response != "1" {
		log.Info("TestHookCheck: hook return non-1", "url", hookURL, "response", response, "blockNumber", blockNumber)
		return fmt.Errorf("test hook %s return non-1 (response was %q)", hookURL, response)
	}

	return nil
}
