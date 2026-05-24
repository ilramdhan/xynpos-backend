package event

import (
	"context"

	"github.com/google/uuid"

	appevents "github.com/extendedsynaptic/xynpos/shared/pkg/events"
)

// Publisher defines events published by the auth service.
type Publisher interface {
	PublishUserRegistered(ctx context.Context, userID, tenantID uuid.UUID, email, name, otp string) error
	PublishPasswordReset(ctx context.Context, userID uuid.UUID, email, name, otp string) error
	PublishPasswordChanged(ctx context.Context, userID uuid.UUID, email string) error
}

// publisher implements Publisher using NATS JetStream.
type publisher struct {
	client *appevents.Client
	source string
}

// NewPublisher creates a new auth event publisher.
func NewPublisher(client *appevents.Client) Publisher {
	return &publisher{client: client, source: "/services/auth"}
}

// PublishUserRegistered publishes an event to notify notification-service to send welcome email + OTP.
func (p *publisher) PublishUserRegistered(ctx context.Context, userID, tenantID uuid.UUID, email, name, otp string) error {
	return p.client.PublishNew(ctx, appevents.EventAuthUserRegistered, p.source, tenantID.String(), map[string]interface{}{
		"user_id":   userID.String(),
		"tenant_id": tenantID.String(),
		"email":     email,
		"name":      name,
		"otp":       otp,
	})
}

// PublishPasswordReset publishes an event to send password reset OTP.
func (p *publisher) PublishPasswordReset(ctx context.Context, userID uuid.UUID, email, name, otp string) error {
	return p.client.PublishNew(ctx, appevents.EventAuthPasswordChanged, p.source, "", map[string]interface{}{
		"type":    "password_reset",
		"user_id": userID.String(),
		"email":   email,
		"name":    name,
		"otp":     otp,
	})
}

// PublishPasswordChanged publishes an event when a password is changed (for security alerts).
func (p *publisher) PublishPasswordChanged(ctx context.Context, userID uuid.UUID, email string) error {
	return p.client.PublishNew(ctx, appevents.EventAuthPasswordChanged, p.source, "", map[string]interface{}{
		"type":    "password_changed",
		"user_id": userID.String(),
		"email":   email,
	})
}
