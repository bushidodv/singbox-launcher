package core

import (
	"encoding/base64"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// DecodeSubscriptionContent decodes subscription content from base64 or returns plain text
// Returns decoded content and error if decoding fails
func DecodeSubscriptionContent(content []byte) ([]byte, error) {
	// Try to decode as base64
	decoded, err := base64.URLEncoding.DecodeString(strings.TrimSpace(string(content)))
	if err != nil {
		// If URL encoding fails, try standard encoding
		decoded, err = base64.StdEncoding.DecodeString(strings.TrimSpace(string(content)))
		if err != nil {
			// If both fail, assume it's plain text
			log.Printf("DecodeSubscriptionContent: Content is not base64, treating as plain text")
			return content, nil
		}
	}
	return decoded, nil
}

// FetchSubscription fetches subscription content from URL and decodes it
// Returns decoded content and error if fetch or decode fails
func FetchSubscription(url string) ([]byte, error) {
	// Create HTTP client with timeout
	client := &http.Client{
		Timeout: 20 * time.Second,
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// Set user agent to avoid blocking
	req.Header.Set("User-Agent", "singbox-launcher/1.0")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch subscription: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("subscription server returned status %d", resp.StatusCode)
	}

	content, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read subscription content: %w", err)
	}

	// Decode base64 if needed
	decoded, err := DecodeSubscriptionContent(content)
	if err != nil {
		return nil, fmt.Errorf("failed to decode subscription content: %w", err)
	}

	return decoded, nil
}

