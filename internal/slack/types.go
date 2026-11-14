package slack

type ReplyMode string

const (
	ReplyModeThread  ReplyMode = "thread"
	ReplyModeChannel ReplyMode = "channel"
)

type SlackAuthorization struct {
	UserID string `json:"user_id"`
	IsBot  bool   `json:"is_bot"`
}

type EventEnvelope struct {
	Type           string               `json:"type"`
	EventID        string               `json:"event_id,omitempty"`
	EventTime      int64                `json:"event_time,omitempty"`
	Challenge      string               `json:"challenge,omitempty"`
	TeamID         string               `json:"team_id,omitempty"`
	Event          SlackEvent           `json:"event"`
	Authorizations []SlackAuthorization `json:"authorizations,omitempty"`
}

type SlackEvent struct {
	Type     string `json:"type"`
	User     string `json:"user"`
	Text     string `json:"text"`
	Channel  string `json:"channel"`
	ThreadTS string `json:"thread_ts,omitempty"`
	TS       string `json:"ts,omitempty"`
	EventTS  string `json:"event_ts"`
	BotID    string `json:"bot_id,omitempty"`
	Subtype  string `json:"subtype,omitempty"`

	// reaction_added / reaction_removed
	Reaction string       `json:"reaction,omitempty"`
	ItemUser string       `json:"item_user,omitempty"`
	Item     ReactionItem `json:"item,omitempty"`

	// tokens_revoked
	Tokens RevokedTokens `json:"tokens,omitempty"`
}

// RevokedTokens lists the user IDs whose tokens were revoked, by token type.
type RevokedTokens struct {
	Bot []string `json:"bot"`
}

type ReactionItem struct {
	Type    string `json:"type"`
	Channel string `json:"channel"`
	TS      string `json:"ts"`
}

type SlashCommandPayload struct {
	TeamID      string `json:"-"`
	ChannelID   string `json:"-"`
	UserID      string `json:"-"`
	Command     string `json:"-"`
	Text        string `json:"-"`
	ResponseURL string `json:"-"`
	TriggerID   string `json:"-"`
}

type MessageResponse struct {
	ResponseType string `json:"response_type"`
	Text         string `json:"text,omitempty"`
	Blocks       any    `json:"blocks,omitempty"`
}
