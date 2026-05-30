package persistence

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

const (
	defaultReplyMode  = "thread"
	defaultSnarkLevel = 5
	minSnarkLevel     = 1
	maxSnarkLevel     = 10
)

type DynamoSettingsRepository struct {
	client *dynamodb.Client
	table  string
}

func NewDynamoSettingsRepository(client *dynamodb.Client, table string) *DynamoSettingsRepository {
	return &DynamoSettingsRepository{client: client, table: table}
}

type settingsItem struct {
	ReplyMode  string `dynamodbav:"reply_mode"`
	SnarkLevel int    `dynamodbav:"snark_level"`
}

func (r *DynamoSettingsRepository) key(teamID, channelID string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		attrTeamID:    &types.AttributeValueMemberS{Value: teamID},
		attrChannelID: &types.AttributeValueMemberS{Value: channelID},
	}
}

func (r *DynamoSettingsRepository) GetChannelSettings(
	ctx context.Context,
	teamID string,
	channelID string,
) (replyMode string, snarkLevel int, err error) {
	out, err := r.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(r.table),
		Key:       r.key(teamID, channelID),
	})
	if err != nil {
		return "", 0, fmt.Errorf("get channel settings: %w", err)
	}
	if out.Item == nil {
		return defaultReplyMode, defaultSnarkLevel, nil
	}

	var item settingsItem
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return "", 0, fmt.Errorf("decode channel settings: %w", err)
	}

	mode := item.ReplyMode
	if mode == "" {
		mode = defaultReplyMode
	}
	level := item.SnarkLevel
	if level < minSnarkLevel || level > maxSnarkLevel {
		level = defaultSnarkLevel
	}
	return mode, level, nil
}

func (r *DynamoSettingsRepository) GetReplyMode(
	ctx context.Context,
	teamID string,
	channelID string,
) (string, error) {
	mode, _, err := r.GetChannelSettings(ctx, teamID, channelID)
	return mode, err
}

// SetReplyMode updates reply_mode and seeds snark_level to its default only when
// the row is new (matching the prior ON CONFLICT semantics).
func (r *DynamoSettingsRepository) SetReplyMode(
	ctx context.Context,
	teamID string,
	channelID string,
	actorUserID string,
	replyMode string,
) error {
	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:        aws.String(r.table),
		Key:              r.key(teamID, channelID),
		UpdateExpression: aws.String("SET #mode = :mode, #by = :by, #at = :at, #snark = if_not_exists(#snark, :defSnark)"),
		ExpressionAttributeNames: map[string]string{
			"#mode":  attrReplyMode,
			"#by":    attrUpdatedBy,
			"#at":    attrUpdatedAt,
			"#snark": attrSnarkLevel,
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":mode":     &types.AttributeValueMemberS{Value: replyMode},
			":by":       &types.AttributeValueMemberS{Value: actorUserID},
			":at":       &types.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339)},
			":defSnark": &types.AttributeValueMemberN{Value: strconv.Itoa(defaultSnarkLevel)},
		},
	})
	if err != nil {
		return fmt.Errorf("set reply mode: %w", err)
	}
	return nil
}

// SetSnarkLevel updates snark_level and seeds reply_mode to its default only when
// the row is new (matching the prior ON CONFLICT semantics).
func (r *DynamoSettingsRepository) SetSnarkLevel(
	ctx context.Context,
	teamID string,
	channelID string,
	actorUserID string,
	snarkLevel int,
) error {
	_, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:        aws.String(r.table),
		Key:              r.key(teamID, channelID),
		UpdateExpression: aws.String("SET #snark = :snark, #by = :by, #at = :at, #mode = if_not_exists(#mode, :defMode)"),
		ExpressionAttributeNames: map[string]string{
			"#snark": attrSnarkLevel,
			"#by":    attrUpdatedBy,
			"#at":    attrUpdatedAt,
			"#mode":  attrReplyMode,
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":snark":   &types.AttributeValueMemberN{Value: strconv.Itoa(snarkLevel)},
			":by":      &types.AttributeValueMemberS{Value: actorUserID},
			":at":      &types.AttributeValueMemberS{Value: time.Now().UTC().Format(time.RFC3339)},
			":defMode": &types.AttributeValueMemberS{Value: defaultReplyMode},
		},
	})
	if err != nil {
		return fmt.Errorf("set snark level: %w", err)
	}
	return nil
}
