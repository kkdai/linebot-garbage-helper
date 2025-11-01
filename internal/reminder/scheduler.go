package reminder

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/line/line-bot-sdk-go/v8/linebot/messaging_api"

	"linebot-garbage-helper/internal/store"
)

type Scheduler struct {
	store        *store.FirestoreClient
	messagingAPI *messaging_api.MessagingApiAPI
}

type ReminderService struct {
	scheduler *Scheduler
}

func NewScheduler(store *store.FirestoreClient, messagingAPI *messaging_api.MessagingApiAPI) *Scheduler {
	return &Scheduler{
		store:        store,
		messagingAPI: messagingAPI,
	}
}

func NewReminderService(scheduler *Scheduler) *ReminderService {
	return &ReminderService{
		scheduler: scheduler,
	}
}

func (rs *ReminderService) ProcessReminders(ctx context.Context) error {
	return rs.scheduler.ProcessReminders(ctx)
}

func (s *Scheduler) ProcessReminders(ctx context.Context) error {
	now := time.Now()
	
	reminders, err := s.store.GetActiveReminders(ctx, now)
	if err != nil {
		return fmt.Errorf("failed to get active reminders: %w", err)
	}

	log.Printf("Found %d active reminders to process at %s", len(reminders), now.Format("2006-01-02 15:04:05"))

	for _, reminder := range reminders {
		notificationTime := reminder.ETA.Add(-time.Duration(reminder.AdvanceMinutes) * time.Minute)
		log.Printf("Processing reminder %s: ETA=%s, NotificationTime=%s, AdvanceMinutes=%d", 
			reminder.ID, reminder.ETA.Format("2006-01-02 15:04:05"), 
			notificationTime.Format("2006-01-02 15:04:05"), reminder.AdvanceMinutes)
		
		if err := s.processReminder(ctx, reminder); err != nil {
			log.Printf("Error processing reminder %s: %v", reminder.ID, err)
			continue
		}
	}

	return nil
}

func (s *Scheduler) processReminder(ctx context.Context, reminder *store.Reminder) error {
	now := time.Now()
	
	notificationTime := reminder.ETA.Add(-time.Duration(reminder.AdvanceMinutes) * time.Minute)
	
	log.Printf("Reminder %s evaluation: now=%s, notificationTime=%s, ETA=%s", 
		reminder.ID, now.Format("15:04:05"), notificationTime.Format("15:04:05"), reminder.ETA.Format("15:04:05"))
	
	if now.Before(notificationTime) {
		log.Printf("Reminder %s: Too early to send notification (current time before notification time)", reminder.ID)
		return nil
	}

	if now.After(reminder.ETA) {
		log.Printf("Reminder %s: ETA has passed, marking as expired", reminder.ID)
		err := s.store.UpdateReminderStatus(ctx, reminder.ID, "expired")
		if err != nil {
			log.Printf("Failed to update expired reminder status: %v", err)
		}
		return nil
	}

	log.Printf("Reminder %s: Sending notification to user %s for stop %s", reminder.ID, reminder.UserID, reminder.StopName)
	err := s.sendReminderNotification(ctx, reminder)
	if err != nil {
		return fmt.Errorf("failed to send reminder notification: %w", err)
	}

	err = s.store.UpdateReminderStatus(ctx, reminder.ID, "sent")
	if err != nil {
		return fmt.Errorf("failed to update reminder status: %w", err)
	}

	log.Printf("Successfully sent reminder for user %s, stop %s", reminder.UserID, reminder.StopName)
	return nil
}

func (s *Scheduler) sendReminderNotification(ctx context.Context, reminder *store.Reminder) error {
	timeUntilArrival := time.Until(reminder.ETA)
	minutes := int(timeUntilArrival.Minutes())
	
	var message string
	if minutes <= 0 {
		message = fmt.Sprintf("🗑️ 垃圾車提醒\n\n垃圾車即將抵達 %s！\n請準備好垃圾袋出門。", reminder.StopName)
	} else {
		message = fmt.Sprintf("🗑️ 垃圾車提醒\n\n垃圾車將在 %d 分鐘後抵達 %s\n請準備好垃圾袋。", minutes, reminder.StopName)
	}

	textMessage := messaging_api.TextMessage{
		Text: message,
	}

	req := &messaging_api.PushMessageRequest{
		To:       reminder.UserID,
		Messages: []messaging_api.MessageInterface{&textMessage},
	}

	_, err := s.messagingAPI.PushMessage(req, "")
	return err
}

func (s *Scheduler) CleanupExpiredReminders(ctx context.Context) error {
	cutoffTime := time.Now().Add(-24 * time.Hour)
	
	reminders, err := s.store.GetActiveReminders(ctx, cutoffTime)
	if err != nil {
		return fmt.Errorf("failed to get expired reminders: %w", err)
	}

	for _, reminder := range reminders {
		if reminder.ETA.Before(cutoffTime) {
			err := s.store.UpdateReminderStatus(ctx, reminder.ID, "expired")
			if err != nil {
				log.Printf("Failed to cleanup expired reminder %s: %v", reminder.ID, err)
			}
		}
	}

	return nil
}

func (s *Scheduler) StartScheduler(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()

	cleanupTicker := time.NewTicker(1 * time.Hour)
	defer cleanupTicker.Stop()

	log.Println("Reminder scheduler started")

	for {
		select {
		case <-ctx.Done():
			log.Println("Reminder scheduler stopped")
			return
		case <-ticker.C:
			if err := s.ProcessReminders(ctx); err != nil {
				log.Printf("Error processing reminders: %v", err)
			}
		case <-cleanupTicker.C:
			if err := s.CleanupExpiredReminders(ctx); err != nil {
				log.Printf("Error cleaning up expired reminders: %v", err)
			}
		}
	}
}

func (s *Scheduler) GetUserReminders(ctx context.Context, userID string) ([]*store.Reminder, error) {
	return s.store.GetActiveReminders(ctx, time.Now().Add(24*time.Hour))
}

func (s *Scheduler) CancelReminder(ctx context.Context, reminderID string) error {
	return s.store.UpdateReminderStatus(ctx, reminderID, "cancelled")
}