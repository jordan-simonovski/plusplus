package main

import (
	"context"
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
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	dynamoClient, err := persistence.NewDynamoClient(context.Background(), cfg)
	if err != nil {
		log.Fatalf("init dynamodb client: %v", err)
	}
	if err := persistence.EnsureTables(context.Background(), dynamoClient, cfg); err != nil {
		log.Fatalf("ensure dynamodb tables: %v", err)
	}

	karmaRepo := persistence.NewDynamoKarmaRepository(dynamoClient, cfg.KarmaTable)
	settingsRepo := persistence.NewDynamoSettingsRepository(dynamoClient, cfg.SettingsTable)
	karmaService := domain.NewKarmaService(karmaRepo, domain.RandomSnarkPicker(), cfg.MaxKarmaPerAction)
	settingsService := appslack.NewChannelSettingsService(settingsRepo)

	var workspaceRepo *persistence.DynamoWorkspaceRepository
	// tokenStore stays a nil interface (not a typed-nil) when OAuth is disabled, so
	// TeamResolvingClient correctly selects the single-workspace fallback token.
	var tokenStore appslack.BotTokenStore
	if cfg.WorkspaceEncryptor != nil {
		workspaceRepo = persistence.NewDynamoWorkspaceRepository(dynamoClient, cfg.WorkspacesTable, cfg.WorkspaceEncryptor)
		tokenStore = workspaceRepo
	}
	slackClient := appslack.NewTeamResolvingClient(tokenStore, cfg.SlackBotToken)

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
