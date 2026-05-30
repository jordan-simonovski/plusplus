package main

import (
	"bufio"
	"context"
	"encoding/hex"
	"fmt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"

	"plusplus/internal/config"
)

// copyHeader matches a pg_dump COPY directive, e.g.
// COPY "public"."karma_totals" ("team_id", "user_id", ...) FROM stdin;
var copyHeader = regexp.MustCompile(`^COPY\s+(?:"?([^".\s]+)"?\.)?"?([^".\s(]+)"?\s*\(([^)]*)\)\s+FROM stdin;`)

// dumpData holds the rows we care about, each as column name -> value.
type dumpData struct {
	karma      []map[string]string
	settings   []map[string]string
	workspaces []map[string]string
}

// parseDump parses a plain-format pg_dump file and extracts the three public
// tables. No Postgres instance required. Other tables (auth.*, storage.*) are
// consumed and ignored.
func parseDump(path string) (dumpData, error) {
	f, err := os.Open(path)
	if err != nil {
		return dumpData{}, fmt.Errorf("open dump %s: %w", path, err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)

	var data dumpData
	for scanner.Scan() {
		m := copyHeader.FindStringSubmatch(scanner.Text())
		if m == nil {
			continue
		}

		schema, table, cols := m[1], m[2], parseColumns(m[3])
		rows := readCopyRows(scanner, cols)

		if schema != "" && schema != "public" {
			continue
		}
		switch table {
		case "karma_totals":
			data.karma = append(data.karma, rows...)
		case "channel_settings":
			data.settings = append(data.settings, rows...)
		case "slack_workspaces":
			data.workspaces = append(data.workspaces, rows...)
		}
	}
	if err := scanner.Err(); err != nil {
		return dumpData{}, fmt.Errorf("read dump: %w", err)
	}
	return data, nil
}

func migrateFromDump(ctx context.Context, path string, client *dynamodb.Client, cfg config.Config) (karma, settings, workspaces int) {
	data, err := parseDump(path)
	if err != nil {
		log.Fatalf("%v", err)
	}

	for _, row := range data.karma {
		putItem(ctx, client, cfg.KarmaTable, karmaItem(row))
		karma++
	}
	for _, row := range data.settings {
		putItem(ctx, client, cfg.SettingsTable, settingsItem(row))
		settings++
	}
	for _, row := range data.workspaces {
		putItem(ctx, client, cfg.WorkspacesTable, workspaceItem(row))
		workspaces++
	}
	return karma, settings, workspaces
}

// readCopyRows consumes lines until the COPY terminator "\." and returns each
// data row as a column name -> value map (values already unescaped, NULLs dropped).
func readCopyRows(scanner *bufio.Scanner, cols []string) []map[string]string {
	var rows []map[string]string
	for scanner.Scan() {
		line := scanner.Text()
		if line == `\.` {
			break
		}
		rows = append(rows, splitCopyLine(line, cols))
	}
	return rows
}

func splitCopyLine(line string, cols []string) map[string]string {
	fields := strings.Split(line, "\t")
	out := make(map[string]string, len(cols))
	for i, col := range cols {
		if i >= len(fields) {
			break
		}
		if fields[i] == `\N` {
			continue // NULL: leave column absent
		}
		out[col] = decodeCopyField(fields[i])
	}
	return out
}

func parseColumns(spec string) []string {
	parts := strings.Split(spec, ",")
	cols := make([]string, 0, len(parts))
	for _, p := range parts {
		cols = append(cols, strings.Trim(strings.TrimSpace(p), `"`))
	}
	return cols
}

func karmaItem(row map[string]string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"team_id":          &types.AttributeValueMemberS{Value: row["team_id"]},
		"user_id":          &types.AttributeValueMemberS{Value: row["user_id"]},
		"karma_total":      &types.AttributeValueMemberN{Value: intString(row["karma_total"])},
		"karma_max":        &types.AttributeValueMemberN{Value: intString(row["karma_max"])},
		"last_activity_at": &types.AttributeValueMemberS{Value: toRFC3339(row["last_activity_at"])},
	}
}

func settingsItem(row map[string]string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"team_id":     &types.AttributeValueMemberS{Value: row["team_id"]},
		"channel_id":  &types.AttributeValueMemberS{Value: row["channel_id"]},
		"reply_mode":  &types.AttributeValueMemberS{Value: row["reply_mode"]},
		"snark_level": &types.AttributeValueMemberN{Value: intString(row["snark_level"])},
		"updated_by":  &types.AttributeValueMemberS{Value: row["updated_by"]},
		"updated_at":  &types.AttributeValueMemberS{Value: toRFC3339(row["updated_at"])},
	}
}

func workspaceItem(row map[string]string) map[string]types.AttributeValue {
	return map[string]types.AttributeValue{
		"team_id":              &types.AttributeValueMemberS{Value: row["team_id"]},
		"bot_token_ciphertext": &types.AttributeValueMemberB{Value: decodeBytea(row["bot_token_ciphertext"])},
		"installed_at":         &types.AttributeValueMemberS{Value: toRFC3339(row["installed_at"])},
	}
}

func intString(s string) string {
	if s == "" {
		return "0"
	}
	if _, err := strconv.Atoi(s); err != nil {
		log.Fatalf("non-integer value %q in dump", s)
	}
	return s
}

// decodeBytea turns a pg_dump bytea hex literal (\x9ba4..., already COPY-unescaped)
// into raw bytes.
func decodeBytea(s string) []byte {
	b, err := hex.DecodeString(strings.TrimPrefix(s, `\x`))
	if err != nil {
		log.Fatalf("decode bytea %q: %v", s, err)
	}
	return b
}

// toRFC3339 reformats a Postgres timestamp to match what the app writes. Falls
// back to the original string if it cannot be parsed.
func toRFC3339(s string) string {
	if s == "" {
		return s
	}
	layouts := []string{
		"2006-01-02 15:04:05.999999-07",
		"2006-01-02 15:04:05-07",
		"2006-01-02 15:04:05.999999-07:00",
	}
	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t.UTC().Format(time.RFC3339)
		}
	}
	return s
}

// decodeCopyField applies Postgres COPY text-format unescaping to a single field.
func decodeCopyField(s string) string {
	if !strings.ContainsRune(s, '\\') {
		return s
	}
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' || i == len(s)-1 {
			b.WriteByte(c)
			continue
		}
		i++
		switch s[i] {
		case '\\':
			b.WriteByte('\\')
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'v':
			b.WriteByte('\v')
		default:
			// Unknown escape: drop the backslash, keep the character (COPY semantics).
			b.WriteByte(s[i])
		}
	}
	return b.String()
}
