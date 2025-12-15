package core

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// DecodeSubscriptionContent decodes subscription content from base64 or returns plain text
// Returns decoded content and error if decoding fails
func DecodeSubscriptionContent(content []byte) ([]byte, error) {
	if len(content) == 0 {
		return nil, fmt.Errorf("content is empty")
	}

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

	// Check if decoded content is empty
	if len(decoded) == 0 {
		return nil, fmt.Errorf("decoded content is empty")
	}

	return decoded, nil
}

// FetchSubscription fetches subscription content from URL and decodes it
// Returns decoded content and error if fetch or decode fails
func FetchSubscription(url string) ([]byte, error) {
	startTime := time.Now()
	log.Printf("[DEBUG] FetchSubscription: START at %s, URL: %s", startTime.Format("15:04:05.000"), url)

	// Создаем контекст с таймаутом
	ctx, cancel := context.WithTimeout(context.Background(), NetworkRequestTimeout)
	defer cancel()

	// Используем универсальный HTTP клиент
	client := createHTTPClient(NetworkRequestTimeout)

	requestStartTime := time.Now()
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		log.Printf("[DEBUG] FetchSubscription: Failed to create request (took %v): %v", time.Since(requestStartTime), err)
		return nil, fmt.Errorf("failed to create request: %w", err)
	}
	log.Printf("[DEBUG] FetchSubscription: Created request in %v", time.Since(requestStartTime))

	// Set user agent to avoid blocking
	req.Header.Set("User-Agent", "singbox-launcher/1.0")

	doStartTime := time.Now()
	log.Printf("[DEBUG] FetchSubscription: Sending HTTP request")
	resp, err := client.Do(req)
	doDuration := time.Since(doStartTime)
	if err != nil {
		log.Printf("[DEBUG] FetchSubscription: HTTP request failed (took %v): %v", doDuration, err)
		// Проверяем тип ошибки
		if IsNetworkError(err) {
			return nil, fmt.Errorf("network error: %s", GetNetworkErrorMessage(err))
		}
		return nil, fmt.Errorf("failed to fetch subscription: %w", err)
	}
	defer resp.Body.Close()
	log.Printf("[DEBUG] FetchSubscription: Received HTTP response in %v (status: %d, content-length: %d)",
		doDuration, resp.StatusCode, resp.ContentLength)

	if resp.StatusCode != http.StatusOK {
		log.Printf("[DEBUG] FetchSubscription: Non-OK status code: %d", resp.StatusCode)
		return nil, fmt.Errorf("subscription server returned status %d", resp.StatusCode)
	}

	readStartTime := time.Now()
	log.Printf("[DEBUG] FetchSubscription: Reading response body")
	content, err := io.ReadAll(resp.Body)
	readDuration := time.Since(readStartTime)
	if err != nil {
		log.Printf("[DEBUG] FetchSubscription: Failed to read response body (took %v): %v", readDuration, err)
		return nil, fmt.Errorf("failed to read subscription content: %w", err)
	}
	log.Printf("[DEBUG] FetchSubscription: Read %d bytes in %v", len(content), readDuration)

	// Check if content is empty
	if len(content) == 0 {
		log.Printf("[DEBUG] FetchSubscription: Empty content received")
		return nil, fmt.Errorf("subscription returned empty content")
	}

	// Decode base64 if needed
	decodeStartTime := time.Now()
	log.Printf("[DEBUG] FetchSubscription: Decoding subscription content")
	decoded, err := DecodeSubscriptionContent(content)
	decodeDuration := time.Since(decodeStartTime)
	if err != nil {
		log.Printf("[DEBUG] FetchSubscription: Failed to decode content (took %v): %v", decodeDuration, err)
		return nil, fmt.Errorf("failed to decode subscription content: %w", err)
	}
	log.Printf("[DEBUG] FetchSubscription: Decoded content in %v (original: %d bytes, decoded: %d bytes)",
		decodeDuration, len(content), len(decoded))

	totalDuration := time.Since(startTime)
	log.Printf("[DEBUG] FetchSubscription: END (total duration: %v)", totalDuration)
	return decoded, nil
}

