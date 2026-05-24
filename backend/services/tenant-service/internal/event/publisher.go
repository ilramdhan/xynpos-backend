package event

import (
	"context"

	"github.com/google/uuid"

	appevents "github.com/extendedsynaptic/xynpos/shared/pkg/events"
)

// Publisher defines events published by the tenant service.
type Publisher interface {
	PublishTenantCreated(ctx context.Context, tenantID, ownerID uuid.UUID, name, plan string) error
	PublishUserInvited(ctx context.Context, tenantID uuid.UUID, email, token, tenantName string) error
}

// publisher implements Publisher using NATS JetStream.
type publisher struct {
	client *appevents.Client
	source string
}

// NewPublisher creates a new tenant event publisher.
func NewPublisher(client *appevents.Client) Publisher {
	return &publisher{client: client, source: "/services/tenant"}
}

// PublishTenantCreated publishes an event when a new tenant is provisioned.
func (p *publisher) PublishTenantCreated(ctx context.Context, tenantID, ownerID uuid.UUID, name, plan string) error {
	return p.client.PublishNew(ctx, appevents.EventTenantCreated, p.source, tenantID.String(), map[string]interface{}{
		"tenant_id": tenantID.String(),
		"owner_id":  ownerID.String(),
		"name":      name,
		"plan":      plan,
	})
}

// PublishUserInvited publishes an event to send invitation email via notification-service.
func (p *publisher) PublishUserInvited(ctx context.Context, tenantID uuid.UUID, email, token, tenantName string) error {
	return p.client.PublishNew(ctx, appevents.EventTenantUserInvited, p.source, tenantID.String(), map[string]interface{}{
		"tenant_id":   tenantID.String(),
		"email":       email,
		"token":       token,
		"tenant_name": tenantName,
	})
}
