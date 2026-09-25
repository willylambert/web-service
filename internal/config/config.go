package config

import (
	"os"
	"strconv"
)

// Config holds runtime configuration for the web service.
type Config struct {
	Addr         string
	SNCFAPIToken string
}

// Load reads configuration from environment variables with sensible defaults.
func Load() Config {
	addr := getenv("ADDR", ":8080")
	if port := os.Getenv("PORT"); port != "" {
		if _, err := strconv.Atoi(port); err == nil {
			addr = ":" + port
		}
	}
	return Config{
		Addr:         addr,
		SNCFAPIToken: getenv("SNCF_API_TOKEN", ""),
	}
}

func getenv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