// ParserConfig represents the configuration structure from @ParserConfig block
// Supports versions 1, 2, and 3 (current) with automatic migration
type ParserConfig struct {
	// Version 1 structure (legacy support)
	Version      int `json:"version,omitempty"`
	ParserConfig struct {
		Version   int              `json:"version,omitempty"`
		Proxies   []ProxySource    `json:"proxies"`
		Outbounds []OutboundConfig `json:"outbounds"`
		Parser    struct {
			Reload      string `json:"reload,omitempty"`       // Интервал автоматического обновления
			LastUpdated string `json:"last_updated,omitempty"` // Время последнего обновления (RFC3339, UTC)
		} `json:"parser,omitempty"`
	} `json:"ParserConfig"`
}

// ParserConfigVersion is the current version of ParserConfig format
const ParserConfigVersion = 3

// MigrationFunc is a function that migrates config from version N to version N+1
type MigrationFunc func(*ParserConfig) error

// ConfigMigrator handles automatic migration of ParserConfig between versions
type ConfigMigrator struct {
	migrations map[int]MigrationFunc
}

// NewConfigMigrator creates a new migrator with registered migrations
func NewConfigMigrator() *ConfigMigrator {
	migrator := &ConfigMigrator{
		migrations: make(map[int]MigrationFunc),
	}

	// Register all migrations
	migrator.RegisterMigration(1, migrateV1ToV2)
	migrator.RegisterMigration(2, migrateV2ToV3)
	// Future migrations can be added here:
	// migrator.RegisterMigration(3, migrateV3ToV4)

	return migrator
}

// RegisterMigration registers a migration function for a specific version
func (m *ConfigMigrator) RegisterMigration(fromVersion int, fn MigrationFunc) {
	m.migrations[fromVersion] = fn
}

// Migrate migrates ParserConfig from its current version to the target version
// Applies migrations sequentially: currentVersion -> currentVersion+1 -> ... -> targetVersion
func (m *ConfigMigrator) Migrate(config *ParserConfig, targetVersion int) error {
	if config == nil {
		return fmt.Errorf("config is nil")
	}

	currentVersion := config.ParserConfig.Version

	// Handle legacy version 1 format (version at top level)
	if config.Version > 0 && config.ParserConfig.Version == 0 {
		currentVersion = config.Version
		log.Printf("ConfigMigrator: Detected legacy version 1 format, will migrate to version 2")
	}

	if currentVersion == 0 {
		// No version specified - treat as version 1 and migrate from there
		// This ensures all migrations are applied even for new configs
		currentVersion = 1
		log.Printf("ConfigMigrator: No version specified, treating as version 1 and migrating to version %d", targetVersion)
		// Don't set version yet - migrations will set it
	}

	if currentVersion == targetVersion {
		// Already at target version
		return nil
	}

	if currentVersion > targetVersion {
		return fmt.Errorf("config version %d is newer than supported version %d. Please update the application",
			currentVersion, targetVersion)
	}

	// Apply migrations sequentially
	for version := currentVersion; version < targetVersion; version++ {
		migration, exists := m.migrations[version]
		if !exists {
			return fmt.Errorf("migration from version %d to %d not found", version, version+1)
		}

		log.Printf("ConfigMigrator: Migrating from version %d to version %d", version, version+1)

		if err := migration(config); err != nil {
			return fmt.Errorf("failed to migrate from version %d to %d: %w", version, version+1, err)
		}

		// Update version after successful migration
		config.ParserConfig.Version = version + 1
		log.Printf("ConfigMigrator: Successfully migrated to version %d", version+1)
	}

	return nil
}

// migrateV1ToV2 migrates ParserConfig from version 1 to version 2
// Version 1 had version at top level, version 2 moved it inside ParserConfig
func migrateV1ToV2(config *ParserConfig) error {
	// If version is at top level, move it inside ParserConfig
	if config.Version > 0 && config.ParserConfig.Version == 0 {
		config.ParserConfig.Version = config.Version
		config.Version = 0
		log.Printf("migrateV1ToV2: Moved version from top level to ParserConfig")
	}

	// Note: reload will be set by NormalizeParserConfig if missing

	return nil
}

