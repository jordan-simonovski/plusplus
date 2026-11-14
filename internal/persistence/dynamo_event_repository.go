package persistence

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// eventClaimPrefix marks per-event claim rows in the karma table. Like the other
// bookkeeping rows they carry no karma_total, so the leaderboard GSI skips them.
const eventClaimPrefix = "#evt#"

// eventClaimTTL outlives Slack's retries (minutes) by a wide margin. Expiry needs DynamoDB
// TTL on expires_at; without it the rows just accumulate.
const eventClaimTTL = 24 * time.Hour

const (
	attrReleased  = "released"
	attrExpiresAt = "expires_at"
)

// DynamoEventClaims records which Slack events have been processed so a retried delivery
// is not applied twice. It uses only PutItem and UpdateItem, which the documented runtime
// IAM policies already grant.
type DynamoEventClaims struct {
	client *dynamodb.Client
	table  string
	now    func() time.Time
}

func NewDynamoEventClaims(client *dynamodb.Client, table string) *DynamoEventClaims {
	return &DynamoEventClaims{client: client, table: table, now: time.Now}
}

// ClaimEvent returns true for the first delivery of an event, and again after
// ReleaseEvent; false while an earlier delivery holds the claim.
func (s *DynamoEventClaims) ClaimEvent(ctx context.Context, teamID, eventID string) (bool, error) {
	_, err := s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.table),
		Item: map[string]types.AttributeValue{
			attrTeamID:    &types.AttributeValueMemberS{Value: teamID},
			attrUserID:    &types.AttributeValueMemberS{Value: eventClaimPrefix + eventID},
			attrExpiresAt: &types.AttributeValueMemberN{Value: strconv.FormatInt(s.now().Add(eventClaimTTL).Unix(), 10)},
		},
		// Replacing the whole item clears `released`, so a released claim can be taken again.
		ConditionExpression:       aws.String("attribute_not_exists(#user) OR #released = :true"),
		ExpressionAttributeNames:  map[string]string{"#user": attrUserID, "#released": attrReleased},
		ExpressionAttributeValues: map[string]types.AttributeValue{":true": &types.AttributeValueMemberBOOL{Value: true}},
	})
	var taken *types.ConditionalCheckFailedException
	if errors.As(err, &taken) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("claim event: %w", err)
	}
	return true, nil
}

// ReleaseEvent lets a later delivery of the event claim it again. Releasing a claim that
// no longer exists (expired) is a no-op rather than creating a row with no expiry.
func (s *DynamoEventClaims) ReleaseEvent(ctx context.Context, teamID, eventID string) error {
	_, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			attrTeamID: &types.AttributeValueMemberS{Value: teamID},
			attrUserID: &types.AttributeValueMemberS{Value: eventClaimPrefix + eventID},
		},
		UpdateExpression:          aws.String("SET #released = :true"),
		ConditionExpression:       aws.String("attribute_exists(#user)"),
		ExpressionAttributeNames:  map[string]string{"#user": attrUserID, "#released": attrReleased},
		ExpressionAttributeValues: map[string]types.AttributeValue{":true": &types.AttributeValueMemberBOOL{Value: true}},
	})
	var missing *types.ConditionalCheckFailedException
	if errors.As(err, &missing) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("release event: %w", err)
	}
	return nil
}
