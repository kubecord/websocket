package main

import "github.com/gorilla/websocket"

type Shard struct {
	Token string
	SessionID string
	Sequence int64
	Conn *websocket.Conn
	ShardId uint16
	ShardCount uint16
}

func (s *Shard) onMessage(messageType int, message []byte) (*Event, error) {

}