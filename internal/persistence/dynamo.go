package persistence

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/credentials"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	appconfig "plusplus/internal/config"
)

// Attribute names, shared across repositories and table creation.
const (
	attrTeamID       = "team_id"
	attrUserID       = "user_id"
	attrChannelID    = "channel_id"
	attrKarmaTotal   = "karma_total"
	attrKarmaMax     = "karma_max"
	attrLastActivity = "last_activity_at"
	attrReplyMode    = "reply_mode"
	attrSnarkLevel   = "snark_level"
	attrUpdatedBy    = "updated_by"
	attrUpdatedAt    = "updated_at"
	attrBotToken     = "bot_token_ciphertext"
	attrInstalledAt  = "installed_at"
	attrRecentJSON   = "recent_json"

	leaderboardIndex = "leaderboard"
)

// NewDynamoClient builds a DynamoDB client. Credentials come from the AWS SDK
// default chain (AWS_ACCESS_KEY_ID / AWS_SECRET_ACCESS_KEY on Railway). When
// DYNAMODB_ENDPOINT is set (DynamoDB Local) and no real credentials are present,
// dummy static credentials are injected so the SDK does not block on resolution.
func NewDynamoClient(ctx context.Context, cfg appconfig.Config) (*dynamodb.Client, error) {
	optFns := []func(*awsconfig.LoadOptions) error{
		awsconfig.WithRegion(cfg.AWSRegion),
	}
	if cfg.DynamoDBEndpoint != "" && os.Getenv("AWS_ACCESS_KEY_ID") == "" {
		optFns = append(optFns, awsconfig.WithCredentialsProvider(
			credentials.NewStaticCredentialsProvider("local", "local", ""),
		))
	}

	awsCfg, err := awsconfig.LoadDefaultConfig(ctx, optFns...)
	if err != nil {
		return nil, fmt.Errorf("load aws config: %w", err)
	}

	return dynamodb.NewFromConfig(awsCfg, func(o *dynamodb.Options) {
		if cfg.DynamoDBEndpoint != "" {
			o.BaseEndpoint = aws.String(cfg.DynamoDBEndpoint)
		}
	}), nil
}

// EnsureTables creates the karma, settings, and workspaces tables if they do not
// already exist, then waits for each to become ACTIVE. Safe to call on every boot.
func EnsureTables(ctx context.Context, client *dynamodb.Client, cfg appconfig.Config) error {
	log.Printf("dynamodb: ensuring tables")

	creates := []*dynamodb.CreateTableInput{
		{
			TableName:   aws.String(cfg.KarmaTable),
			BillingMode: types.BillingModePayPerRequest,
			AttributeDefinitions: []types.AttributeDefinition{
				{AttributeName: aws.String(attrTeamID), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String(attrUserID), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String(attrKarmaTotal), AttributeType: types.ScalarAttributeTypeN},
			},
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String(attrTeamID), KeyType: types.KeyTypeHash},
				{AttributeName: aws.String(attrUserID), KeyType: types.KeyTypeRange},
			},
			GlobalSecondaryIndexes: []types.GlobalSecondaryIndex{
				{
					IndexName: aws.String(leaderboardIndex),
					KeySchema: []types.KeySchemaElement{
						{AttributeName: aws.String(attrTeamID), KeyType: types.KeyTypeHash},
						{AttributeName: aws.String(attrKarmaTotal), KeyType: types.KeyTypeRange},
					},
					Projection: &types.Projection{ProjectionType: types.ProjectionTypeAll},
				},
			},
		},
		{
			TableName:   aws.String(cfg.SettingsTable),
			BillingMode: types.BillingModePayPerRequest,
			AttributeDefinitions: []types.AttributeDefinition{
				{AttributeName: aws.String(attrTeamID), AttributeType: types.ScalarAttributeTypeS},
				{AttributeName: aws.String(attrChannelID), AttributeType: types.ScalarAttributeTypeS},
			},
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String(attrTeamID), KeyType: types.KeyTypeHash},
				{AttributeName: aws.String(attrChannelID), KeyType: types.KeyTypeRange},
			},
		},
		{
			TableName:   aws.String(cfg.WorkspacesTable),
			BillingMode: types.BillingModePayPerRequest,
			AttributeDefinitions: []types.AttributeDefinition{
				{AttributeName: aws.String(attrTeamID), AttributeType: types.ScalarAttributeTypeS},
			},
			KeySchema: []types.KeySchemaElement{
				{AttributeName: aws.String(attrTeamID), KeyType: types.KeyTypeHash},
			},
		},
	}

	for _, input := range creates {
		if err := ensureTable(ctx, client, input); err != nil {
			return err
		}
	}

	log.Printf("dynamodb: tables ready")
	return nil
}

// ensureTable is describe-first: if the table already exists (e.g. provisioned
// by Terraform), it only waits for ACTIVE and needs nothing more than
// dynamodb:DescribeTable. It creates the table only when missing (local dev /
// DynamoDB Local), which is the only path that needs dynamodb:CreateTable.
func ensureTable(ctx context.Context, client *dynamodb.Client, input *dynamodb.CreateTableInput) error {
	name := aws.ToString(input.TableName)

	_, err := client.DescribeTable(ctx, &dynamodb.DescribeTableInput{TableName: input.TableName})
	switch {
	case err == nil:
		return waitActive(ctx, client, input.TableName)
	case isNotFound(err):
		log.Printf("dynamodb: table %s missing, creating", name)
		if _, err := client.CreateTable(ctx, input); err != nil {
			var inUse *types.ResourceInUseException
			if errors.As(err, &inUse) {
				return waitActive(ctx, client, input.TableName)
			}
			return fmt.Errorf("create table %s: %w", name, err)
		}
		return waitActive(ctx, client, input.TableName)
	default:
		return fmt.Errorf("describe table %s: %w", name, err)
	}
}

func waitActive(ctx context.Context, client *dynamodb.Client, name *string) error {
	waiter := dynamodb.NewTableExistsWaiter(client)
	if err := waiter.Wait(ctx, &dynamodb.DescribeTableInput{TableName: name}, 2*time.Minute); err != nil {
		return fmt.Errorf("wait for table %s: %w", aws.ToString(name), err)
	}
	return nil
}

func isNotFound(err error) bool {
	var notFound *types.ResourceNotFoundException
	return errors.As(err, &notFound)
}
