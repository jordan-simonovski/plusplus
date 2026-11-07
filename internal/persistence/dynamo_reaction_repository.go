package persistence

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// reactionRowPrefix marks per-(message, reactor) rows in the karma table. Like the
// karma-war sentinel they carry no karma_total, so the leaderboard GSI skips them.
const reactionRowPrefix = "#rx#"

// reactionTierAttrPrefix prefixes one attribute per active reaction, holding the tier
// it was worth when added. Removals undo exactly that, even if the emoji map changed.
const reactionTierAttrPrefix = "t_"

// DynamoReactionStore tracks the tier emojis each reactor has on a message. Every
// change is one atomic UpdateItem returning the row as it was before, so concurrent
// reactions from the same person can't double-count.
// ponytail: rows are never expired; add a DynamoDB TTL attribute if storage ever matters.
type DynamoReactionStore struct {
	client *dynamodb.Client
	table  string
}

func NewDynamoReactionStore(client *dynamodb.Client, table string) *DynamoReactionStore {
	return &DynamoReactionStore{client: client, table: table}
}

// AddReaction records emoji at tier and returns the reactor's previous emoji -> tier map.
func (s *DynamoReactionStore) AddReaction(ctx context.Context, teamID, channelID, messageTS, reactorID, emoji string, tier int) (map[string]int, error) {
	out, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                 aws.String(s.table),
		Key:                       s.key(teamID, channelID, messageTS, reactorID),
		UpdateExpression:          aws.String("SET #e = :tier"),
		ExpressionAttributeNames:  map[string]string{"#e": reactionTierAttrPrefix + emoji},
		ExpressionAttributeValues: map[string]types.AttributeValue{":tier": &types.AttributeValueMemberN{Value: strconv.Itoa(tier)}},
		ReturnValues:              types.ReturnValueAllOld,
	})
	if err != nil {
		return nil, fmt.Errorf("add reaction: %w", err)
	}
	return storedTiers(out.Attributes), nil
}

// RemoveReaction forgets emoji and returns the previous map. Removing an emoji that
// was never recorded (not a tier emoji when added) is a no-op that writes nothing.
func (s *DynamoReactionStore) RemoveReaction(ctx context.Context, teamID, channelID, messageTS, reactorID, emoji string) (map[string]int, error) {
	out, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName:                aws.String(s.table),
		Key:                      s.key(teamID, channelID, messageTS, reactorID),
		UpdateExpression:         aws.String("REMOVE #e"),
		ConditionExpression:      aws.String("attribute_exists(#e)"),
		ExpressionAttributeNames: map[string]string{"#e": reactionTierAttrPrefix + emoji},
		ReturnValues:             types.ReturnValueAllOld,
	})
	var notStored *types.ConditionalCheckFailedException
	if errors.As(err, &notStored) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("remove reaction: %w", err)
	}
	return storedTiers(out.Attributes), nil
}

func (s *DynamoReactionStore) key(teamID, channelID, messageTS, reactorID string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		attrTeamID: &types.AttributeValueMemberS{Value: teamID},
		attrUserID: &types.AttributeValueMemberS{Value: reactionRowPrefix + channelID + "#" + messageTS + "#" + reactorID},
	}
}

func storedTiers(item map[string]types.AttributeValue) map[string]int {
	tiers := map[string]int{}
	for name, value := range item {
		emoji, ok := strings.CutPrefix(name, reactionTierAttrPrefix)
		num, isNum := value.(*types.AttributeValueMemberN)
		if !ok || !isNum {
			continue
		}
		if tier, err := strconv.Atoi(num.Value); err == nil {
			tiers[emoji] = tier
		}
	}
	return tiers
}
