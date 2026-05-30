package persistence

import (
	"context"
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
