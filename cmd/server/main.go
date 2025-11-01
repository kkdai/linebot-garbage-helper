package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/gorilla/mux"

	"linebot-garbage-helper/internal/config"
	"linebot-garbage-helper/internal/garbage"
	"linebot-garbage-helper/internal/gemini"
	"linebot-garbage-helper/internal/geo"
	"linebot-garbage-helper/internal/line"
	"linebot-garbage-helper/internal/reminder"
	"linebot-garbage-helper/internal/store"
)

func main() {
	log.Println("Starting garbage LINE bot server...")

	cfg := config.Load()

	if err := validateConfig(cfg); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	firestoreClient, err := store.NewFirestoreClient(ctx, cfg.GCPProjectID)
	if err != nil {
		log.Fatalf("Failed to create Firestore client: %v", err)
	}
	defer firestoreClient.Close()

	geoClient, err := geo.NewGeocodeClient(cfg.GoogleMapsAPIKey)
	if err != nil {
		log.Fatalf("Failed to create geocoding client: %v", err)
	}

	garbageAdapter := garbage.NewGarbageAdapter()

	geminiClient, err := gemini.NewGeminiClient(ctx, cfg.GeminiAPIKey, cfg.GeminiModel)
	if err != nil {
		log.Fatalf("Failed to create Gemini client: %v", err)
	}
	defer geminiClient.Close()

	lineHandler, err := line.NewHandler(
		cfg.LineChannelAccessToken,
		cfg.LineChannelSecret,
		firestoreClient,
		geoClient,
		garbageAdapter,
		geminiClient,
	)
	if err != nil {
		log.Fatalf("Failed to create LINE handler: %v", err)
	}

	reminderScheduler := reminder.NewScheduler(firestoreClient, lineHandler.GetMessagingAPI())
	reminderService := reminder.NewReminderService(reminderScheduler)

	go reminderScheduler.StartScheduler(ctx)

	server := setupServer(cfg, lineHandler, reminderService)

	go func() {
		log.Printf("Server starting on port %s", cfg.Port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Server failed to start: %v", err)
		}
	}()

	waitForShutdown(ctx, server)
}

func setupServer(cfg *config.Config, lineHandler *line.Handler, reminderService *reminder.ReminderService) *http.Server {
	r := mux.NewRouter()

	r.HandleFunc("/line/callback", lineHandler.HandleWebhook).Methods("POST")

	r.HandleFunc("/tasks/dispatch-reminders", func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token != "Bearer "+cfg.InternalTaskToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		ctx := r.Context()
		if err := reminderService.ProcessReminders(ctx); err != nil {
			log.Printf("Error processing reminders: %v", err)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("POST")

	r.HandleFunc("/internal/refresh-routes", func(w http.ResponseWriter, r *http.Request) {
		token := r.Header.Get("Authorization")
		if token != "Bearer "+cfg.InternalTaskToken {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}

		w.WriteHeader(http.StatusOK)
		w.Write([]byte("Routes refresh triggered"))
	}).Methods("POST")

	r.HandleFunc("/healthz", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("OK"))
	}).Methods("GET")

	r.HandleFunc("/internal/token", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf(`{"token":"%s"}`, cfg.InternalTaskToken)))
	}).Methods("GET")

	return &http.Server{
		Addr:         ":" + cfg.Port,
		Handler:      r,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

func validateConfig(cfg *config.Config) error {
	required := map[string]string{
		"LINE_CHANNEL_SECRET":        cfg.LineChannelSecret,
		"LINE_CHANNEL_ACCESS_TOKEN":  cfg.LineChannelAccessToken,
		"GOOGLE_MAPS_API_KEY":        cfg.GoogleMapsAPIKey,
		"GEMINI_API_KEY":             cfg.GeminiAPIKey,
		"GCP_PROJECT_ID":             cfg.GCPProjectID,
	}

	var missing []string
	for key, value := range required {
		if strings.TrimSpace(value) == "" {
			missing = append(missing, key)
		}
	}

	// Check INTERNAL_TASK_TOKEN separately since it can be auto-generated
	if strings.TrimSpace(cfg.InternalTaskToken) == "" {
		log.Println("Warning: INTERNAL_TASK_TOKEN is empty, which should not happen")
	}

	if len(missing) > 0 {
		log.Fatalf("Missing required environment variables: %v", missing)
	}

	return nil
}

func waitForShutdown(ctx context.Context, server *http.Server) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	<-quit
	log.Println("Shutting down server...")

	shutdownCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("Server forced to shutdown: %v", err)
	}

	log.Println("Server exited")
}