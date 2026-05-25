//go:build !cilint

package main

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gofiber/contrib/swagger"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"
	"github.com/gofiber/fiber/v3/middleware/helmet"
	"github.com/gofiber/fiber/v3/middleware/requestid"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	_ "github.com/extendedsynaptic/xynpos/auth-service/docs" // Swagger docs

	"github.com/extendedsynaptic/xynpos/auth-service/internal/delivery/http/handler"
	grpcdelivery "github.com/extendedsynaptic/xynpos/auth-service/internal/delivery/grpc"
	"github.com/extendedsynaptic/xynpos/auth-service/internal/domain"
	"github.com/extendedsynaptic/xynpos/auth-service/internal/event"
	repopg "github.com/extendedsynaptic/xynpos/auth-service/internal/repository/postgres"
	"github.com/extendedsynaptic/xynpos/auth-service/internal/usecase"
	"github.com/extendedsynaptic/xynpos/shared/pkg/config"
	"github.com/extendedsynaptic/xynpos/shared/pkg/database"
	appevents "github.com/extendedsynaptic/xynpos/shared/pkg/events"
	"github.com/extendedsynaptic/xynpos/shared/pkg/health"
	appjwt "github.com/extendedsynaptic/xynpos/shared/pkg/jwt"
	"github.com/extendedsynaptic/xynpos/shared/pkg/logger"
	"github.com/extendedsynaptic/xynpos/shared/pkg/metrics"
	"github.com/extendedsynaptic/xynpos/shared/pkg/middleware"
	appredis "github.com/extendedsynaptic/xynpos/shared/pkg/redis"
	"github.com/extendedsynaptic/xynpos/shared/pkg/tracer"
	authpb "github.com/extendedsynaptic/xynpos/shared/proto/auth"
	"github.com/google/uuid"
)

// @title           XynPOS Auth Service API
// @version         1.0
// @description     Authentication and Authorization service for XynPOS
// @host            localhost:8001
// @BasePath        /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter: Bearer <token>

var (
	Version   = "dev"
	BuildTime = "unknown"
)

