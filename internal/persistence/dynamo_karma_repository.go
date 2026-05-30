package persistence

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"plusplus/internal/domain"
)

type DynamoKarmaRepository struct {
	client *dynamodb.Client
	table  string
}

func NewDynamoKarmaRepository(client *dynamodb.Client, table string) *DynamoKarmaRepository {
	return &DynamoKarmaRepository{client: client, table: table}
}

type karmaItem struct {
	TeamID       string `dynamodbav:"team_id"`
	UserID       string `dynamodbav:"user_id"`
	KarmaTotal   int    `dynamodbav:"karma_total"`
	KarmaMax     int    `dynamodbav:"karma_max"`
	LastActivity string `dynamodbav:"last_activity_at"`
}

func (i karmaItem) toRecord() domain.KarmaRecord {
	return domain.KarmaRecord{
		TeamID:       i.TeamID,
		UserID:       i.UserID,
		KarmaTotal:   i.KarmaTotal,
		KarmaMax:     i.KarmaMax,
		LastActivity: i.LastActivity,
	}
}

// ApplyDelta atomically increments the running total and bumps karma_max to the
// new peak. DynamoDB has no GREATEST, so this is two writes worst case: an
// unconditional ADD, then a conditional SET on karma_max that only fires when a
// new peak is reached.
func (r *DynamoKarmaRepository) ApplyDelta(
	ctx context.Context,
	teamID string,
	userID string,
	delta int,
) (domain.KarmaRecord, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	key := map[string]types.AttributeValue{
		attrTeamID: &types.AttributeValueMemberS{Value: teamID},
		attrUserID: &types.AttributeValueMemberS{Value: userID},
	}

	addOut, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:        aws.String(r.table),
		Key:              key,
		UpdateExpression: aws.String("ADD #total :delta SET #last = :now"),
		ExpressionAttributeNames: map[string]string{
			"#total": attrKarmaTotal,
			"#last":  attrLastActivity,
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":delta": &types.AttributeValueMemberN{Value: strconv.Itoa(delta)},
			":now":   &types.AttributeValueMemberS{Value: now},
		},
		ReturnValues: types.ReturnValueAllNew,
	})
	if err != nil {
		return domain.KarmaRecord{}, fmt.Errorf("apply karma delta: %w", err)
	}

	var afterAdd karmaItem
	if err := attributevalue.UnmarshalMap(addOut.Attributes, &afterAdd); err != nil {
		return domain.KarmaRecord{}, fmt.Errorf("decode karma item: %w", err)
	}

	newTotal := strconv.Itoa(afterAdd.KarmaTotal)
	maxOut, err := r.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:           aws.String(r.table),
		Key:                 key,
		UpdateExpression:    aws.String("SET #max = :total"),
		ConditionExpression: aws.String("attribute_not_exists(#max) OR #max < :total"),
		ExpressionAttributeNames: map[string]string{
			"#max": attrKarmaMax,
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":total": &types.AttributeValueMemberN{Value: newTotal},
		},
		ReturnValues: types.ReturnValueAllNew,
	})
	if err != nil {
		var condFailed *types.ConditionalCheckFailedException
		if errors.As(err, &condFailed) {
			// No new peak; the ADD result already carries the current karma_max.
			return afterAdd.toRecord(), nil
		}
		return domain.KarmaRecord{}, fmt.Errorf("update karma max: %w", err)
	}

	var afterMax karmaItem
	if err := attributevalue.UnmarshalMap(maxOut.Attributes, &afterMax); err != nil {
		return domain.KarmaRecord{}, fmt.Errorf("decode karma item: %w", err)
	}
	return afterMax.toRecord(), nil
}

func (r *DynamoKarmaRepository) GetLeaderboard(
	ctx context.Context,
	teamID string,
	limit int,
) ([]domain.KarmaRecord, error) {
	out, err := r.client.Query(ctx, &dynamodb.QueryInput{
		TableName:              aws.String(r.table),
		IndexName:              aws.String(leaderboardIndex),
		KeyConditionExpression: aws.String("#team = :team"),
		ExpressionAttributeNames: map[string]string{
			"#team": attrTeamID,
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":team": &types.AttributeValueMemberS{Value: teamID},
		},
		ScanIndexForward: aws.Bool(false), // highest karma_total first
		Limit:            aws.Int32(int32(limit)),
	})
	if err != nil {
		return nil, fmt.Errorf("query leaderboard: %w", err)
	}

	records := make([]domain.KarmaRecord, 0, len(out.Items))
	for _, item := range out.Items {
		var ki karmaItem
		if err := attributevalue.UnmarshalMap(item, &ki); err != nil {
			return nil, fmt.Errorf("decode leaderboard item: %w", err)
		}
		records = append(records, ki.toRecord())
	}
	return records, nil
}
