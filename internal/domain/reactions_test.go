package domain

import "testing"

func TestNormalizeEmoji(t *testing.T) {
	cases := map[string]string{
		":clap::skin-tone-3:": "clap",
		"Fire":                "fire",
		":-1:":                "-1",
		":party-parrot:":      "party-parrot",
		":thumbsdown:":        "-1",
		"poop":                "hankey",
		"<b>":                 "",
		"":                    "",
	}
	for in, want := range cases {
		if got := NormalizeEmoji(in); got != want {
			t.Errorf("NormalizeEmoji(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestResolveReactionTiersLayering(t *testing.T) {
	workspace := TierOverrides{3: {"partyparrot"}}
	channel := TierOverrides{1: {"fire"}} // steals :fire: from default tier 4
	got := ResolveReactionTiers(workspace, channel)

	want := map[string]int{"partyparrot": 3, "fire": 1, "trophy": 5, "wastebasket": -5}
	for emoji, tier := range want {
		if got[emoji] != tier {
			t.Errorf("%s: got tier %d, want %d", emoji, got[emoji], tier)
		}
	}
	if _, ok := got["star"]; ok {
		t.Errorf("default tier 3 (:star:) should be replaced by the workspace override")
	}
	if _, ok := got["clap"]; ok {
		t.Errorf("default tier 1 (:clap:) should be replaced by the channel override")
	}
}

func TestNetReactionTier(t *testing.T) {
	cases := []struct {
		stored map[string]int
		want   int
	}{
		{nil, 0},
		{map[string]int{"clap": 1}, 1},
		{map[string]int{"clap": 1, "fire": 4, "star": 3}, 4},     // highest tier only
		{map[string]int{"trophy": 5, "wastebasket": -5}, 0},      // +5 and -5 cancel
		{map[string]int{"fire": 4, "-1": -2}, 2},                 // +4 - 2
		{map[string]int{"thumbsdown": -2, "clown_face": -3}, -3}, // strongest negative only
	}
	for _, c := range cases {
		if got := NetReactionTier(c.stored); got != c.want {
			t.Errorf("NetReactionTier(%v) = %d, want %d", c.stored, got, c.want)
		}
	}
}

func TestClampReactionTier(t *testing.T) {
	if ClampReactionTier(5, 3) != 3 || ClampReactionTier(-5, 3) != -3 || ClampReactionTier(2, 3) != 2 || ClampReactionTier(5, 0) != 5 {
		t.Fatal("clamp to the per-action cap")
	}
}
