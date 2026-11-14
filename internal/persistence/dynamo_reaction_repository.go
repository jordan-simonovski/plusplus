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
// It also remembers who each typed karma message credited (RecordKarmaTargets).
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

// karmaMessageRowPrefix marks one row per typed karma message, holding who it
// credited, so reactions to that message go to them instead of the poster.
const karmaMessageRowPrefix = "#km#"

const attrKarmaTargets = "targets"

// RecordKarmaTargets remembers the users a typed karma message credited. An empty
// list (a user-group-only message) is still recorded so reactions credit nobody.
func (s *DynamoReactionStore) RecordKarmaTargets(ctx context.Context, teamID, channelID, messageTS string, targets []string) error {
	_, err := s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: aws.String(s.table),
		Item: map[string]types.AttributeValue{
			attrTeamID:       &types.AttributeValueMemberS{Value: teamID},
			attrUserID:       &types.AttributeValueMemberS{Value: karmaMessageRowPrefix + channelID + "#" + messageTS},
			attrKarmaTargets: &types.AttributeValueMemberS{Value: strings.Join(targets, " ")},
		},
	})
	if err != nil {
		return fmt.Errorf("record karma targets: %w", err)
	}
	return nil
}

// KarmaTargets returns the users a message credited; ok is false if it wasn't a
// typed karma message.
func (s *DynamoReactionStore) KarmaTargets(ctx context.Context, teamID, channelID, messageTS string) ([]string, bool, error) {
	out, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: aws.String(s.table),
		Key: map[string]types.AttributeValue{
			attrTeamID: &types.AttributeValueMemberS{Value: teamID},
			attrUserID: &types.AttributeValueMemberS{Value: karmaMessageRowPrefix + channelID + "#" + messageTS},
		},
		// A reaction can land milliseconds after the message is recorded.
		ConsistentRead: aws.Bool(true),
	})
	if err != nil {
		return nil, false, fmt.Errorf("get karma targets: %w", err)
	}
	v, ok := out.Item[attrKarmaTargets].(*types.AttributeValueMemberS)
	if !ok {
		return nil, false, nil
	}
	return strings.Fields(v.Value), true, nil
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
