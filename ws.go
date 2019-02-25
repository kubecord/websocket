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
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"
)

type Shard struct {
	sync.RWMutex

	Token            string
	SessionID        string
	Sequence         *int64
	Conn             *websocket.Conn
	ShardId          int
	ShardCount       int
	Cache            *redis.Client
	NC               *nats.Conn
	SC               stan.Conn
	wsLock           sync.Mutex
	listening        chan interface{}
	LastHeartbeatAck time.Time
	Gateway          string
}

func (s *Shard) Open(gateway string) error {
	var err error

	s.Lock()
	defer s.Unlock()

	if s.Conn != nil {
		return ErrWSAlreadyOpen
	}
	s.Gateway = gateway
	gateway = gateway + "?v=6&encoding=json&compress=zlib-stream"

	log.Info("Shard %d connecting to gateway", s.ShardId)
	header := http.Header{}
	header.Add("accept-encoding", "zlib")

	s.Conn, _, err = websocket.DefaultDialer.Dial(gateway, header)
	if err != nil {
		log.Warn("error connecting to gateway on shard %d", err)
		s.Conn = nil
		return err
	}

	s.Conn.SetCloseHandler(func(code int, text string) error {
		return nil
	})

	defer func() {
		// because of this, all code below must set err to the error
		// when exiting with an error :)  Maybe someone has a better
		// way :)
		if err != nil {
			_ = s.Conn.Close()
			s.Conn = nil
		}
	}()

	messagetype, message, err := s.wsConn.ReadMessage()
	if err != nil {
		return err
	}
	e, err := s.Dispatch(messagetype, message)
	if err != nil {
		return err
	}

	if e.Op != OP_HELLO {
		log.Error("expecting Op 10, got Op %d", e.Op)
		return err
	}

	s.LastHeartbeatAck = time.Now().UTC()

	var h Hello
	if err = json.Unmarshal(e.Data, &h); err != nil {
		return fmt.Errorf("error unmarshalling hello: %s", err)
	}

	sequence := atomic.LoadInt64(s.Sequence)
	if s.SessionID == "" && sequence == 0 {
		err = s.Identify()
		if err != nil {
			return fmt.Errorf("error sending identify package on shard %d: %s", s.ShardId, err)
		}
	} else {
		data := ResumeData{
			Token:     s.Token,
			SessionID: s.SessionID,
			Sequence:  sequence,
		}

		p := ResumePayload{
			Op:   OP_RESUME,
			Data: data,
		}

		s.wsLock.Lock()
		err = s.Conn.WriteJSON(p)
		s.wsLock.Unlock()
		if err != nil {
			return fmt.Errorf("error sending resume packet for shard %d: %s", s.ShardId, err)
		}
	}

	s.listening = make(chan interface{})
	// TODO: Set up heartbeat logic
	go s.onPayload(s.Conn, s.listening)
	return nil
}

func (s *Shard) onPayload(wsConn *websocket.Conn, listening <-chan interface{}) {
	log.Info("Websocket message")
	for {
		messageType, message, err := wsConn.ReadMessage()

		if err != nil {
			s.RLock()
			sameConnection := s.Conn == wsConn
			s.RUnlock()

			if sameConnection {
				err := s.Close()
				if err != nil {
					log.Warn("error closing connection, %s", err)
				}
				log.Info("calling reconnect()")
				s.Reconnect()
			}
			return
		}

		select {
		case <-listening:
			return
		default:
			_, err := s.Dispatch(messageType, message)
			if err != nil {
				log.Error("Error dispatching message: %s", err)
			}
		}
	}
}

func (s *Shard) Dispatch(messageType int, message []byte) (*GatewayPayload, error) {
	var buffer io.Reader
	var err error
	var e *GatewayPayload
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

	decoder := json.NewDecoder(buffer)
	if err = decoder.Decode(&e); err != nil {
		log.Error("error decoding message: %s", err)
		return e, err
	}

	log.Debug("Op: %d, Seq: %d, Type: %s, Data: %s\n\n", e.Op, e.Sequence, e.Event, string(e.Data))

	switch e.Op {
	case OP_HEARTBEAT:
		s.wsLock.Lock()
		err = s.Conn.WriteJSON(HeartBeatOp{OP_HEARTBEAT, atomic.LoadInt64(s.Sequence)})
		s.wsLock.Unlock()
		if err != nil {
			log.Error("error sending heartbeat")
			return e, err
		}

		return e, nil
	case OP_RECONNECT:
		_ = s.Close()
		s.Reconnect()
		return e, nil
	case OP_INVALID_SESSION:
		err = s.Identify()
		if err != nil {
			log.Error("error identifying with gateway: %s", err)
			return e, err
		}
	case OP_HELLO:
		return e, nil
	case OP_HEARTBEAT_ACK:
		s.Lock()
		s.LastHeartbeatAck = time.Now().UTC()
		s.Unlock()
		log.Debug("got heartbeat ACK")
		return e, nil
	case OP_DISPATCH:
		// Dispatch the message
		atomic.StoreInt64(s.Sequence, e.Sequence)
		return e, nil
	default:
		log.Warn("Unknown Op: %d", e.Op)
		return e, nil
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

func (s *Shard) Identify() error {
	props := IdentifyProperties{
		OS:      "Kubecord v0.0.1",
		Browser: "",
		Device:  "",
	}
	payload := Identify{
		Token:          s.Token,
		Properties:     props,
		Compress:       true,
		LargeThreshold: 250,
		Shard:          &[2]int{s.ShardId, s.ShardCount},
	}
	data := OutgoingPayload{
		Op:   OP_IDENTIFY,
		Data: payload,
	}

	s.wsLock.Lock()
	err := s.Conn.WriteJSON(data)
	s.wsLock.Unlock()

	return err
}

func (s *Shard) Reconnect() {
	var err error

	wait := time.Duration(1)

	for {
		err = s.Open()
		if err == nil {
			log.Info("reconnected shard %d to gateway", s.ShardId)
			return
		}

		if err == ErrWSAlreadyOpen {
			return
		}

		log.Error("error reconnecting to gateway: %s", err)

		<-time.After(wait * time.Second)
		wait *= 2
		if wait > 600 {
			wait = 600
		}
	}

}

func (s *Shard) Close() (err error) {
	s.Lock()
	if s.listening != nil {
		close(s.listening)
		s.listening = nil
	}

	if s.Conn != nil {
		s.wsLock.Lock()
		err := s.Conn.WriteMessage(websocket.CloseMessage, websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""))
		s.wsLock.Unlock()
		if err != nil {
			log.Error("error closing websocket: %s", err)
		}

		time.Sleep(1 * time.Second)

		err = s.Conn.Close()
		if err != nil {
			log.Error("error closing websocket: %s", err)
		}

		s.Conn = nil
	}

	s.Unlock()

	return
}
