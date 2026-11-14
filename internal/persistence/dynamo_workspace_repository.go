package persistence

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"plusplus/internal/crypto"
	"plusplus/internal/domain"
)

// DynamoWorkspaceRepository stores per-workspace Slack bot tokens encrypted at rest.
type DynamoWorkspaceRepository struct {
	client *dynamodb.Client
	table  string
	enc    *crypto.AESEncryptor
}

func NewDynamoWorkspaceRepository(client *dynamodb.Client, table string, enc *crypto.AESEncryptor) *DynamoWorkspaceRepository {
	return &DynamoWorkspaceRepository{client: client, table: table, enc: enc}
}

// UpsertInstallation encrypts and stores the bot token for a workspace.
func (r *DynamoWorkspaceRepository) UpsertInstallation(ctx context.Context, teamID, botToken string) error {
	if teamID == "" {
		return fmt.Errorf("team id empty")
	}
	ct, err := r.enc.Encrypt([]byte(botToken))
	if err != nil {
		return fmt.Errorf("encrypt token: %w", err)
	}

	_, err = r.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(r.table),
		Item: map[string]types.AttributeValue{
			attrTeamID:      &types.AttributeValueMemberS{Value: teamID},
			attrBotToken:    &types.AttributeValueMemberB{Value: ct},
			attrInstalledAt: &types.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339)},
		},
	})
	if err != nil {
		return fmt.Errorf("upsert slack workspace: %w", err)
	}
	return nil
}

// GetBotToken decrypts the stored bot token for a workspace. Returns
// domain.ErrNotFound when no installation exists.
func (r *DynamoWorkspaceRepository) GetBotToken(ctx context.Context, teamID string) (string, error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.table),
		Key: map[string]types.AttributeValue{
			attrTeamID: &types.AttributeValueMemberS{Value: teamID},
		},
	})
	if err != nil {
		return "", fmt.Errorf("get slack workspace: %w", err)
	}
	if out.Item == nil {
		return "", domain.ErrNotFound
	}

	blobAttr, ok := out.Item[attrBotToken].(*types.AttributeValueMemberB)
	if !ok {
		return "", domain.ErrNotFound
	}
	pt, err := r.enc.Decrypt(blobAttr.Value)
	if err != nil {
		return "", fmt.Errorf("decrypt token: %w", err)
	}
	return string(pt), nil
}

// RemoveInstallation forgets a workspace's bot token when the app is uninstalled. The row
// stays (installed_at is kept) but has no token, so GetBotToken reports ErrNotFound. An
// unknown workspace is not an error and gets no row. Uses UpdateItem rather than
// DeleteItem, which the documented runtime IAM policies do not grant.
func (r *DynamoWorkspaceRepository) RemoveInstallation(ctx context.Context, teamID string) error {
	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(r.table),
		Key: map[string]types.AttributeValue{
			attrTeamID: &types.AttributeValueMemberS{Value: teamID},
		},
		UpdateExpression:         aws.String("REMOVE #token"),
		ConditionExpression:      aws.String("attribute_exists(#team)"),
		ExpressionAttributeNames: map[string]string{"#token": attrBotToken, "#team": attrTeamID},
	})
	var unknown *types.ConditionalCheckFailedException
	if errors.As(err, &unknown) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("remove slack installation: %w", err)
	}
	return nil
}
