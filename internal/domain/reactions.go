package domain

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// MaxReactionTier is the strongest reaction tier: ±1 through ±5 karma.
const MaxReactionTier = 5

// TierOverrides maps a reaction tier (-5..-1, 1..5) to the emoji names that
// trigger it. A layer only lists the tiers it changes.
type TierOverrides map[int][]string

// DefaultReactionTiers applies when neither the workspace nor the channel has
// customised a tier. Both canonical names and common aliases are listed because
// Slack reports whichever name the emoji is registered under.
var DefaultReactionTiers = TierOverrides{
	1:  {"clap"},
	2:  {"raised_hands"},
	3:  {"star"},
	4:  {"fire"},
	5:  {"trophy"},
	-1: {"face_with_rolling_eyes"},
	-2: {"-1", "thumbsdown"},
	-3: {"clown_face"},
	-4: {"hankey", "poop"},
	-5: {"wastebasket"},
}

var emojiNamePattern = regexp.MustCompile(`^[a-z0-9_+'-]+$`)

func ValidReactionTier(tier int) bool {
	return tier != 0 && tier >= -MaxReactionTier && tier <= MaxReactionTier
}

// NormalizeEmoji turns ":clap::skin-tone-3:" or "Clap" into "clap"; "" if invalid.
func NormalizeEmoji(raw string) string {
	name := strings.ToLower(strings.TrimSpace(raw))
	name = strings.Trim(name, ":")
	if i := strings.Index(name, "::skin-tone-"); i >= 0 {
		name = name[:i]
	}
	if !emojiNamePattern.MatchString(name) {
		return ""
	}
	return name
}

// ResolveReactionTiers layers overrides, least to most specific, on top of the
// defaults. Each tier comes from the most specific layer that sets it, and an
// emoji claimed by a more specific layer wins over a less specific one.
func ResolveReactionTiers(layers ...TierOverrides) map[string]int {
	all := append([]TierOverrides{DefaultReactionTiers}, layers...)
	source := map[int]int{} // tier -> index of the layer it comes from
	for i, layer := range all {
		for tier := range layer {
			if ValidReactionTier(tier) {
				source[tier] = i
			}
		}
	}

	tiers := make([]int, 0, len(source))
	for tier := range source {
		tiers = append(tiers, tier)
	}
	// Assign least specific first so more specific layers overwrite shared emojis.
	sort.Slice(tiers, func(a, b int) bool {
		if source[tiers[a]] != source[tiers[b]] {
			return source[tiers[a]] < source[tiers[b]]
		}
		return tiers[a] < tiers[b]
	})

	resolved := map[string]int{}
	for _, tier := range tiers {
		for _, emoji := range all[source[tier]][tier] {
			resolved[emoji] = tier
		}
	}
	return resolved
}

// ReactionEffect is one reactor's net award on a message: their strongest
// positive tier minus their strongest negative tier, so it stays within ±5.
func ReactionEffect(tiers map[string]int, emojis []string) int {
	best, worst := 0, 0
	for _, emoji := range emojis {
		tier := tiers[emoji]
		if tier > best {
			best = tier
		}
		if tier < worst {
			worst = tier
		}
	}
	return best + worst
}

// FormatReactionTiers renders the resolved map, strongest award first.
func FormatReactionTiers(resolved map[string]int) string {
	byTier := map[int][]string{}
	for emoji, tier := range resolved {
		byTier[tier] = append(byTier[tier], ":"+emoji+":")
	}
	var lines []string
	for tier := MaxReactionTier; tier >= -MaxReactionTier; tier-- {
		emojis := byTier[tier]
		if tier == 0 || len(emojis) == 0 {
			continue
		}
		sort.Strings(emojis)
		lines = append(lines, fmt.Sprintf("`%+d` %s", tier, strings.Join(emojis, " ")))
	}
	return strings.Join(lines, "\n")
}
