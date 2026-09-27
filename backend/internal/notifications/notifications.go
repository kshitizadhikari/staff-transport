package notifications

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/hibiken/asynq"
	"gorm.io/gorm"
)

// Notification statuses.
const (
	StatusPending = "pending"
	StatusSent    = "sent"
	StatusFailed  = "failed"
)

// Notification types emitted by the domain.
const (
	TypeTripAssigned  = "trip.assigned"
	TypeTripCancelled = "trip.cancelled"
)

// TypeSendNotification is the Asynq task type for push delivery.
const TypeSendNotification = "notification:send"

var ErrNotFound = errors.New("notification not found")

// Notification is a durable record of a user-facing notification.
type Notification struct {
	ID        string
	UserID    string
	Type      string
	Payload   map[string]any
	Status    string
	SentAt    *time.Time
	CreatedAt time.Time
}

// PushToken is a device push token for a user.
type PushToken struct {
	ID       string
	UserID   string
	Token    string
	Platform string
}

// Repository persists notifications and push tokens.
type Repository interface {
	CreateNotification(ctx context.Context, userID, typ string, payload map[string]any) (*Notification, error)
	GetNotification(ctx context.Context, id string) (*Notification, error)
	MarkSent(ctx context.Context, id string, sentAt time.Time) error
	MarkFailed(ctx context.Context, id string) error
	ListNotifications(ctx context.Context, userID string, offset, limit int) ([]Notification, int64, error)
	UpsertToken(ctx context.Context, userID, token, platform string) error
	TokensForUser(ctx context.Context, userID string) ([]PushToken, error)
}

type notificationRow struct {
	ID        string          `gorm:"column:id"`
	UserID    string          `gorm:"column:user_id"`
	Type      string          `gorm:"column:type"`
	Payload   json.RawMessage `gorm:"column:payload"`
	Status    string          `gorm:"column:status"`
	SentAt    *time.Time      `gorm:"column:sent_at"`
	CreatedAt time.Time       `gorm:"column:created_at"`
}

func (r *notificationRow) toNotification() (*Notification, error) {
	payload := map[string]any{}
	if len(r.Payload) > 0 {
		if err := json.Unmarshal(r.Payload, &payload); err != nil {
			return nil, err
		}
	}
	return &Notification{
		ID:        r.ID,
		UserID:    r.UserID,
		Type:      r.Type,
		Payload:   payload,
		Status:    r.Status,
		SentAt:    r.SentAt,
		CreatedAt: r.CreatedAt,
	}, nil
}

type gormRepository struct {
	db *gorm.DB
}

// NewRepository returns a GORM-backed notification repository.
func NewRepository(gdb *gorm.DB) Repository {
	return &gormRepository{db: gdb}
}

func (r *gormRepository) CreateNotification(ctx context.Context, userID, typ string, payload map[string]any) (*Notification, error) {
	id := uuid.NewString()
	raw := "{}"
	if len(payload) > 0 {
		encoded, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		raw = string(encoded)
	}
	err := r.db.WithContext(ctx).Table("notifications").Create(map[string]any{
		"id":      id,
		"user_id": userID,
		"type":    typ,
		"payload": gorm.Expr("?::jsonb", raw),
		"status":  StatusPending,
	}).Error
	if err != nil {
		return nil, err
	}
	return r.GetNotification(ctx, id)
}

func (r *gormRepository) GetNotification(ctx context.Context, id string) (*Notification, error) {
	var row notificationRow
	err := r.db.WithContext(ctx).
		Table("notifications").
		Select("id, user_id, type, payload, status, sent_at, created_at").
		Where("id = ?", id).
		Take(&row).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return row.toNotification()
}

func (r *gormRepository) MarkSent(ctx context.Context, id string, sentAt time.Time) error {
	return r.db.WithContext(ctx).Table("notifications").
		Where("id = ?", id).
		Updates(map[string]any{"status": StatusSent, "sent_at": sentAt.UTC()}).Error
}

func (r *gormRepository) MarkFailed(ctx context.Context, id string) error {
	return r.db.WithContext(ctx).Table("notifications").
		Where("id = ?", id).
		Update("status", StatusFailed).Error
}

func (r *gormRepository) ListNotifications(ctx context.Context, userID string, offset, limit int) ([]Notification, int64, error) {
	base := func() *gorm.DB {
		return r.db.WithContext(ctx).Table("notifications").Where("user_id = ?", userID)
	}

	var total int64
	if err := base().Count(&total).Error; err != nil {
		return nil, 0, err
	}

	var rows []notificationRow
	if err := base().
		Select("id, user_id, type, payload, status, sent_at, created_at").
		Order("created_at DESC").
		Limit(limit).
		Offset(offset).
		Scan(&rows).Error; err != nil {
		return nil, 0, err
	}

	out := make([]Notification, 0, len(rows))
	for i := range rows {
		item, err := rows[i].toNotification()
		if err != nil {
			return nil, 0, err
		}
		out = append(out, *item)
	}
	return out, total, nil
}

