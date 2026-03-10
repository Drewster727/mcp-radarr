package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

// Config holds all runtime configuration for the MCP server.
type Config struct {
	RadarrURL             string
	APIKey                string
	DefaultQualityProfile string
	RootFolderPath        string
	// AllowMutations enables update and delete tools. Defaults to false (read + add only).
	AllowMutations bool
}

// Load reads configuration from environment variables.
// Required: RADARR_API_KEY, RADARR_ROOT_FOLDER_PATH
func Load() (*Config, error) {
	cfg := &Config{
		RadarrURL:             strings.TrimRight(getEnv("RADARR_URL", "http://localhost:7878"), "/"),
		APIKey:                getEnv("RADARR_API_KEY", ""),
		DefaultQualityProfile: getEnv("RADARR_DEFAULT_QUALITY_PROFILE", "Any"),
		RootFolderPath:        getEnv("RADARR_ROOT_FOLDER_PATH", ""),
	}

	if raw := getEnv("RADARR_ALLOW_MUTATIONS", "false"); raw != "" {
		v, err := strconv.ParseBool(raw)
		if err != nil {
			return nil, fmt.Errorf("invalid RADARR_ALLOW_MUTATIONS value %q: %w", raw, err)
		}
		cfg.AllowMutations = v
	}

	if cfg.APIKey == "" {
		return nil, fmt.Errorf("RADARR_API_KEY is required")
	}
	if cfg.RootFolderPath == "" {
		return nil, fmt.Errorf("RADARR_ROOT_FOLDER_PATH is required")
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
