package config

import (
	"errors"
	"os"
	"strings"
)

const (
	defaultDriverName   = "hcloud.espressolab.objectstorage.k8s.io"
	defaultCOSIEndpoint = "unix:///var/lib/cosi/cosi.sock"
)

// Config holds all runtime configuration for the driver.
type Config struct {
	DriverName   string // X_COSI_DRIVER_NAME
	COSIEndpoint string // COSI_ENDPOINT
	HCloudToken  string // HCLOUD_TOKEN (required)
}

// Load reads configuration from environment variables.
// Returns an error if any required variable is missing.
func Load() (Config, error) {
	cfg := Config{
		DriverName:   env("X_COSI_DRIVER_NAME", defaultDriverName),
		COSIEndpoint: env("COSI_ENDPOINT", defaultCOSIEndpoint),
		HCloudToken:  strings.TrimSpace(os.Getenv("HCLOUD_TOKEN")),
	}

	if cfg.HCloudToken == "" {
		return cfg, errors.New("HCLOUD_TOKEN is required")
	}

	return cfg, nil
}

func env(key, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return fallback
}
