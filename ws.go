package main

import (
	"github.com/go-redis/redis"
	"github.com/gorilla/websocket"
	"github.com/nats-io/go-nats"
	"github.com/nats-io/go-nats-streaming"
)

type Shard struct {
	Token      string
	SessionID  string
	Sequence   int64
	Conn       *websocket.Conn
	ShardId    uint16
	ShardCount uint16
	Cache      *redis.Client
	NC         *nats.Conn
	SC         *stan.Conn
}

func (s *Shard) onPayload(messageType int, message []byte) (*Event, error) {

}

func (s *Shard) run() {
	s.Cache = redis.NewClient()
}
