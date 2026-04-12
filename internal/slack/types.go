package slack

type EventEnvelope struct {
	Type      string     `json:"type"`
	Token     string     `json:"token,omitempty"`
	Challenge string     `json:"challenge,omitempty"`
	TeamID    string     `json:"team_id,omitempty"`
	Event     SlackEvent `json:"event"`
}

type SlackEvent struct {
	Type      string `json:"type"`
	User      string `json:"user"`
	Text      string `json:"text"`
	Channel   string `json:"channel"`
	ThreadTS  string `json:"thread_ts,omitempty"`
	EventTS   string `json:"event_ts"`
	BotID     string `json:"bot_id,omitempty"`
	Subtype   string `json:"subtype,omitempty"`
	ChannelID string `json:"channel_id,omitempty"`
}

type SlashCommandPayload struct {
	Token          string `json:"-"`
	TeamID         string `json:"-"`
	ChannelID      string `json:"-"`
	UserID         string `json:"-"`
	Command        string `json:"-"`
	Text           string `json:"-"`
	ResponseURL    string `json:"-"`
	TriggerID      string `json:"-"`
	ChannelName    string `json:"-"`
	EnterpriseID   string `json:"-"`
	EnterpriseName string `json:"-"`
}

type MessageResponse struct {
	ResponseType string `json:"response_type"`
	Text         string `json:"text"`
	ThreadTS     string `json:"thread_ts,omitempty"`
}
