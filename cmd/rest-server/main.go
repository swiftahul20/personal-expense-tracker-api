package main

import (
	"context"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/swiftahul20/expense-tracker/internal/auth"
	"github.com/swiftahul20/expense-tracker/internal/category"
	"github.com/swiftahul20/expense-tracker/internal/config"
	"github.com/swiftahul20/expense-tracker/internal/expense"
	"github.com/swiftahul20/expense-tracker/internal/logger"
	"github.com/swiftahul20/expense-tracker/internal/ratelimit"
	"github.com/swiftahul20/expense-tracker/internal/rest"
	"github.com/swiftahul20/expense-tracker/internal/user"
)

// @title           Expense Tracker API
// @version         1.0
// @description     REST API for tracking personal expenses
// @host            https://01a0d77e-2ac4-781d-a870-26f4e9a39a72-8080.eur-1.aiven.app/
// @BasePath        /
// @schemes         https
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
func main() {
	log := logger.New()

	cfg, err := config.Load()
	if err != nil {
		log.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx := context.Background()
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		log.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	if err := pool.Ping(ctx); err != nil {
		log.Error("failed to ping database", "error", err)
		os.Exit(1)
	}

	expenseStore := expense.NewPostgresStore(pool)
	userStore := user.NewPostgresStore(pool)
	jwtManager := auth.NewJWTManager(cfg.JWTSecret, cfg.JWTTTL)
	loginLimiter := ratelimit.New(5, 15*time.Minute)
	healthHandler := rest.NewHealthHandler(pool)

	expenseHandler := rest.NewHandler(expenseStore)
	authHandler := auth.NewHandler(userStore, jwtManager, cfg.RefreshTTL)
	categoryStore := category.NewPostgresStore(pool)
	categoryHandler := rest.NewCategoryHandler(categoryStore)

	router := rest.NewRouter(expenseHandler, authHandler, jwtManager, loginLimiter, healthHandler, categoryHandler, log)

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	srv := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	go func() {
		log.Info("server starting", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Error("server failed to start", "error", err)
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	log.Info("shutdown signal received, draining connections")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Error("graceful shutdown failed", "error", err)
	} else {
		log.Info("server shut down cleanly")
	}

	pool.Close()
	log.Info("database pool closed")
}
