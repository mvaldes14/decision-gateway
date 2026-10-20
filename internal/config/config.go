package config

import (
	"fmt"
	"os"
	"strconv"
)

type Config struct {
	ListenAddr     string
	DefinitionsDir string
	MaxBodyBytes   int64
	MaxConcurrent  int
}

func Load() (Config, error) {
	config := Config{
		ListenAddr:     envOrDefault("DG_LISTEN_ADDR", ":8080"),
		DefinitionsDir: envOrDefault("DG_DEFINITIONS_DIR", "./decisions"),
		MaxBodyBytes:   1 << 20,
		MaxConcurrent:  64,
	}

	if value := os.Getenv("DG_MAX_BODY_BYTES"); value != "" {
		parsed, err := strconv.ParseInt(value, 10, 64)
		if err != nil || parsed <= 0 {
			return Config{}, fmt.Errorf("DG_MAX_BODY_BYTES must be a positive integer")
		}
		config.MaxBodyBytes = parsed
	}
	if value := os.Getenv("DG_MAX_CONCURRENT"); value != "" {
		parsed, err := strconv.Atoi(value)
		if err != nil || parsed <= 0 {
			return Config{}, fmt.Errorf("DG_MAX_CONCURRENT must be a positive integer")
		}
		config.MaxConcurrent = parsed
	}
	return config, nil
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}
