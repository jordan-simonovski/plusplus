package main

import (
	"context"
	"database/sql"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"plusplus/internal/config"
	"plusplus/internal/domain"
	transport "plusplus/internal/http"
	"plusplus/internal/persistence"
	appslack "plusplus/internal/slack"
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

	var workspaceRepo *persistence.PostgresWorkspaceRepository
	if cfg.WorkspaceEncryptor != nil {
		workspaceRepo = persistence.NewPostgresWorkspaceRepository(db, cfg.WorkspaceEncryptor)
	}
	slackClient := appslack.NewTeamResolvingClient(workspaceRepo, cfg.SlackBotToken)

	var oauthInstall, oauthCallback http.HandlerFunc
	if cfg.SlackClientID != "" && workspaceRepo != nil {
		oauth := appslack.NewOAuthHandler(cfg.SlackClientID, cfg.SlackClientSecret, cfg.PublicBaseURL, workspaceRepo, cfg.SlackSigningSecret)
		oauthInstall = oauth.Install
		oauthCallback = oauth.Callback
	}

	interactions := appslack.NewInteractionsProcessor(cfg.SlackSigningSecret, settingsService)
	server := transport.NewServer(
		transport.NewEventsHandler(appslack.NewEventsProcessor(cfg.SlackSigningSecret, karmaService, settingsService, slackClient, slackClient)),
		transport.NewCommandsHandler(appslack.NewCommandsProcessor(cfg.SlackSigningSecret, karmaService, settingsService)),
		interactions,
		oauthInstall,
		oauthCallback,
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
