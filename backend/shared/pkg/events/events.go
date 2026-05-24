package events

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"github.com/nats-io/nats.go/jetstream"
)

// ──────────────────────────────────────────────
// CloudEvents envelope
// ──────────────────────────────────────────────

// Event is a CloudEvents v1.0 compliant event envelope.
type Event struct {
	SpecVersion     string          `json:"specversion"`
	ID              string          `json:"id"`
	Type            string          `json:"type"`    // e.g. "pos.transaction.completed"
	Source          string          `json:"source"`  // e.g. "/services/pos"
	Time            time.Time       `json:"time"`
	TenantID        string          `json:"tenantid"`
	DataContentType string          `json:"datacontenttype"`
	TraceID         string          `json:"traceid,omitempty"` // OpenTelemetry trace propagation
	SpanID          string          `json:"spanid,omitempty"`
	Data            json.RawMessage `json:"data"`
}

// NewEvent creates a new CloudEvent.
func NewEvent(eventType, source, tenantID string, data interface{}) (*Event, error) {
	raw, err := json.Marshal(data)
	if err != nil {
		return nil, fmt.Errorf("events: marshal data for %s: %w", eventType, err)
	}

	return &Event{
		SpecVersion:     "1.0",
		ID:              uuid.NewString(),
		Type:            eventType,
		Source:          source,
		Time:            time.Now().UTC(),
		TenantID:        tenantID,
		DataContentType: "application/json",
		Data:            raw,
	}, nil
}

// Unmarshal decodes the event data into a target struct.
func (e *Event) Unmarshal(target interface{}) error {
	return json.Unmarshal(e.Data, target)
}

// ──────────────────────────────────────────────
// Client
// ──────────────────────────────────────────────

// Client wraps NATS JetStream for publishing and subscribing to events.
type Client struct {
	nc  *nats.Conn
	js  jetstream.JetStream
	cfg Config
}

// Config holds NATS connection configuration.
type Config struct {
	URL        string
	StreamName string // e.g. "XYNPOS_EVENTS"
}

// Handler is the function signature for event subscribers.
type Handler func(ctx context.Context, event *Event) error

// Connect establishes a NATS connection and sets up JetStream.
func Connect(cfg Config) (*Client, error) {
	nc, err := nats.Connect(cfg.URL,
		nats.RetryOnFailedConnect(true),
		nats.MaxReconnects(-1),
		nats.ReconnectWait(2*time.Second),
		nats.DisconnectErrHandler(func(nc *nats.Conn, err error) {
			if err != nil {
				// log disconnect — caller should set up logging
			}
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("events: connect to NATS at %s: %w", cfg.URL, err)
	}

	js, err := jetstream.New(nc)
	if err != nil {
		return nil, fmt.Errorf("events: create jetstream context: %w", err)
	}

	// Ensure the stream exists (idempotent)
	ctx := context.Background()
	_, err = js.CreateOrUpdateStream(ctx, jetstream.StreamConfig{
		Name:     cfg.StreamName,
		Subjects: []string{cfg.StreamName + ".>"},
		MaxAge:   7 * 24 * time.Hour, // Retain messages for 7 days
		Storage:  jetstream.FileStorage,
	})
	if err != nil {
		return nil, fmt.Errorf("events: create stream %s: %w", cfg.StreamName, err)
	}

	return &Client{nc: nc, js: js, cfg: cfg}, nil
}

// Publish publishes a CloudEvent to NATS JetStream.
// The subject is: {StreamName}.{eventType}
func (c *Client) Publish(ctx context.Context, event *Event) error {
	data, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("events: marshal event %s: %w", event.Type, err)
	}

	subject := c.cfg.StreamName + "." + event.Type

	_, err = c.js.Publish(ctx, subject, data)
	if err != nil {
		return fmt.Errorf("events: publish to %s: %w", subject, err)
	}
	return nil
}

// PublishNew is a convenience method that creates and publishes an event in one call.
func (c *Client) PublishNew(ctx context.Context, eventType, source, tenantID string, data interface{}) error {
	evt, err := NewEvent(eventType, source, tenantID, data)
	if err != nil {
		return err
	}
	return c.Publish(ctx, evt)
}

// Subscribe creates a durable push consumer and calls handler for each message.
// consumerName must be unique per subscriber (e.g. "inventory-transaction-completed").
// filterSubject is the NATS subject pattern (e.g. "XYNPOS_EVENTS.pos.transaction.completed").
func (c *Client) Subscribe(
	ctx context.Context,
	consumerName string,
	filterSubject string,
	handler Handler,
) error {
	consumer, err := c.js.CreateOrUpdateConsumer(ctx, c.cfg.StreamName, jetstream.ConsumerConfig{
		Name:          consumerName,
		Durable:       consumerName,
		FilterSubject: c.cfg.StreamName + "." + filterSubject,
		AckPolicy:     jetstream.AckExplicitPolicy,
		MaxDeliver:    5, // Retry up to 5 times
		AckWait:       30 * time.Second,
		DeliverPolicy: jetstream.DeliverAllPolicy,
	})
	if err != nil {
		return fmt.Errorf("events: create consumer %s: %w", consumerName, err)
	}

	_, err = consumer.Consume(func(msg jetstream.Msg) {
		var evt Event
		if err := json.Unmarshal(msg.Data(), &evt); err != nil {
			_ = msg.Nak() // Reject malformed messages
			return
		}

		handlerErr := handler(ctx, &evt)
		if handlerErr != nil {
			_ = msg.NakWithDelay(5 * time.Second) // Retry after 5s
			return
		}
		_ = msg.Ack()
	})

	return err
}

// Close closes the NATS connection.
func (c *Client) Close() {
	c.nc.Drain()
}

// ──────────────────────────────────────────────
// Well-known event type constants
// ──────────────────────────────────────────────

const (
	// POS Events
	EventPOSTransactionCompleted = "pos.transaction.completed"
	EventPOSTransactionVoided    = "pos.transaction.voided"
	EventPOSTransactionRefunded  = "pos.transaction.refunded"
	EventPOSSessionOpened        = "pos.session.opened"
	EventPOSSessionClosed        = "pos.session.closed"

	// Inventory Events
	EventInventoryStockLow    = "inventory.stock.low"
	EventInventoryOutOfStock  = "inventory.out_of_stock"
	EventInventoryTransfer    = "inventory.transfer.created"

	// Payment Events
	EventPaymentReceived = "payment.received"
	EventPaymentFailed   = "payment.failed"

	// Product Events
	EventProductCreated      = "product.created"
	EventProductPriceChanged = "product.price_changed"
	EventProductDeleted      = "product.deleted"

	// Auth Events
	EventAuthUserRegistered   = "auth.user_registered"
	EventAuthLoginFailed      = "auth.login_failed"
	EventAuthPasswordChanged  = "auth.password_changed"

	// User Events
	EventUserInvited     = "user.invited"
	EventUserRoleChanged = "user.role_changed"

	// Subscription Events
	EventSubscriptionPlanChanged    = "subscription.plan_changed"
	EventSubscriptionPaymentFailed  = "subscription.payment_failed"
	EventSubscriptionTrialEnding    = "subscription.trial_ending"

	// Customer Events
	EventCustomerDataErased = "customer.data_erased"
)
