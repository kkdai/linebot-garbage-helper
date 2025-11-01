package store

import (
	"context"
	"time"

	"cloud.google.com/go/firestore"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type FirestoreClient struct {
	client *firestore.Client
}

type User struct {
	ID        string     `firestore:"id"`
	Favorites []Favorite `firestore:"favorites"`
	CreatedAt time.Time  `firestore:"createdAt"`
	UpdatedAt time.Time  `firestore:"updatedAt"`
}

type Favorite struct {
	Name    string  `firestore:"name"`
	Lat     float64 `firestore:"lat"`
	Lng     float64 `firestore:"lng"`
	Address string  `firestore:"address"`
}

type Reminder struct {
	ID             string    `firestore:"id"`
	UserID         string    `firestore:"userId"`
	StopName       string    `firestore:"stopName"`
	RouteID        string    `firestore:"routeId"`
	ETA            time.Time `firestore:"eta"`
	AdvanceMinutes int       `firestore:"advanceMinutes"`
	Status         string    `firestore:"status"`
	CreatedAt      time.Time `firestore:"createdAt"`
	UpdatedAt      time.Time `firestore:"updatedAt"`
}

type Route struct {
	ID        string                 `firestore:"id"`
	Data      map[string]interface{} `firestore:"data"`
	UpdatedAt time.Time              `firestore:"updatedAt"`
}

func NewFirestoreClient(ctx context.Context, projectID string) (*FirestoreClient, error) {
	client, err := firestore.NewClient(ctx, projectID)
	if err != nil {
		return nil, err
	}
	return &FirestoreClient{client: client}, nil
}

func (fc *FirestoreClient) Close() error {
	return fc.client.Close()
}

func (fc *FirestoreClient) GetUser(ctx context.Context, userID string) (*User, error) {
	doc, err := fc.client.Collection("users").Doc(userID).Get(ctx)
	if err != nil {
		return nil, err
	}
	
	var user User
	if err := doc.DataTo(&user); err != nil {
		return nil, err
	}
	
	return &user, nil
}

func (fc *FirestoreClient) UpsertUser(ctx context.Context, user *User) error {
	user.UpdatedAt = time.Now()
	if user.CreatedAt.IsZero() {
		user.CreatedAt = time.Now()
	}
	
	_, err := fc.client.Collection("users").Doc(user.ID).Set(ctx, user)
	return err
}

func (fc *FirestoreClient) AddFavorite(ctx context.Context, userID string, favorite Favorite) error {
	userRef := fc.client.Collection("users").Doc(userID)
	
	return fc.client.RunTransaction(ctx, func(ctx context.Context, tx *firestore.Transaction) error {
		doc, err := tx.Get(userRef)
		if err != nil {
			if status.Code(err) == codes.NotFound {
				user := &User{
					ID:        userID,
					Favorites: []Favorite{favorite},
					CreatedAt: time.Now(),
					UpdatedAt: time.Now(),
				}
				return tx.Set(userRef, user)
			}
			return err
		}
		
		var user User
		if err := doc.DataTo(&user); err != nil {
			return err
		}
		
		user.Favorites = append(user.Favorites, favorite)
		user.UpdatedAt = time.Now()
		
		return tx.Set(userRef, user)
	})
}

func (fc *FirestoreClient) CreateReminder(ctx context.Context, reminder *Reminder) error {
	reminder.CreatedAt = time.Now()
	reminder.UpdatedAt = time.Now()
	reminder.Status = "active"
	
	_, _, err := fc.client.Collection("reminders").Add(ctx, reminder)
	return err
}

func (fc *FirestoreClient) GetActiveReminders(ctx context.Context, targetTime time.Time) ([]*Reminder, error) {
	// Use single field query to avoid index requirement
	query := fc.client.Collection("reminders").
		Where("status", "==", "active")
	
	docs, err := query.Documents(ctx).GetAll()
	if err != nil {
		return nil, err
	}
	
	var reminders []*Reminder
	for _, doc := range docs {
		var reminder Reminder
		if err := doc.DataTo(&reminder); err != nil {
			continue
		}
		reminder.ID = doc.Ref.ID
		
		// Filter by ETA: should be after targetTime (future reminders)
		// We want reminders where ETA > now and notification time <= now
		if reminder.ETA.After(targetTime) {
			reminders = append(reminders, &reminder)
		}
	}
	
	return reminders, nil
}

func (fc *FirestoreClient) UpdateReminderStatus(ctx context.Context, reminderID, status string) error {
	_, err := fc.client.Collection("reminders").Doc(reminderID).Update(ctx, []firestore.Update{
		{Path: "status", Value: status},
		{Path: "updatedAt", Value: time.Now()},
	})
	return err
}

func (fc *FirestoreClient) StoreRouteData(ctx context.Context, routeID string, data map[string]interface{}) error {
	route := &Route{
		ID:        routeID,
		Data:      data,
		UpdatedAt: time.Now(),
	}
	
	_, err := fc.client.Collection("routes").Doc(routeID).Set(ctx, route)
	return err
}

func (fc *FirestoreClient) GetRouteData(ctx context.Context, routeID string) (*Route, error) {
	doc, err := fc.client.Collection("routes").Doc(routeID).Get(ctx)
	if err != nil {
		return nil, err
	}
	
	var route Route
	if err := doc.DataTo(&route); err != nil {
		return nil, err
	}
	
	return &route, nil
}

func (fc *FirestoreClient) GetAllRoutes(ctx context.Context) ([]*Route, error) {
	docs, err := fc.client.Collection("routes").Documents(ctx).GetAll()
	if err != nil {
		return nil, err
	}
	
	var routes []*Route
	for _, doc := range docs {
		var route Route
		if err := doc.DataTo(&route); err != nil {
			continue
		}
		route.ID = doc.Ref.ID
		routes = append(routes, &route)
	}
	
	return routes, nil
}