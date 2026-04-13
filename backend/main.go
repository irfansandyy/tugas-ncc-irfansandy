package main

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"app-backend/config"
	"app-backend/handlers"
	"app-backend/middleware"
	"app-backend/repositories"
	"app-backend/services"

	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/cors"
	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	cfg := config.Load()
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to open database", slog.String("error", err.Error()))
		os.Exit(1)
	}
	defer db.Close()

	db.SetMaxOpenConns(cfg.DBMaxOpenConns)
	db.SetMaxIdleConns(cfg.DBMaxIdleConns)
	db.SetConnMaxLifetime(cfg.DBConnMaxLifetime)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := db.PingContext(ctx); err != nil {
		logger.Error("failed to ping database", slog.String("error", err.Error()))
		os.Exit(1)
	}

	userRepo := repositories.NewPostgresUserRepository(db)
	chatRepo := repositories.NewPostgresChatRepository(db)

	authService := services.NewAuthService(userRepo, cfg.JWTSecret, cfg.TokenTTL)
	llmService := services.GetLLMService(cfg)
	chatService := services.NewChatService(chatRepo, llmService)
	healthService := services.NewHealthService(db, llmService)

	authHandler := handlers.NewAuthHandler(authService)
	chatHandler := handlers.NewChatHandler(chatService)
	healthHandler := handlers.NewHealthHandler(healthService)

	router := chi.NewRouter()
	router.Use(chiMiddleware.Recoverer)
	router.Use(chiMiddleware.RequestID)
	router.Use(middleware.RequestLogger(logger))
	router.Use(middleware.RateLimit(cfg.RateLimitRequestsPerS, cfg.RateLimitBurstRequests))
	router.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{cfg.AllowedOrigin},
		AllowedMethods:   []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type"},
		AllowCredentials: true,
		MaxAge:           300,
	}))

	router.Get("/health", healthHandler.Health)

	router.Route("/api", func(api chi.Router) {
		api.Route("/auth", func(auth chi.Router) {
			auth.Post("/register", authHandler.Register)
			auth.Post("/login", authHandler.Login)
		})

		api.Group(func(protected chi.Router) {
			protected.Use(middleware.JWTAuth(authService))
			protected.Get("/chats", chatHandler.ListChats)
			protected.Post("/chats", chatHandler.CreateChat)
			protected.Get("/chats/{chatSlug}/messages", chatHandler.ListMessages)
			protected.Post("/chats/{chatSlug}/messages", chatHandler.SendMessage)
			protected.Post("/chats/{chatSlug}/messages/stream", chatHandler.SendMessageStream)
		})
	})

	addr := fmt.Sprintf(":%s", cfg.AppPort)
	server := &http.Server{
		Addr:         addr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 120 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	shutdownComplete := make(chan struct{})
	go func() {
		sigCh := make(chan os.Signal, 1)
		signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
		<-sigCh

		shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer shutdownCancel()
		_ = server.Shutdown(shutdownCtx)
		close(shutdownComplete)
	}()

	logger.Info("server_started", slog.String("address", addr))
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		logger.Error("server_failed", slog.String("error", err.Error()))
		os.Exit(1)
	}

	<-shutdownComplete
	logger.Info("server_stopped")
}
