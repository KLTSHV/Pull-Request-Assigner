package main

import (
	"PullRequestAssigner/internal/config"
	"PullRequestAssigner/internal/repository/postgres"
	"PullRequestAssigner/internal/service"
	"PullRequestAssigner/internal/stats"
	httptransport "PullRequestAssigner/internal/transport/http"
	"context"
	"database/sql"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"
)

func main() {
	cfg := config.Load()

	// Подключаемся к БД
	db, err := sql.Open("postgres", cfg.DB.DSN)
	if err != nil {
		log.Fatalf("failed to open db: %v", err)
	}
	defer db.Close()

	// Настройки пула соединений
	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(30 * time.Minute)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := db.PingContext(ctx); err != nil {
		log.Fatalf("failed to ping db: %v", err)
	}
	userRepo := postgres.NewUserRepository(db)
	teamRepo := postgres.NewTeamRepository(db)
	prRepo := postgres.NewPullRequestRepository(db)
	statsRepo := postgres.NewStatsRepository(db)

	// Сервисы
	userSvc := service.NewUserService(userRepo)
	teamSvc := service.NewTeamService(teamRepo, userRepo, prRepo)
	prSvc := service.NewPRService(userRepo, teamRepo, prRepo)
	statsSvc := stats.NewService(statsRepo)

	// HTTP-роутер
	router := httptransport.NewRouter(userSvc, teamSvc, prSvc, statsSvc)

	srv := &http.Server{
		Addr:         cfg.HTTPAddr,
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	// Горутина с сервером
	go func() {
		log.Printf("HTTP server listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("http server error: %v", err)
		}
	}()

	// Грейсфул-шатдаун по SIGINT/SIGTERM
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	log.Println("shutting down server")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		log.Printf("server shutdown error: %v", err)
	} else {
		log.Println("server stopped gracefully")
	}
}
