package config

import (
	"fmt"
	"os"
	"strings"
	"time"
)

type Config struct {
	AppPort         string
	AuthToken       string
	DatabaseURL     string
	DBTimeout       time.Duration
	RequestTimeout  time.Duration
	ShutdownTimeout time.Duration
}

func Load() (Config, error) {
	cfg := Config{
		AppPort:         os.Getenv("APP_PORT"),
		AuthToken:       os.Getenv("AUTH_TOKEN"),
		DatabaseURL:     os.Getenv("DATABASE_URL"),
		DBTimeout:       5 * time.Second,
		RequestTimeout:  10 * time.Second,
		ShutdownTimeout: 10 * time.Second,
	}

	var missing []string

	if cfg.AuthToken == "" {
		missing = append(missing, "AUTH_TOKEN")
	}
	if cfg.DatabaseURL == "" {
		missing = append(missing, "DATABASE_URL")
	}
	if cfg.AppPort == "" {
		missing = append(missing, "APP_PORT")
	}
	if len(missing) > 0 {
		return Config{}, fmt.Errorf("missing required env: %s", strings.Join(missing, ", "))
	}

	return cfg, nil
}