// migrateV2ToV3 migrates ParserConfig from version 2 to version 3
// Version 3 removes nested "outbounds" object and renames "proxies" to "filters"
func migrateV2ToV3(config *ParserConfig) error {
	for i := range config.ParserConfig.Outbounds {
		outbound := &config.ParserConfig.Outbounds[i]

		// Migrate nested outbounds structure to flat structure
		if outbound.Outbounds.Proxies != nil {
			outbound.Filters = outbound.Outbounds.Proxies
			outbound.Outbounds.Proxies = nil
			log.Printf("migrateV2ToV3: Migrated 'outbounds.proxies' to 'filters' for outbound '%s'", outbound.Tag)
		}

		if len(outbound.Outbounds.AddOutbounds) > 0 {
			// Copy addOutbounds to top level
			outbound.AddOutbounds = outbound.Outbounds.AddOutbounds
			outbound.Outbounds.AddOutbounds = nil
			log.Printf("migrateV2ToV3: Migrated 'outbounds.addOutbounds' to top level for outbound '%s'", outbound.Tag)
		}

		if len(outbound.Outbounds.PreferredDefault) > 0 {
			// Copy preferredDefault to top level
			outbound.PreferredDefault = outbound.Outbounds.PreferredDefault
			outbound.Outbounds.PreferredDefault = nil
			log.Printf("migrateV2ToV3: Migrated 'outbounds.preferredDefault' to top level for outbound '%s'", outbound.Tag)
		}
	}

	return nil
}

// NormalizeParserConfig normalizes ParserConfig structure:
// - Migrates to current version using ConfigMigrator
// - Ensures version is set to ParserConfigVersion
// - Sets default reload to "4h" if not specified
// - Optionally updates last_updated timestamp (if updateLastUpdated is true)
func NormalizeParserConfig(parserConfig *ParserConfig, updateLastUpdated bool) {
	if parserConfig == nil {
		return
	}

	// Create migrator and apply migrations
	migrator := NewConfigMigrator()
	if err := migrator.Migrate(parserConfig, ParserConfigVersion); err != nil {
		log.Printf("NormalizeParserConfig: Migration error: %v", err)
		// Continue anyway - try to set defaults
	}

	// Ensure parser object exists (create if missing)
	// Set default reload to "4h" if not specified
	if parserConfig.ParserConfig.Parser.Reload == "" {
		parserConfig.ParserConfig.Parser.Reload = "4h"
	}

	// Optionally update last_updated timestamp
	if updateLastUpdated {
		parserConfig.ParserConfig.Parser.LastUpdated = time.Now().UTC().Format(time.RFC3339)
	}
}

// ProxySource represents a proxy subscription source
type ProxySource struct {
	Source      string              `json:"source,omitempty"`
	Connections []string            `json:"connections,omitempty"`
	Skip        []map[string]string `json:"skip,omitempty"`
}

// OutboundConfig represents an outbound selector configuration
// Version 3: flat structure without nested "outbounds" object
type OutboundConfig struct {
	Tag              string                 `json:"tag"`
	Type             string                 `json:"type"`
	Options          map[string]interface{} `json:"options,omitempty"`
	Filters          map[string]interface{} `json:"filters,omitempty"`          // Version 3: renamed from outbounds.proxies
	AddOutbounds     []string               `json:"addOutbounds,omitempty"`     // Version 3: moved from outbounds.addOutbounds
	PreferredDefault map[string]interface{} `json:"preferredDefault,omitempty"` // Version 3: moved from outbounds.preferredDefault
	Comment          string                 `json:"comment,omitempty"`

	// Legacy fields for migration (only used during unmarshaling)
	// These fields are ignored during marshaling if empty
	Outbounds struct {
		Proxies          map[string]interface{} `json:"proxies,omitempty"`
		AddOutbounds     []string               `json:"addOutbounds,omitempty"`
		PreferredDefault map[string]interface{} `json:"preferredDefault,omitempty"`
	} `json:"outbounds,omitempty"` // Version 2 and below: nested structure
}