func main() {
	ctx := context.Background()

	// ── Load config ────────────────────────────────────────────
	cfg := config.MustLoad("auth-service")

	// ── Logger ─────────────────────────────────────────────────
	log := logger.New(cfg.App.LogLevel, cfg.App.Env)
	defer log.Sync() //nolint:errcheck
	zap.ReplaceGlobals(log)

	log.Info("Starting auth-service",
		zap.String("version", Version),
		zap.String("build_time", BuildTime),
		zap.String("env", cfg.App.Env),
	)

	// ── Tracer ─────────────────────────────────────────────────
	tracerShutdown, err := tracer.Init(ctx, tracer.Config{
		ServiceName: "auth-service",
		Environment: cfg.App.Env,
		JaegerURL:   cfg.Tracer.JaegerURL,
		Enabled:     cfg.Tracer.Enabled,
	})
	if err != nil {
		log.Fatal("failed to init tracer", zap.Error(err))
	}
	defer tracerShutdown(ctx)

	// ── Database ───────────────────────────────────────────────
	db, err := database.New(database.Config{
		URL:          cfg.Database.URL,
		MaxOpenConns: cfg.Database.MaxOpenConns,
		MinOpenConns: cfg.Database.MinOpenConns,
		MaxIdleTime:  cfg.Database.MaxIdleTime,
	})
	if err != nil {
		log.Fatal("failed to connect to database", zap.Error(err))
	}
	defer database.Close(db) //nolint:errcheck

	// ── Redis ──────────────────────────────────────────────────
	rdb, err := appredis.New(appredis.Config{
		URL:      cfg.Redis.URL,
		Password: cfg.Redis.Password,
		DB:       cfg.Redis.DB,
	})
	if err != nil {
		log.Fatal("failed to connect to redis", zap.Error(err))
	}
	defer rdb.Close() //nolint:errcheck

	// ── NATS ───────────────────────────────────────────────────
	natsClient, err := appevents.Connect(appevents.Config{
		URL:        cfg.NATS.URL,
		StreamName: "XYNPOS_EVENTS",
	})
	if err != nil {
		log.Fatal("failed to connect to NATS", zap.Error(err))
	}
	defer natsClient.Close()

	// ── JWT Manager ────────────────────────────────────────────
	jwtMgr := appjwt.New(appjwt.Config{
		AccessSecret:  cfg.JWT.AccessSecret,
		RefreshSecret: cfg.JWT.RefreshSecret,
		AccessExpiry:  cfg.JWT.AccessExpiry,
		RefreshExpiry: cfg.JWT.RefreshExpiry,
		Issuer:        cfg.JWT.Issuer,
	})

	// ── Metrics ────────────────────────────────────────────────
	svcMetrics := metrics.New("auth_service")

	// ── Wire dependencies ──────────────────────────────────────
	userRepo := repopg.NewUserRepository(db)
	tokenRepo := repopg.NewRefreshTokenRepository(db)
	otpRepo := repopg.NewOTPRepository(db)
	evtPublisher := event.NewPublisher(natsClient)

	authUC := usecase.New(
		userRepo, tokenRepo, otpRepo, jwtMgr,
		evtPublisher, &stubTenantClient{},
		usecase.AuthUsecaseConfig{
			OTPExpiryMinutes:   10,
			MaxOTPPerHour:      5,
			RefreshTokenExpiry: cfg.JWT.RefreshExpiry,
		},
	)

	authHandler := handler.NewAuthHandler(authUC)

	// ── Fiber App ──────────────────────────────────────────────
	port, _ := strconv.Atoi(cfg.App.Port)
	if port == 0 {
		port = 8001
	}

	app := fiber.New(fiber.Config{
		AppName:      "auth-service v" + Version,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
		ErrorHandler: func(c fiber.Ctx, err error) error {
			return c.Status(500).JSON(map[string]string{
				"code":    "INTERNAL_ERROR",
				"message": "An unexpected error occurred",
			})
		},
	})

	// ── Global Middleware ──────────────────────────────────────
	app.Use(middleware.RecoverPanic())
	app.Use(requestid.New())
	app.Use(logger.FiberMiddleware(log))
	app.Use(tracer.FiberMiddleware("auth-service"))
	app.Use(svcMetrics.FiberMiddleware())
	app.Use(helmet.New())
	app.Use(cors.New(cors.Config{
		AllowOrigins: []string{"*"},
		AllowHeaders: []string{"Authorization", "Content-Type", "X-Request-ID", "X-Idempotency-Key"},
		AllowMethods: []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
	}))

	// ── Routes ─────────────────────────────────────────────────
	authMW := middleware.RequireAuth(jwtMgr)
	authHandler.Register(app, authMW)

	// Swagger UI
	app.Use(swagger.New(swagger.Config{
		BasePath: "/",
		FilePath: "./docs/swagger.json",
		Path:     "docs",
		Title:    "XynPOS Auth Service — API Docs",
	}))

	// Health checks
	healthHandler := health.New("auth-service", db, rdb)
	healthHandler.Register(app)

	// ── Metrics HTTP server (internal) ─────────────────────────
	go func() {
		mux := http.NewServeMux()
		mux.Handle("/metrics", metrics.PrometheusHandler())
		addr := fmt.Sprintf(":%d", port+1000)
		log.Info("Metrics server listening", zap.String("addr", addr))
		srv := &http.Server{Addr: addr, Handler: mux, ReadTimeout: 5 * time.Second, WriteTimeout: 5 * time.Second}
		_ = srv.ListenAndServe()
	}()

	// ── gRPC Server ────────────────────────────────────────────
	grpcServer := grpc.NewServer(
		grpc.ChainUnaryInterceptor(
			tracer.UnaryServerInterceptor(),
		),
	)
	// Register auth gRPC service
	authGRPCServer := grpcdelivery.NewAuthServer(authUC, log)
	authpb.RegisterAuthServiceServer(grpcServer, authGRPCServer)
	
	go func() {
		grpcPort := cfg.GRPC.Port
		if grpcPort == "" {
			grpcPort = fmt.Sprintf("%d", port+1)
		}
		grpcAddr := ":" + grpcPort
		lis, err := net.Listen("tcp", grpcAddr)
		if err != nil {
			log.Fatal("failed to listen gRPC", zap.Error(err))
		}
		log.Info("gRPC server listening", zap.String("addr", grpcAddr))
		if err := grpcServer.Serve(lis); err != nil {
			log.Error("gRPC server error", zap.Error(err))
		}
	}()

	// ── Graceful shutdown ──────────────────────────────────────
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		addr := fmt.Sprintf(":%d", port)
		log.Info("HTTP server listening", zap.String("addr", addr))
		if err := app.Listen(addr); err != nil {
			log.Error("HTTP server error", zap.Error(err))
		}
	}()

	<-quit
	log.Info("Shutdown signal received")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	grpcServer.GracefulStop()
	_ = app.ShutdownWithContext(shutdownCtx)
	log.Info("auth-service stopped gracefully")
}

// stubTenantClient is a placeholder until tenant-service is built.
// Implements usecase.TenantServiceClient.
type stubTenantClient struct{}

func (s *stubTenantClient) CreateTenant(ctx context.Context, ownerUserID uuid.UUID, input domain.RegisterInput) (uuid.UUID, error) {
	// Returns a fixed tenant ID for local development without tenant-service
	return uuid.MustParse("00000000-0000-0000-0000-000000000001"), nil
}
