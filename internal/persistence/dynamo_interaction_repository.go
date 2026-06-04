package persistence

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"plusplus/internal/domain"
)

// recentSentinelUserID is a per-team sentinel row holding the karma-war window.
// It deliberately carries no karma_total attribute, so the sparse leaderboard
// GSI never indexes it.
const recentSentinelUserID = "#recent"

// DynamoInteractionStore persists a team's recent karma activity as a single
// JSON blob on a sentinel item in the karma table. One GetItem to load, one
// PutItem to save; last-writer-wins.
type DynamoInteractionStore struct {
	client *dynamodb.Client
	table  string
}

func NewDynamoInteractionStore(client *dynamodb.Client, table string) *DynamoInteractionStore {
	return &DynamoInteractionStore{client: client, table: table}
}

type recentItem struct {
	TeamID     string `dynamodbav:"team_id"`
	UserID     string `dynamodbav:"user_id"`
	RecentJSON string `dynamodbav:"recent_json"`
}

func (s *DynamoInteractionStore) LoadRecent(ctx context.Context, teamID string) (domain.InteractionWindow, error) {
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			attrTeamID: &types.AttributeValueMemberS{Value: teamID},
			attrUserID: &types.AttributeValueMemberS{Value: recentSentinelUserID},
		},
	})
	if err != nil {
		return domain.InteractionWindow{}, fmt.Errorf("load recent interactions: %w", err)
	}
	if out.Item == nil {
		return domain.InteractionWindow{}, nil
	}

	var item recentItem
	if err := attributevalue.UnmarshalMap(out.Item, &item); err != nil {
		return domain.InteractionWindow{}, fmt.Errorf("decode recent item: %w", err)
	}
	if item.RecentJSON == "" {
		return domain.InteractionWindow{}, nil
	}

	var window domain.InteractionWindow
	if err := json.Unmarshal([]byte(item.RecentJSON), &window); err != nil {
		// Corrupt blob: start fresh rather than wedging karma replies.
		return domain.InteractionWindow{}, nil
	}
	return window, nil
}

func (s *DynamoInteractionStore) SaveRecent(ctx context.Context, teamID string, window domain.InteractionWindow) error {
	blob, err := json.Marshal(window)
	if err != nil {
		return fmt.Errorf("encode recent window: %w", err)
	}

	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.table),
		Item: map[string]types.AttributeValue{
			attrTeamID:     &types.AttributeValueMemberS{Value: teamID},
			attrUserID:     &types.AttributeValueMemberS{Value: recentSentinelUserID},
			attrRecentJSON: &types.AttributeValueMemberS{Value: string(blob)},
		},
	})
	if err != nil {
		return fmt.Errorf("save recent interactions: %w", err)
	}
	return nil
}
