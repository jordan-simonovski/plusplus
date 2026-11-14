//go:build integration

package persistence

import (
	"context"
	"errors"
	"reflect"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	appconfig "plusplus/internal/config"
	"plusplus/internal/crypto"
	"plusplus/internal/domain"
)

// integrationSetup connects to DynamoDB Local and ensures the _it tables exist.
// Each test uses team IDs unique to the run, so leftovers from a failed run don't matter.
func integrationSetup(t *testing.T) (context.Context, *dynamodb.Client, appconfig.Config) {
	t.Helper()
	cfg, err := appconfig.Load()
	if err != nil {
		t.Fatalf("load config: %v", err)
	}
	if cfg.DynamoDBEndpoint == "" {
		t.Skip("DYNAMODB_ENDPOINT not set")
	}
	cfg.KarmaTable = "plusplus_karma_it"
	cfg.SettingsTable = "plusplus_channel_settings_it"
	cfg.WorkspacesTable = "plusplus_workspaces_it"

	ctx := context.Background()
	client, err := NewDynamoClient(ctx, cfg)
	if err != nil {
		t.Fatalf("dynamo client: %v", err)
	}
	if err := EnsureTables(ctx, client, cfg); err != nil {
		t.Fatalf("ensure tables: %v", err)
	}
	return ctx, client, cfg
}

func uniqueTeam(prefix string) string {
	return prefix + "-" + strconv.FormatInt(time.Now().UnixNano(), 10)
}

func testEncryptor(t *testing.T, seed byte) *crypto.AESEncryptor {
	t.Helper()
	key := make([]byte, 32)
	for i := range key {
		key[i] = seed + byte(i)
	}
	enc, err := crypto.NewAESEncryptor(key)
	if err != nil {
		t.Fatal(err)
	}
	return enc
}

func TestDynamoWorkspaceRepositoryIntegration(t *testing.T) {
	ctx, client, cfg := integrationSetup(t)
	repo := NewDynamoWorkspaceRepository(client, cfg.WorkspacesTable, testEncryptor(t, 1))
	team := uniqueTeam("T-ws-it")

	if err := repo.UpsertInstallation(ctx, team, "xoxb-first"); err != nil {
		t.Fatal(err)
	}
	if err := repo.UpsertInstallation(ctx, team, "xoxb-second"); err != nil {
		t.Fatal(err)
	}
	got, err := repo.GetBotToken(ctx, team)
	if err != nil || got != "xoxb-second" {
		t.Fatalf("token round trip = %q, %v; want the latest install's token", got, err)
	}

	if _, err := repo.GetBotToken(ctx, uniqueTeam("T-ws-missing")); !errors.Is(err, domain.ErrNotFound) {
		t.Fatalf("unknown team: err = %v, want domain.ErrNotFound", err)
	}

	otherKey := NewDynamoWorkspaceRepository(client, cfg.WorkspacesTable, testEncryptor(t, 100))
	if got, err := otherKey.GetBotToken(ctx, team); err == nil || errors.Is(err, domain.ErrNotFound) || got != "" {
		t.Fatalf("wrong key: got %q, %v; want a decryption error and no token", got, err)
	}
}

func TestDynamoInteractionStoreIntegration(t *testing.T) {
	ctx, client, cfg := integrationSetup(t)
	store := NewDynamoInteractionStore(client, cfg.KarmaTable)
	team := uniqueTeam("T-war-it")

	empty, err := store.LoadRecent(ctx, team)
	if err != nil || len(empty.Recent) != 0 || len(empty.Cooldowns) != 0 {
		t.Fatalf("unknown team: window = %+v, %v; want empty, no error", empty, err)
	}

	want := domain.InteractionWindow{
		Recent: []domain.Interaction{
			{Actor: "U1", Target: "U2", Sign: 1, UnixAt: 100},
			{Actor: "U2", Target: "S9", IsGroup: true, Sign: -1, UnixAt: 101},
		},
		Cooldowns: map[string]int64{"recip:U1|U2": 100},
	}
	if err := store.SaveRecent(ctx, team, want); err != nil {
		t.Fatal(err)
	}
	got, err := store.LoadRecent(ctx, team)
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("round trip = %+v, %v; want %+v", got, err, want)
	}

	if _, err := client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(cfg.KarmaTable),
		Item: map[string]types.AttributeValue{
			attrTeamID:     &types.AttributeValueMemberS{Value: team},
			attrUserID:     &types.AttributeValueMemberS{Value: recentSentinelUserID},
			attrRecentJSON: &types.AttributeValueMemberS{Value: "{not json"},
		},
	}); err != nil {
		t.Fatal(err)
	}
	corrupt, err := store.LoadRecent(ctx, team)
	if err != nil || len(corrupt.Recent) != 0 || len(corrupt.Cooldowns) != 0 {
		t.Fatalf("corrupt blob: window = %+v, %v; want empty, no error", corrupt, err)
	}
}

// The karma table also holds bookkeeping rows (#recent, #rx#…) that must never reach a
// leaderboard (AGENTS.md rule 5), and one team's rows must never show on another's.
func TestDynamoLeaderboardOnlyKarmaRowsIntegration(t *testing.T) {
	ctx, client, cfg := integrationSetup(t)
	karma := NewDynamoKarmaRepository(client, cfg.KarmaTable)
	team, other := uniqueTeam("T-lb-it"), uniqueTeam("T-lb-other")

	for user, delta := range map[string]int{"U1": 2, "U2": 7, "U3": -1} {
		if _, err := karma.ApplyDelta(ctx, team, user, delta); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := karma.ApplyDelta(ctx, other, "U9", 50); err != nil {
		t.Fatal(err)
	}
	if err := NewDynamoInteractionStore(client, cfg.KarmaTable).SaveRecent(ctx, team, domain.InteractionWindow{
		Recent: []domain.Interaction{{Actor: "U1", Target: "U2", Sign: 1, UnixAt: 1}},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := NewDynamoReactionStore(client, cfg.KarmaTable).AddReaction(ctx, team, "C1", "1.0", "U1", "fire", 4); err != nil {
		t.Fatal(err)
	}

	board, err := karma.GetLeaderboard(ctx, team, 10)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range board {
		got = append(got, r.UserID+"="+strconv.Itoa(r.KarmaTotal))
	}
	if want := []string{"U2=7", "U1=2", "U3=-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("leaderboard = %v, want %v", got, want)
	}
}

func TestDynamoApplyDeltaConcurrentIntegration(t *testing.T) {
	ctx, client, cfg := integrationSetup(t)
	karma := NewDynamoKarmaRepository(client, cfg.KarmaTable)
	team := uniqueTeam("T-conc-it")

	const n = 50
	var wg sync.WaitGroup
	errs := make(chan error, n)
	for range n {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if _, err := karma.ApplyDelta(ctx, team, "U1", 1); err != nil {
				errs <- err
			}
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		t.Fatalf("concurrent ApplyDelta: %v", err)
	}

	// A zero delta reads the row back without changing it.
	rec, err := karma.ApplyDelta(ctx, team, "U1", 0)
	if err != nil {
		t.Fatal(err)
	}
	if rec.KarmaTotal != n || rec.KarmaMax != n {
		t.Fatalf("after %d concurrent +1s: total=%d max=%d, want %d and %d", n, rec.KarmaTotal, rec.KarmaMax, n, n)
	}
}
