package config

import (
	"os"
	"strconv"
)

type Config struct {
	Port                     string
	LineChannelSecret        string
	LineChannelAccessToken   string
	GoogleMapsAPIKey         string
	GeminiAPIKey             string
	GeminiModel              string
	GCPProjectID             string
	InternalTaskToken        string
}

func Load() *Config {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	return &Config{
		Port:                   port,
		LineChannelSecret:      os.Getenv("LINE_CHANNEL_SECRET"),
		LineChannelAccessToken: os.Getenv("LINE_CHANNEL_ACCESS_TOKEN"),
		GoogleMapsAPIKey:       os.Getenv("GOOGLE_MAPS_API_KEY"),
		GeminiAPIKey:           os.Getenv("GEMINI_API_KEY"),
		GeminiModel:            getEnvOrDefault("GEMINI_MODEL", "gemini-1.5-pro"),
		GCPProjectID:           os.Getenv("GCP_PROJECT_ID"),
		InternalTaskToken:      os.Getenv("INTERNAL_TASK_TOKEN"),
	}
}

func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvAsIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}