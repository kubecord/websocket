package main

import "time"

// Websocket OpCodes
const (
	OP_DISPATCH              = 0
	OP_HEARTBEAT             = 1
	OP_IDENTIFY              = 2
	OP_STATUS_UPDATE         = 3
	OP_VOICE_STATE_UPDATE    = 4
	OP_RESUME                = 6
	OP_RECONNECT             = 7
	OP_REQUEST_GUILD_MEMBERS = 8
	OP_INVALID_SESSION       = 9
	OP_HELLO                 = 10
	OP_HEARTBEAT_ACK         = 11
)

const FailedHeartbeatAcks = 5 * time.Millisecond
