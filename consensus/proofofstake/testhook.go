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

func TestHookCheck(blockNumber uint64) error {
	testHookEndpoint := defaults.GetTestHookEndpoint()
	if testHookEndpoint == "" {
		return nil
	}

	parsed, err := url.Parse(testHookEndpoint)
	if err != nil {
		log.Error("TestHookCheck: invalid endpoint URL", "endpoint", testHookEndpoint, "error", err)
		return fmt.Errorf("test hook endpoint is not a valid URL: %w", err)
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		log.Error("TestHookCheck: endpoint must be http or https", "endpoint", testHookEndpoint, "scheme", parsed.Scheme)
		return fmt.Errorf("test hook endpoint must be http or https, got scheme %q", parsed.Scheme)
	}

	base := strings.TrimSuffix(testHookEndpoint, "/")
	hookURL := fmt.Sprintf("%s/block/%d", base, blockNumber)

	log.Debug("TestHookCheck: calling hook", "url", hookURL, "blockNumber", blockNumber)

	resp, err := http.Get(hookURL)
	if err != nil {
		log.Error("TestHookCheck: HTTP request failed", "url", hookURL, "error", err)
		return fmt.Errorf("test hook request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Error("TestHookCheck: failed to read response body", "url", hookURL, "error", err)
		return fmt.Errorf("test hook: failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		log.Error("TestHookCheck: non-OK status", "url", hookURL, "status", resp.StatusCode, "body", string(body))
		return fmt.Errorf("test hook returned status %d: %s", resp.StatusCode, string(body))
	}

	response := strings.TrimSpace(string(body))
	if response != "1" {
		log.Warn("TestHookCheck: hook return non-1", "url", hookURL, "response", response, "blockNumber", blockNumber)
		return errors.New("test hook return non-1 (response was not 1)")
	}

	log.Info("TestHookCheck: hook return 1", "blockNumber", blockNumber)
	return nil
}
