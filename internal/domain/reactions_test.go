package domain

import "testing"

func TestNormalizeEmoji(t *testing.T) {
	cases := map[string]string{
		":clap::skin-tone-3:": "clap",
		"Fire":                "fire",
		":-1:":                "-1",
		":party-parrot:":      "party-parrot",
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

func TestReactionEffect(t *testing.T) {
	tiers := ResolveReactionTiers()
	cases := []struct {
		emojis []string
		want   int
	}{
		{nil, 0},
		{[]string{"clap"}, 1},
		{[]string{"clap", "fire", "star"}, 4},      // highest tier only
		{[]string{"trophy", "wastebasket"}, 0},     // +5 and -5 cancel
		{[]string{"fire", "-1"}, 2},                // +4 - 2
		{[]string{"thumbsdown", "clown_face"}, -3}, // strongest negative only
		{[]string{"tada"}, 0},                      // not a tier emoji
	}
	for _, c := range cases {
		if got := ReactionEffect(tiers, c.emojis); got != c.want {
			t.Errorf("ReactionEffect(%v) = %d, want %d", c.emojis, got, c.want)
		}
	}
}
