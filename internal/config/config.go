package config

import (
	"os"
	"strconv"
)

// Config holds the application configuration
type Config struct {
	ProxyAPIKey string
	APIDomain   string
	DBPath      string
	Port        int
	Debug       bool
}

// Load loads configuration from environment variables
func Load() *Config {
	cfg := &Config{
		ProxyAPIKey: os.Getenv("PROXY_API_KEY"),
		APIDomain:   os.Getenv("PROXY_API_DOMAIN"),
		DBPath:      getEnv("DB_PATH", "data/proxy.db"),
		Port:        getEnvAsInt("PORT", 80),
		Debug:       getEnvAsBool("DEBUG", false),
	}

	return cfg
}

// getEnv gets an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvAsInt gets an environment variable as integer or returns a default value
func getEnvAsInt(key string, defaultValue int) int {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.Atoi(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}

// getEnvAsBool gets an environment variable as boolean or returns a default value
func getEnvAsBool(key string, defaultValue bool) bool {
	valueStr := os.Getenv(key)
	if valueStr == "" {
		return defaultValue
	}
	value, err := strconv.ParseBool(valueStr)
	if err != nil {
		return defaultValue
	}
	return value
}
