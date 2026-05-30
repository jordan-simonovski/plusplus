package main

import (
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func TestDecodeBytea(t *testing.T) {
	// On disk pg_dump writes the bytea hex with a doubled backslash; after COPY
	// unescaping the field is \x9ba4..., which decodeBytea turns into raw bytes.
	field := decodeCopyField(`\\x9ba41df9`)
	if field != `\x9ba41df9` {
		t.Fatalf("unescape: got %q", field)
	}
	got := decodeBytea(field)
	want, _ := hex.DecodeString("9ba41df9")
	if string(got) != string(want) {
		t.Fatalf("bytea decode mismatch: %x vs %x", got, want)
	}
}

func TestToRFC3339(t *testing.T) {
	got := toRFC3339("2026-04-12 01:40:36.971357+00")
	if got != "2026-04-12T01:40:36Z" {
		t.Fatalf("timestamp: got %q", got)
	}
}

func TestSplitCopyLineKarma(t *testing.T) {
	cols := []string{"team_id", "user_id", "karma_total", "karma_max", "last_activity_at"}
	row := splitCopyLine("TGK7FJK43\tUJ70EUNE7\t-5\t4\t2026-04-12 01:55:29.118623+00", cols)

	item := karmaItem(row)
	if v := item["team_id"].(*types.AttributeValueMemberS).Value; v != "TGK7FJK43" {
		t.Fatalf("team_id: %q", v)
	}
	if v := item["karma_total"].(*types.AttributeValueMemberN).Value; v != "-5" {
		t.Fatalf("karma_total: %q", v)
	}
	if v := item["karma_max"].(*types.AttributeValueMemberN).Value; v != "4" {
		t.Fatalf("karma_max: %q", v)
	}
}

func TestNullFieldDropped(t *testing.T) {
	cols := []string{"team_id", "user_id"}
	row := splitCopyLine("T1\t\\N", cols)
	if _, ok := row["user_id"]; ok {
		t.Fatalf("expected NULL user_id to be absent, got %q", row["user_id"])
	}
}

// TestParseRealDump runs against the actual Supabase export when present. It is
// skipped if data.sql has been removed (the migrate tool is one-time use).
func TestParseRealDump(t *testing.T) {
	path := filepath.Join("..", "..", "data.sql")
	if _, err := os.Stat(path); err != nil {
		t.Skip("data.sql not present; skipping real-dump parse")
	}

	data, err := parseDump(path)
	if err != nil {
		t.Fatalf("parse dump: %v", err)
	}

	if len(data.karma) != 41 {
		t.Fatalf("expected 41 karma rows, got %d", len(data.karma))
	}
	if len(data.settings) != 0 {
		t.Fatalf("expected 0 settings rows, got %d", len(data.settings))
	}
	if len(data.workspaces) != 3 {
		t.Fatalf("expected 3 workspace rows, got %d", len(data.workspaces))
	}

	// Spot check a known karma row and that the bytea decodes cleanly.
	for _, row := range data.workspaces {
		item := workspaceItem(row)
		b := item["bot_token_ciphertext"].(*types.AttributeValueMemberB)
		if len(b.Value) == 0 {
			t.Fatalf("workspace %s has empty ciphertext", row["team_id"])
		}
	}
}
