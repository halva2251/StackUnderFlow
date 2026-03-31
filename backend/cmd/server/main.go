package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	chimw "github.com/go-chi/chi/v5/middleware"
	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"golang.org/x/time/rate"

	"github.com/halva2251/stackunderflow/internal/ai"
	"github.com/halva2251/stackunderflow/internal/config"
	"github.com/halva2251/stackunderflow/internal/database"
	"github.com/halva2251/stackunderflow/internal/handler"
	"github.com/halva2251/stackunderflow/internal/middleware"
	"github.com/halva2251/stackunderflow/internal/repository"
	"github.com/halva2251/stackunderflow/internal/service"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	cfg, err := config.Load()
	if err != nil {
		slog.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	if err := runMigrations(cfg.DatabaseURL); err != nil {
		slog.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := database.Connect(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	r := chi.NewRouter()
	r.Use(chimw.RequestID)
	r.Use(chimw.RealIP)
	r.Use(chimw.Recoverer)
	r.Use(middleware.RequestSizeLimit(1 << 20))
	r.Use(middleware.SecurityHeaders())

	// General rate limiter: 10 req/s with burst of 100
	// Trusted proxies: Docker bridge network (172.20.0.0/16) and localhost.
	// X-Real-IP / X-Forwarded-For headers are only trusted from these sources.
	// This prevents spoofing if the API port is accidentally exposed directly.
	generalLimiter := middleware.NewRateLimiter(rate.Limit(10), 100, []string{"127.0.0.0/8", "172.20.0.0/16"})
	defer generalLimiter.Stop()
	r.Use(generalLimiter.Limit)

	r.Get("/health", handler.Health(pool))

	// Dependencies
	aiClient := ai.NewGroqClient(cfg.GroqAPIKey, cfg.GroqBaseURL, cfg.GroqModel)
	userRepo := repository.NewUserRepository(pool)
	questionRepo := repository.NewQuestionRepository(pool)
	answerRepo := repository.NewAnswerRepository(pool)

	authSvc, err := service.NewAuthService(userRepo, cfg.JWTSecret)
	if err != nil {
		slog.Error("failed to create auth service", "error", err)
		os.Exit(1)
	}
	questionSvc := service.NewQuestionService(service.NewTxBeginner(pool), questionRepo, answerRepo, aiClient)
	voteRepo := repository.NewVoteRepository(pool)
	voteSvc := service.NewVoteService(service.NewTxBeginner(pool), voteRepo)

	authHandler := handler.NewAuthHandler(authSvc)
	questionHandler := handler.NewQuestionHandler(questionSvc)
	voteHandler := handler.NewVoteHandler(voteSvc)

	// Stricter rate limiter for auth endpoints: 1 req per 10 sec
	authLimiter := middleware.NewRateLimiter(rate.Limit(0.1), 1, []string{"127.0.0.0/8", "172.20.0.0/16"})
	defer authLimiter.Stop()

	// API routes
	r.Route("/api/v1", func(r chi.Router) {
		// Public routes with strict rate limiting
		r.With(authLimiter.Limit).Post("/auth/register", authHandler.Register)
		r.With(authLimiter.Limit).Post("/auth/login", authHandler.Login)
		r.Get("/questions/{id}", questionHandler.Get)

		// Protected routes
		r.Group(func(r chi.Router) {
			r.Use(middleware.Auth(authSvc))
			r.Post("/questions", questionHandler.Create)
			r.Post("/questions/{id}/argue", questionHandler.Argue)

			r.Put("/questions/{id}/vote", voteHandler.VoteOn("question"))
			r.Delete("/questions/{id}/vote", voteHandler.RemoveVoteOn("question"))
			r.Put("/answers/{id}/vote", voteHandler.VoteOn("answer"))
			r.Delete("/answers/{id}/vote", voteHandler.RemoveVoteOn("answer"))
		})
	})

	srv := &http.Server{
		Addr:           fmt.Sprintf(":%d", cfg.Port),
		Handler:        r,
		ReadTimeout:    10 * time.Second,
		WriteTimeout:   45 * time.Second,
		IdleTimeout:    60 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	// Graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	serverErr := make(chan error, 1)
	go func() {
		slog.Info("server starting", "port", cfg.Port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	select {
	case <-quit:
		slog.Info("shutting down server")
	case err := <-serverErr:
		slog.Error("server failed", "error", err)
		os.Exit(1)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("server forced to shutdown", "error", err)
		os.Exit(1)
	}

	slog.Info("server stopped")
}

func runMigrations(databaseURL string) error {
	// golang-migrate expects the pgx5 scheme for pgx v5 driver
	if !strings.HasPrefix(databaseURL, "postgres://") {
		return fmt.Errorf("unsupported database URL scheme, expected postgres://")
	}
	pgx5URL := "pgx5://" + strings.TrimPrefix(databaseURL, "postgres://")

	m, err := migrate.New("file://migrations", pgx5URL)
	if err != nil {
		return fmt.Errorf("create migrator: %w", err)
	}
	defer m.Close()

	if err := m.Up(); err != nil && err != migrate.ErrNoChange {
		return fmt.Errorf("run migrations: %w", err)
	}

	return nil
}