func (r *gormRepository) UpsertToken(ctx context.Context, userID, token, platform string) error {
	return r.db.WithContext(ctx).Exec(`
		INSERT INTO push_tokens (id, user_id, token, platform, created_at, updated_at)
		VALUES (?, ?, ?, ?, now(), now())
		ON CONFLICT (token) DO UPDATE
			SET user_id = EXCLUDED.user_id,
			    platform = EXCLUDED.platform,
			    updated_at = now()`,
		uuid.NewString(), userID, token, platform,
	).Error
}

func (r *gormRepository) TokensForUser(ctx context.Context, userID string) ([]PushToken, error) {
	var rows []PushToken
	err := r.db.WithContext(ctx).
		Table("push_tokens").
		Select("id, user_id, token, platform").
		Where("user_id = ?", userID).
		Scan(&rows).Error
	if err != nil {
		return nil, err
	}
	return rows, nil
}

// Enqueuer enqueues background tasks (implemented by *asynq.Client).
type Enqueuer interface {
	Enqueue(task *asynq.Task, opts ...asynq.Option) (*asynq.TaskInfo, error)
}

// Service creates notifications and delivers them through a Sender.
type Service struct {
	repo   Repository
	sender Sender
	queue  Enqueuer
}

// NewService returns a notification service. queue may be nil in the worker.
func NewService(repo Repository, sender Sender, queue Enqueuer) *Service {
	return &Service{repo: repo, sender: sender, queue: queue}
}

// NotifyUsers records a notification for each user and enqueues delivery.
// Duplicate user IDs are collapsed. Enqueue failures are logged, not returned,
// so notification problems cannot fail the triggering business operation.
func (s *Service) NotifyUsers(ctx context.Context, userIDs []string, typ string, payload map[string]any) error {
	seen := make(map[string]struct{}, len(userIDs))
	for _, userID := range userIDs {
		if userID == "" {
			continue
		}
		if _, ok := seen[userID]; ok {
			continue
		}
		seen[userID] = struct{}{}

		notification, err := s.repo.CreateNotification(ctx, userID, typ, payload)
		if err != nil {
			return err
		}
		if s.queue == nil {
			continue
		}
		task, err := NewSendTask(notification.ID)
		if err != nil {
			return err
		}
		if _, err := s.queue.Enqueue(task); err != nil {
			slog.Error("enqueue notification failed", "notification_id", notification.ID, "error", err)
		}
	}
	return nil
}

// RegisterToken stores or updates a device push token for a user.
func (s *Service) RegisterToken(ctx context.Context, userID, token, platform string) error {
	if platform == "" {
		platform = "unknown"
	}
	return s.repo.UpsertToken(ctx, userID, token, platform)
}

// List returns a page of a user's notifications.
func (s *Service) List(ctx context.Context, userID string, offset, limit int) ([]Notification, int64, error) {
	return s.repo.ListNotifications(ctx, userID, offset, limit)
}

// HandleSend delivers a queued notification. It is safe to retry: an
// already-sent notification is skipped.
func (s *Service) HandleSend(ctx context.Context, task *asynq.Task) error {
	var payload sendPayload
	if err := json.Unmarshal(task.Payload(), &payload); err != nil {
		return err
	}
	if payload.NotificationID == "" {
		return nil
	}

	notification, err := s.repo.GetNotification(ctx, payload.NotificationID)
	if errors.Is(err, ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if notification.Status == StatusSent {
		return nil
	}

	tokens, err := s.repo.TokensForUser(ctx, notification.UserID)
	if err != nil {
		return err
	}
	if len(tokens) == 0 || s.sender == nil {
		return s.repo.MarkFailed(ctx, notification.ID)
	}

	values := make([]string, 0, len(tokens))
	for _, token := range tokens {
		values = append(values, token.Token)
	}

	if err := s.sender.Send(ctx, values, messageFor(notification)); err != nil {
		// Leave the record not-sent and return the error so Asynq retries.
		_ = s.repo.MarkFailed(ctx, notification.ID)
		return err
	}
	return s.repo.MarkSent(ctx, notification.ID, time.Now().UTC())
}

type sendPayload struct {
	NotificationID string `json:"notification_id"`
}

// NewSendTask builds the Asynq task that delivers a notification.
func NewSendTask(notificationID string) (*asynq.Task, error) {
	payload, err := json.Marshal(sendPayload{NotificationID: notificationID})
	if err != nil {
		return nil, err
	}
	return asynq.NewTask(TypeSendNotification, payload), nil
}

func messageFor(notification *Notification) Message {
	switch notification.Type {
	case TypeTripAssigned:
		return Message{
			Title: "New trip assigned",
			Body:  "You have a new trip assignment.",
			Data:  map[string]any{"type": notification.Type, "notification_id": notification.ID},
		}
	case TypeTripCancelled:
		return Message{
			Title: "Trip cancelled",
			Body:  "A trip you are part of has been cancelled.",
			Data:  map[string]any{"type": notification.Type, "notification_id": notification.ID},
		}
	default:
		return Message{
			Title: "Staff Transport",
			Body:  "You have a new notification.",
			Data:  map[string]any{"type": notification.Type, "notification_id": notification.ID},
		}
	}
}
