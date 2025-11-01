package config

import (
	"crypto/rand"
	"encoding/hex"
	"log"
	"os"
	"strconv"

	"linebot-garbage-helper/internal/security"
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

	// Generate internal task token if not provided
	internalTaskToken := os.Getenv("INTERNAL_TASK_TOKEN")
	if internalTaskToken == "" {
		token, err := security.GenerateInternalTaskToken()
		if err != nil {
			log.Printf("Warning: Failed to generate internal task token: %v", err)
			log.Printf("Using fallback token. This is less secure.")
			internalTaskToken = "fallback-token-" + getRandomString(16)
		} else {
			internalTaskToken = token
			log.Printf("Generated internal task token: %s", internalTaskToken)
			log.Printf("Please save this token for Cloud Scheduler configuration")
		}
	}

	return &Config{
		Port:                   port,
		LineChannelSecret:      os.Getenv("LINE_CHANNEL_SECRET"),
		LineChannelAccessToken: os.Getenv("LINE_CHANNEL_ACCESS_TOKEN"),
		GoogleMapsAPIKey:       os.Getenv("GOOGLE_MAPS_API_KEY"),
		GeminiAPIKey:           os.Getenv("GEMINI_API_KEY"),
		GeminiModel:            getEnvOrDefault("GEMINI_MODEL", "gemini-1.5-pro"),
		GCPProjectID:           os.Getenv("GCP_PROJECT_ID"),
		InternalTaskToken:      internalTaskToken,
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

// getRandomString generates a random hex string as fallback
func getRandomString(length int) string {
	bytes := make([]byte, length/2)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}