package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"plusplus/internal/domain"
	"plusplus/internal/persistence"
	appslack "plusplus/internal/slack"
	"plusplus/internal/config"
	transport "plusplus/internal/http"
	"syscall"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	db, err := sql.Open("pgx", cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("open postgres: %v", err)
	}
	defer db.Close()
	if err := db.PingContext(context.Background()); err != nil {
		log.Fatalf("ping postgres: %v", err)
	}
	if err := persistence.RunMigrations(context.Background(), db); err != nil {
		log.Fatalf("run migrations: %v", err)
	}

	karmaRepo := persistence.NewPostgresKarmaRepository(db)
	settingsRepo := persistence.NewPostgresSettingsRepository(db)
	karmaService := domain.NewKarmaService(karmaRepo, domain.RandomSnarkPicker(), cfg.MaxKarmaPerAction)
	settingsService := appslack.NewChannelSettingsService(settingsRepo)
	slackClient := appslack.NewAPIClient(cfg.SlackBotToken)

	interactions := appslack.NewInteractionsProcessor(cfg.SlackSigningSecret, settingsService)
	server := transport.NewServer(
		transport.NewEventsHandler(appslack.NewEventsProcessor(cfg.SlackSigningSecret, karmaService, settingsService, slackClient, slackClient)),
		transport.NewCommandsHandler(appslack.NewCommandsProcessor(cfg.SlackSigningSecret, karmaService, settingsService)),
		interactions,
	)

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		log.Printf("server listening on :%s", cfg.Port)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("http server failed: %v", err)
		}
	}()

	signalCtx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	<-signalCtx.Done()
	stop()

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
		os.Exit(1)
	}
}
