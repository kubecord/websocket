package main

import (
	"encoding/json"
	"time"
)

type GatewayPayload struct {
	Op       uint32          `json:"op"`
	Data     json.RawMessage `json:"d"`
	Sequence int64           `json:"s"`
	Event    string          `json:"t"`
	Struct   interface{}     `json:"-"`
}

type OutgoingPayload struct {
	Op   uint32      `json:"op"`
	Data interface{} `json:"d"`
}

type IdentifyProperties struct {
	OS      string `json:"$os"`
	Browser string `json:"$browser"`
	Device  string `json:"$device"`
}

type Identify struct {
	Token          string             `json:"token"`
	Properties     IdentifyProperties `json:"properties"`
	Compress       bool               `json:"compress"`
	LargeThreshold uint32             `json:"large_threshold"`
	Shard          *[2]int            `json:"shard"`
}

type Resume struct {
	Token     string `json:"token"`
	SessionId string `json:"session_id"`
	Sequence  uint32 `json:"seq"`
}

type RequestGuildMembers struct {
	GuildID string `json:"guild_id"`
	Query   string `json:"query"`
	Limit   uint32 `json:"limit"`
}

type PresenceUpdatePayload struct {
	Since uint32 `json:"since"`
	Game  struct {
		Name string `json:"name"`
		Type uint32 `json:"type"`
	} `json:"game"`
	Status string `json:"status"`
	Afk    bool   `json:"afk"`
}

type Hello struct {
	Interval time.Duration `json:"heartbeat_interval"`
	Trace    []string      `json:"_trace"`
}

type Ready struct {
	Version         uint32   `json:"v"`
	User            User     `json:"user"`
	PrivateChannels []string `json:"private_channels"`
	Guilds          []Guild  `json:"guilds"`
	SessionId       string   `json:"session_id"`
	Trace           []string `json:"_trace"`
	Shard           []uint32 `json:"shard"`
}

type Resumed struct {
	Trace []string `json:"_trace"`
}

type HeartBeatOp struct {
	Op       int   `json:"op"`
	Sequence int64 `json:"d"`
}

type ResumeData struct {
	Token     string `json:"token"`
	SessionID string `json:"session_id"`
	Sequence  int64  `json:"seq"`
}

type ResumePayload struct {
	Op   int        `json:"op"`
	Data ResumeData `json:"d"`
}

type GatewayBotResponse struct {
	URL    string `json:"url"`
	Shards int    `json:"shards"`
}

/* Gateway objects */

type User struct {
	Id            string `json:"id"`
	Username      string `json:"username"`
	Discriminator string `json:"discriminator"`
	Avatar        string `json:"avatar"`
	Bot           bool   `json:"bot"`
	MfaEnabled    bool   `json:"mfa_enabled"`
	Locale        string `json:"locale"`
	Verified      bool   `json:"verified"`
	Email         string `json:"email"`
	Flags         uint32 `json:"flags"`
	Premium       uint32 `json:"premium_type"`
}

type Guild struct {
	Id                          string           `json:"id"`
	Name                        string           `json:"name"`
	Icon                        string           `json:"icon"`
	Splash                      string           `json:"splash"`
	Owner                       bool             `json:"owner"`
	OwnerId                     string           `json:"owner_id"`
	Permissions                 uint32           `json:"permissions"`
	Region                      string           `json:"region"`
	AfkChannelId                string           `json:"afk_channel_id"`
	AfkTimeout                  uint32           `json:"afk_timeout"`
	EmbedEnabled                bool             `json:"embed_enabled"`
	EmbedChannelId              string           `json:"embed_channel_id"`
	VerificationLevel           uint32           `json:"verification_level"`
	DefaultMessageNotifications uint32           `json:"default_message_notifications"`
	ExplicitContentFilter       uint32           `json:"explicit_content_filter"`
	Roles                       []Role           `json:"roles"`
	Emojis                      []Emoji          `json:"emojis"`
	Features                    []string         `json:"features"`
	MfaLevel                    uint32           `json:"mfa_level"`
	ApplicationId               string           `json:"application_id"`
	WidgetEnabled               bool             `json:"widget_enabled"`
	WidgetChannelId             string           `json:"widget_channel_id"`
	SystemChannelId             string           `json:"system_channel_id"`
	JoinedAt                    string           `json:"joined_at"`
	Large                       bool             `json:"large"`
	Unavailable                 bool             `json:"unavailable"`
	MemberCount                 uint32           `json:"member_count"`
	VoiceStates                 []VoiceState     `json:"voice_states"`
	Members                     []GuildMember    `json:"members"`
	Channels                    []Channel        `json:"channels"`
	Presences                   []PresenceUpdate `json:"presences"`
}

type Role struct {
	Id          string `json:"id"`
	Name        string `json:"name"`
	Color       uint32 `json:"color"`
	Hoist       bool   `json:"hoist"`
	Position    uint32 `json:"position"`
	Permissions uint32 `json:"permissions"`
	Managed     bool   `json:"managed"`
	Mentionable bool   `json:"mentionable"`
}

type Emoji struct {
	Id            string   `json:"id"`
	Name          string   `json:"name"`
	Roles         []string `json:"roles"`
	User          User     `json:"user"`
	RequireColons bool     `json:"require_colons"`
	Managed       bool     `json:"managed"`
	Animated      bool     `json:"animated"`
}

type VoiceState struct {
	GuildId      string      `json:"guild_id"`
	ChannelId    string      `json:"channel_id"`
	UserId       string      `json:"user_id"`
	Member       GuildMember `json:"member"`
	SessionId    string      `json:"session_id"`
	Deafened     bool        `json:"deaf"`
	Muted        bool        `json:"mute"`
	SelfDeafened bool        `json:"self_deaf"`
	SelfMuted    bool        `json:"self_mute"`
	Suppressed   bool        `json:"suppress"`
}

type GuildMember struct {
	User     User     `json:"user"`
	Nick     string   `json:"nick"`
	Roles    []string `json:"roles"`
	JoinedAt string   `json:"joined_at"`
	Deafened bool     `json:"deaf"`
	Muted    bool     `json:"mute"`
}

type Channel struct {
	Id               string      `json:"id"`
	Type             uint32      `json:"type"`
	GuildId          string      `json:"guild_id"`
	Position         uint32      `json:"position"`
	Overwrites       []Overwrite `json:"permission_overwrites"`
	Name             string      `json:"name"`
	Topic            string      `json:"topic"`
	NSFW             bool        `json:"nsfw"`
	LastMessageId    string      `json:"last_message_id"`
	Bitrate          uint32      `json:"bitrate"`
	UserLimit        uint32      `json:"user_limit"`
	Slowmode         uint32      `json:"rate_limit_per_user"`
	Recipients       []User      `json:"recipients"`
	Icon             string      `json:"icon"`
	OwnerId          string      `json:"owner_id"`
	ApplicationId    string      `json:"application_id"`
	ParentId         string      `json:"parent_id"`
	LastPinTimestamp string      `json:"last_pin_timestamp"`
}

type Overwrite struct {
	Id    string `json:"id"`
	Type  string `json:"type"`
	Allow uint32 `json:"allow"`
	Deny  uint32 `json:"deny"`
}

type PresenceUpdate struct{}
