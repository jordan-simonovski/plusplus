package main

import (
	"context"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"plusplus/internal/config"
	"plusplus/internal/domain"
	transport "plusplus/internal/http"
	"plusplus/internal/persistence"
	appslack "plusplus/internal/slack"
	"plusplus/internal/version"
	"syscall"
	"time"
)

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, nil)))
	// The standard log package now writes through slog. Only fatal startup errors and
	// net/http server errors still use it, so log those at error level.
	slog.SetLogLoggerLevel(slog.LevelError)

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}
	if err := cfg.ValidateServer(); err != nil {
		log.Fatalf("config: %v", err)
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

	nameResolver := appslack.NewCachedNameResolver(slackClient, time.Hour)
	interactionStore := persistence.NewDynamoInteractionStore(dynamoClient, cfg.KarmaTable)
	karmaService := domain.NewKarmaService(karmaRepo, domain.RandomSnarkPicker(), cfg.MaxKarmaPerAction, nameResolver).
		WithKarmaWar(domain.NewKarmaWar(interactionStore))

	var oauthInstall, oauthCallback http.HandlerFunc
	if cfg.SlackClientID != "" && workspaceRepo != nil {
		oauth, err := appslack.NewOAuthHandler(cfg.SlackClientID, cfg.SlackClientSecret, cfg.PublicBaseURL, workspaceRepo, cfg.SlackSigningSecret, cfg.SlackOAuthScopes)
		if err != nil {
			log.Fatalf("oauth: %v", err)
		}
		slog.Info("oauth enabled; set SLACK_OAUTH_SCOPES to change the scopes /slack/install requests", "scopes", oauth.Scopes())
		oauthInstall = oauth.Install
		oauthCallback = oauth.Callback
	}

	reactionSettings := appslack.NewReactionSettingsService(settingsRepo).WithAdminCheck(slackClient)
	reactionState := persistence.NewDynamoReactionStore(dynamoClient, cfg.KarmaTable)

	events := appslack.NewEventsProcessor(karmaService, settingsService, slackClient, slackClient).
		WithReactions(reactionSettings, reactionState, cfg.MaxKarmaPerAction).
		WithEventClaims(persistence.NewDynamoEventClaims(dynamoClient, cfg.KarmaTable))
	if workspaceRepo != nil {
		events.WithInstallationRemover(workspaceRepo)
	}

	commands := appslack.NewCommandsProcessor(karmaService, settingsService, version.Version).
		WithReactionSettings(reactionSettings)
	interactions := appslack.NewInteractionsProcessor(settingsService)
	server := slackServer(cfg.SlackSigningSecret, events, commands, interactions, oauthInstall, oauthCallback)

	httpServer := &http.Server{
		Addr:              ":" + cfg.Port,
		Handler:           server.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
		// ponytail: no WriteTimeout; group karma (capped at 50 members) runs inside the request.
		ReadTimeout: 30 * time.Second,
		IdleTimeout: 120 * time.Second,
	}

	go func() {
		slog.Info("server listening", "port", cfg.Port)
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
		slog.Error("graceful shutdown failed", "err", err)
		os.Exit(1)
	}
}

// slackServer routes the Slack endpoints, each behind the signature check.
func slackServer(signingSecret string, events *appslack.EventsProcessor, commands *appslack.CommandsProcessor, interactions *appslack.InteractionsProcessor, oauthInstall, oauthCallback http.HandlerFunc) *transport.Server {
	return transport.NewServer(
		appslack.VerifiedBody(signingSecret, events.HandleEvent),
		appslack.VerifiedBody(signingSecret, commands.HandleCommand),
		appslack.VerifiedBody(signingSecret, interactions.HandleInteraction),
		oauthInstall,
		oauthCallback,
	)
}
