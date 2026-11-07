package persistence

import (
	"context"
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// reactionRowPrefix marks per-(message, reactor) rows in the karma table. Like the
// karma-war sentinel they carry no karma_total, so the leaderboard GSI skips them.
const reactionRowPrefix = "#rx#"

const attrReactions = "reactions"

// DynamoReactionStore tracks which tier emojis each reactor currently has on a
// message. Every change is one atomic UpdateItem that returns the previous set,
// so concurrent reactions from the same person can't double-count.
// ponytail: rows are never expired; add a DynamoDB TTL attribute if storage ever matters.
type DynamoReactionStore struct {
	client *dynamodb.Client
	table  string
}

func NewDynamoReactionStore(client *dynamodb.Client, table string) *DynamoReactionStore {
	return &DynamoReactionStore{client: client, table: table}
}

func (s *DynamoReactionStore) AddReaction(ctx context.Context, teamID, channelID, messageTS, reactorID, emoji string) ([]string, error) {
	return s.update(ctx, "ADD", teamID, channelID, messageTS, reactorID, emoji)
}

func (s *DynamoReactionStore) RemoveReaction(ctx context.Context, teamID, channelID, messageTS, reactorID, emoji string) ([]string, error) {
	return s.update(ctx, "DELETE", teamID, channelID, messageTS, reactorID, emoji)
}

// update applies ADD or DELETE to the reactor's emoji set and returns the set as it was before.
func (s *DynamoReactionStore) update(ctx context.Context, op, teamID, channelID, messageTS, reactorID, emoji string) ([]string, error) {
	out, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			attrTeamID: &types.AttributeValueMemberS{Value: teamID},
			attrUserID: &types.AttributeValueMemberS{Value: reactionRowPrefix + channelID + "#" + messageTS + "#" + reactorID},
		},
		UpdateExpression:         aws.String(op + " #rx :emoji"),
		ExpressionAttributeNames: map[string]string{"#rx": attrReactions},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":emoji": &types.AttributeValueMemberSS{Value: []string{emoji}},
		},
		ReturnValues: types.ReturnValueUpdatedOld,
	})
	if err != nil {
		return nil, fmt.Errorf("update reaction state: %w", err)
	}
	if set, ok := out.Attributes[attrReactions].(*types.AttributeValueMemberSS); ok {
		return set.Value, nil
	}
	return nil, nil
}
