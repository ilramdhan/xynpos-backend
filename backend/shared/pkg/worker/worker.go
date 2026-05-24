package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
)

// ──────────────────────────────────────────────
// Task types
// ──────────────────────────────────────────────

// Task wraps an Asynq task with typed payload.
type Task struct {
	Type    string
	Payload interface{}
	Options []asynq.Option
}

// ──────────────────────────────────────────────
// Client (enqueue tasks)
// ──────────────────────────────────────────────

// Client enqueues background tasks.
type Client struct {
	asynq *asynq.Client
}

// NewClient creates a new task enqueue client.
func NewClient(redisAddr string) *Client {
	return &Client{
		asynq: asynq.NewClient(asynq.RedisClientOpt{Addr: redisAddr}),
	}
}

// Enqueue enqueues a task for immediate execution.
func (c *Client) Enqueue(ctx context.Context, taskType string, payload interface{}, opts ...asynq.Option) error {
	raw, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("worker enqueue %s: marshal payload: %w", taskType, err)
	}

	task := asynq.NewTask(taskType, raw, opts...)
	_, err = c.asynq.EnqueueContext(ctx, task)
	if err != nil {
		return fmt.Errorf("worker enqueue %s: %w", taskType, err)
	}
	return nil
}

// EnqueueIn enqueues a task to run after a delay.
func (c *Client) EnqueueIn(ctx context.Context, taskType string, payload interface{}, delay time.Duration, opts ...asynq.Option) error {
	opts = append(opts, asynq.ProcessIn(delay))
	return c.Enqueue(ctx, taskType, payload, opts...)
}

// EnqueueAt enqueues a task to run at a specific time.
func (c *Client) EnqueueAt(ctx context.Context, taskType string, payload interface{}, at time.Time, opts ...asynq.Option) error {
	opts = append(opts, asynq.ProcessAt(at))
	return c.Enqueue(ctx, taskType, payload, opts...)
}

// Close closes the underlying connection.
func (c *Client) Close() error {
	return c.asynq.Close()
}

// ──────────────────────────────────────────────
// Server (process tasks)
// ──────────────────────────────────────────────

// HandlerFunc is the signature for a task handler function.
type HandlerFunc func(ctx context.Context, payload []byte) error

// Server processes background tasks.
// Embed this in your service and register handlers.
//
//	srv := worker.NewServer(cfg.Worker.RedisURL, cfg.Worker.Concurrency)
//	srv.Register("email:send_welcome", sendWelcomeHandler)
//	srv.Start()
//	defer srv.Shutdown()
type Server struct {
	server   *asynq.Server
	mux      *asynq.ServeMux
	handlers map[string]HandlerFunc
}

// NewServer creates a new worker server.
// concurrency: number of concurrent goroutines (e.g. 10)
func NewServer(redisAddr string, concurrency int) *Server {
	srv := asynq.NewServer(
		asynq.RedisClientOpt{Addr: redisAddr},
		asynq.Config{
			Concurrency: concurrency,
			Queues: map[string]int{
				"critical": 6, // payment, auth
				"default":  3, // general tasks
				"low":      1, // email, notifications
			},
			RetryDelayFunc: func(n int, e error, t *asynq.Task) time.Duration {
				// Exponential backoff: 5s, 10s, 20s, 40s, 80s
				return time.Duration(5<<n) * time.Second
			},
		},
	)

	return &Server{
		server:   srv,
		mux:      asynq.NewServeMux(),
		handlers: make(map[string]HandlerFunc),
	}
}

// Register registers a handler for a task type.
func (s *Server) Register(taskType string, handler HandlerFunc) {
	s.handlers[taskType] = handler
	s.mux.HandleFunc(taskType, func(ctx context.Context, t *asynq.Task) error {
		return handler(ctx, t.Payload())
	})
}

// Start starts the worker server (non-blocking).
func (s *Server) Start() error {
	return s.server.Start(s.mux)
}

// Shutdown gracefully shuts down the worker.
func (s *Server) Shutdown() {
	s.server.Shutdown()
}

// ──────────────────────────────────────────────
// Cron Scheduler
// ──────────────────────────────────────────────

// Scheduler wraps Asynq's periodic task scheduler.
//
//	sch := worker.NewScheduler(redisAddr)
//	sch.Add("@every 1m", "task:cleanup", nil)
//	sch.Start()
//	defer sch.Shutdown()
type Scheduler struct {
	scheduler *asynq.Scheduler
	client    *Client
}

// NewScheduler creates a new cron scheduler.
func NewScheduler(redisAddr string) *Scheduler {
	return &Scheduler{
		scheduler: asynq.NewScheduler(
			asynq.RedisClientOpt{Addr: redisAddr},
			&asynq.SchedulerOpts{},
		),
		client: NewClient(redisAddr),
	}
}

// Add registers a periodic task. cronSpec follows standard cron format or @every/@daily etc.
func (s *Scheduler) Add(cronSpec, taskType string, payload interface{}, opts ...asynq.Option) error {
	raw, _ := json.Marshal(payload)
	task := asynq.NewTask(taskType, raw, opts...)
	_, err := s.scheduler.Register(cronSpec, task)
	return err
}

// Start starts the cron scheduler (non-blocking).
func (s *Scheduler) Start() error {
	return s.scheduler.Start()
}

// Shutdown stops the scheduler.
func (s *Scheduler) Shutdown() {
	s.scheduler.Shutdown()
}

// ──────────────────────────────────────────────
// Well-known task type constants
// ──────────────────────────────────────────────

const (
	// Auth / Notification tasks
	TaskSendWelcomeEmail       = "email:send_welcome"
	TaskSendOTPEmail           = "email:send_otp"
	TaskSendPasswordResetEmail = "email:send_password_reset"
	TaskSendInvoiceEmail       = "email:send_invoice"
	TaskSendReceiptEmail       = "email:send_receipt"
	TaskSendStockAlertEmail    = "email:send_stock_alert"

	// FCM Push tasks
	TaskSendPushNotification = "push:send_notification"

	// File processing tasks
	TaskProcessImageUpload = "file:process_image"

	// Report export tasks
	TaskExportSalesReport     = "report:export_sales"
	TaskExportInventoryReport = "report:export_inventory"

	// Subscription / Billing tasks
	TaskProcessBilling         = "billing:process"
	TaskSendBillingReminder    = "billing:send_reminder"
	TaskDowngradeSuspendTenant = "billing:downgrade_suspend"
)