// ExtractParserConfig extracts the @ParserConfig block from config.json
// Returns the parsed ParserConfig structure and error if extraction or parsing fails
func ExtractParserConfig(configPath string) (*ParserConfig, error) {
	data, err := os.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config.json: %w", err)
	}

	// Find the @ParserConfig block using regex
	// Pattern matches: /** @ParserConfig ... */
	pattern := regexp.MustCompile(`/\*\*\s*@ParserConfig\s*\n([\s\S]*?)\*/`)
	matches := pattern.FindSubmatch(data)

	if len(matches) < 2 {
		return nil, fmt.Errorf("@ParserConfig block not found in config.json")
	}

	// Extract the JSON content from the comment block
	jsonContent := strings.TrimSpace(string(matches[1]))

	// Parse the JSON
	var parserConfig ParserConfig
	if err := json.Unmarshal([]byte(jsonContent), &parserConfig); err != nil {
		return nil, fmt.Errorf("failed to parse @ParserConfig JSON: %w", err)
	}

	// Automatically migrate to current version
	migrator := NewConfigMigrator()
	if err := migrator.Migrate(&parserConfig, ParserConfigVersion); err != nil {
		return nil, fmt.Errorf("failed to migrate config: %w", err)
	}

	log.Printf("ExtractParserConfig: Successfully extracted @ParserConfig (version %d) with %d proxy sources and %d outbounds",
		parserConfig.ParserConfig.Version,
		len(parserConfig.ParserConfig.Proxies),
		len(parserConfig.ParserConfig.Outbounds))

	return &parserConfig, nil
}

// UpdateLastUpdatedInConfig updates the last_updated field in the @ParserConfig block
func UpdateLastUpdatedInConfig(configPath string, lastUpdated time.Time) error {
	log.Printf("UpdateLastUpdatedInConfig: Updating last_updated to %s", lastUpdated.Format(time.RFC3339))

	// Read config file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read config file: %w", err)
	}

	// Find the @ParserConfig block using regex
	pattern := regexp.MustCompile(`(/\*\*\s*@ParserConfig\s*\n)([\s\S]*?)(\*/)`)
	matches := pattern.FindSubmatch(data)

	if len(matches) < 4 {
		return fmt.Errorf("@ParserConfig block not found in config.json")
	}

	// Extract the JSON content from the comment block
	jsonContent := strings.TrimSpace(string(matches[2]))

	// Parse the JSON
	var parserConfig ParserConfig
	if err := json.Unmarshal([]byte(jsonContent), &parserConfig); err != nil {
		return fmt.Errorf("failed to parse @ParserConfig JSON: %w", err)
	}

	// Normalize config (ensures version is set, applies migrations, sets default reload to "4h" if missing)
	// Pass false to updateLastUpdated because we'll set it explicitly below
	NormalizeParserConfig(&parserConfig, false)

	// Update last_updated field (always update on each run)
	parserConfig.ParserConfig.Parser.LastUpdated = lastUpdated.Format(time.RFC3339)

	// Serialize back to JSON with indentation
	// Wrap ParserConfig in outer object for version 2 format
	outerJSON := map[string]interface{}{
		"ParserConfig": parserConfig.ParserConfig,
	}
	finalJSON, err := json.MarshalIndent(outerJSON, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal outer @ParserConfig: %w", err)
	}

	newBlock := string(matches[1]) + string(finalJSON) + "\n" + string(matches[3])

	// Replace the block in the file
	newContent := pattern.ReplaceAll(data, []byte(newBlock))

	// Write to file
	if err := os.WriteFile(configPath, newContent, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	log.Printf("UpdateLastUpdatedInConfig: Successfully updated last_updated to %s", parserConfig.ParserConfig.Parser.LastUpdated)
	return nil
}
