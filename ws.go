package main

import (
	"bytes"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis"
	"github.com/gorilla/websocket"
	"github.com/labstack/gommon/log"
	"github.com/nats-io/go-nats"
	"github.com/nats-io/go-nats-streaming"
	"io"
	"os"
	"sync"
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
	SC         stan.Conn
	wsLock     sync.Mutex
}

func (s *Shard) onPayload(wsConn *websocket.Conn, listening <-chan interface{}) {
	log.Info("Websocket message")
	for {
		messageType, message, err := wsConn.ReadMessage()

		if err != nil {
			sameConnection := s.Conn == wsConn

			if sameConnection {
				err := s.Close()
				if err != nil {
					log.Warn("error closing connection, %s", err)
				}
				log.Info("calling reconnect()")
				s.reconnect()
			}
			return
		}

		select {
		case <-listening:
			return
		default:
			// Process the event
		}
	}
}

func (s *Shard) Dispatch(messageType int, message []byte) (*GatewayPayload, error) {
	var buffer io.Reader
	var err error
	buffer = bytes.NewBuffer(message)

	if messageType == websocket.BinaryMessage {
		decompressor, zerr := zlib.NewReader(buffer)
		if zerr != nil {
			log.Error("error decompressing message: %s", zerr)
		}

		defer func() {
			zerr := decompressor.Close()
			if zerr != nil {
				log.Warn("error closing zlib: %s", zerr)
			}
		}()

		buffer = decompressor
	}

	var e *GatewayPayload

	decoder := json.NewDecoder(buffer)
	if err = decoder.Decode(&e); err != nil {
		log.Error("error decoding message: %s", err)
		return e, err
	}

	log.Debug("Op: %d, Seq: %d, Type: %s, Data: %s\n\n", e.Op, e.Sequence, e.Event, string(e.Data))

	switch e.Op {
	case OP_HEARTBEAT:
	}

}

func (s *Shard) run() {
	s.Cache = redis.NewClient(&redis.Options{
		Addr: "",
		DB:   0,
	})
	_, err := s.Cache.Ping().Result()
	if err != nil {
		log.Fatal(err)
	}
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatal(err)
	}
	s.NC = nc

	clientId := fmt.Sprintf("%s%d", "shard-", s.ShardId)

	clusterName := os.Getenv("STAN_CLUSTER")

	sc, err := stan.Connect(clusterName, clientId, stan.NatsConn(s.NC))
	if err != nil {
		log.Fatal(err)
	}
	s.SC = sc
}
