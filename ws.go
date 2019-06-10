package main

import (
	"bytes"
	"compress/zlib"
	"encoding/json"
	"fmt"
	"github.com/gorilla/websocket"
	"github.com/nats-io/go-nats"
	"io"
	"log"
	"net/http"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

type Shard struct {
	sync.RWMutex

	Token             string
	SessionID         string
	Sequence          *int64
	Conn              *websocket.Conn
	ShardId           int
	ShardCount        int
	Cache             *RedisCache
	NC                *nats.Conn
	wsLock            sync.Mutex
	listening         chan interface{}
	LastHeartbeatAck  time.Time
	LastHeartbeatSent time.Time
	Gateway           string
	Metrics           *MetricsEngine
}

func NewShard(gateway string, token string, shardCount int, shardID int, metrics *MetricsEngine) (shard Shard) {
	initSequence := int64(0)
	cache, err := NewCache()
	if err != nil {
		log.Fatal("Could not connect to Redis: ", err)
	}

	shard = Shard{
		Gateway:    gateway + "?v=6&encoding=json",
		Sequence:   &initSequence,
		SessionID:  "",
		Token:      token,
		ShardCount: shardCount,
		ShardId:    shardID,
		Cache:      &cache,
		Metrics:    metrics,
	}
	nc, err := nats.Connect(nats.DefaultURL)
	if err != nil {
		log.Fatal(err)
	}
	shard.NC = nc
	return
}

func (s *Shard) Open() error {
	var err error

	s.Lock()
	defer s.Unlock()

	if s.Conn != nil {
		return ErrWSAlreadyOpen
	}

	log.Printf("Shard %d connecting to gateway", s.ShardId)
	header := http.Header{}
	header.Add("accept-encoding", "zlib")

	s.Conn, _, err = websocket.DefaultDialer.Dial(s.Gateway, header)
	if err != nil {
		log.Printf("error connecting to gateway on shard %d", err)
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

	messagetype, message, err := s.Conn.ReadMessage()
	if err != nil {
		return err
	}
	e, err := s.dispatch(messagetype, message)
	if err != nil {
		return err
	}

	if e.Op != OP_HELLO {
		log.Printf("expecting Op 10, got Op %d", e.Op)
		return err
	}

	s.LastHeartbeatAck = time.Now().UTC()

	var h Hello
	if err = json.Unmarshal(e.Data, &h); err != nil {
		return fmt.Errorf("error unmarshalling hello: %s", err)
	}

	sequence := atomic.LoadInt64(s.Sequence)
	if s.SessionID == "" && sequence == 0 {
		err = s.identify()
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
	go s.heartbeat(s.Conn, s.listening, h.Interval)
	go s.onPayload(s.Conn, s.listening)
	return nil
}

func (s *Shard) heartbeat(Conn *websocket.Conn, listening <-chan interface{}, heartbeatInt time.Duration) {
	if listening == nil || Conn == nil {
		return
	}

	var err error
	ticker := time.NewTicker(heartbeatInt * time.Millisecond)
	defer ticker.Stop()

	for {
		s.RLock()
		last := s.LastHeartbeatAck
		s.RUnlock()
		sequence := atomic.LoadInt64(s.Sequence)
		s.wsLock.Lock()
		s.LastHeartbeatSent = time.Now().UTC()
		err = Conn.WriteJSON(HeartBeatOp{1, sequence})
		s.wsLock.Unlock()
		if err != nil || time.Now().UTC().Sub(last) > (heartbeatInt*FailedHeartbeatAcks) {
			if err != nil {
				log.Printf("error sending heartbeat on shard %d, %s", s.ShardId, err)
			} else {
				log.Printf("haven't gotten a heartbeat ACK in %v, triggering a reconnection", time.Now().UTC().Sub(last))
			}
			_ = s.Close()
			s.reconnect()
			return
		}

		select {
		case <-ticker.C:
			// continue loop and send heartbeat
		case <-listening:
			return
		}
	}
}

func (s *Shard) onPayload(wsConn *websocket.Conn, listening <-chan interface{}) {
	for {
		messageType, message, err := wsConn.ReadMessage()

		if err != nil {
			s.RLock()
			sameConnection := s.Conn == wsConn
			s.RUnlock()

			if sameConnection {
				log.Printf("error reading from gateway on shard %d: %s", s.ShardId, err)
				err := s.Close()
				if err != nil {
					log.Printf("error closing connection, %s", err)
				}
				log.Println("calling reconnect()")
				s.reconnect()
			}
			return
		}

		select {
		case <-listening:
			return
		default:
			start := time.Now()
			_, err := s.dispatch(messageType, message)
			if err != nil {
				log.Printf("Error dispatching message: %s", err)
			}
			elapsed := time.Since(start)
			s.Metrics.processingTime.Observe(elapsed.Seconds())
			s.Metrics.events.Inc()
		}
	}
}

func (s *Shard) forwardEvent(e *GatewayPayload) (err error) {
	natsSubject := strings.ToLower(e.Event)
	var natsData []byte
	switch e.Event {
	case "READY":
		log.Printf("Shard %d has successfully connected to the gateway", s.ShardId)
		natsData, err = e.Data.MarshalJSON()
		if err != nil {
			return
		}
		break
	case "RESUMED":
		log.Printf("Shard %d has successfully resumed.", s.ShardId)
		natsData, err = e.Data.MarshalJSON()
		if err != nil {
			return
		}
		break
	case "GUILD_CREATE":
		log.Println("Adding guild to the cache")
		var guild Guild
		if err = json.Unmarshal(e.Data, &guild); err != nil {
			log.Printf("error unmarshalling %s event: %s", e.Event, err)
		}
		err = s.Cache.PutGuild(guild.Id, guild)
		natsData, err = e.Data.MarshalJSON()
		if err != nil {
			return
		}
		_ = s.RequestGuildMembers(guild.Id, "", 1000)
		break
	case "GUILD_UPDATE":
		log.Println("Updating guild data in cache")
		var guild Guild
		if err = json.Unmarshal(e.Data, &guild); err != nil {
			log.Printf("error unmarshalling %s event: %s", e.Event, err)
		}
		guild, err = s.Cache.UpdateGuild(guild.Id, guild)
		if err != nil {
			return
		}
		natsData, err = json.Marshal(guild)
		if err != nil {
			return
		}
		break
	case "GUILD_DELETE":
		log.Println("Removing guild from cache")
		var guild Guild
		if err = json.Unmarshal(e.Data, &guild); err != nil {
			log.Printf("error unmarshalling %s event: %s", e.Event, err)
		}
		guild, err = s.Cache.DeleteGuild(guild.Id)
		if err != nil {
			return
		}
		natsData, err = json.Marshal(guild)
		if err != nil {
			return
		}
		break
	case "CHANNEL_CREATE":
		log.Println("Adding channel to the cache")
		var channel Channel
		if err = json.Unmarshal(e.Data, &channel); err != nil {
			log.Printf("error unmarshalling %s event: %s", e.Event, err)
			return
		}
		err = s.Cache.PutChannel(channel.GuildId, channel.Id, channel)
		natsData, err = json.Marshal(e.Data)
		if err != nil {
			return
		}
		break
	case "CHANNEL_UPDATE":
		log.Println("Updating channel in cache")
		var channel Channel
		if err = json.Unmarshal(e.Data, &channel); err != nil {
			log.Printf("error unmarshalling %s event: %s", e.Event, err)
			return
		}
		channel, err = s.Cache.UpdateChannel(channel.GuildId, channel.Id, channel)
		if err != nil {
			return
		}
		natsData, err = json.Marshal(channel)
		if err != nil {
			return
		}
		break
	case "CHANNEL_DELETE":
		log.Println("Removing channel from cache")
		var channel Channel
		if err = json.Unmarshal(e.Data, &channel); err != nil {
			log.Printf("error unmarshalling %s event: %s", e.Event, err)
			return
		}
		channel, err = s.Cache.DeleteChannel(channel.GuildId, channel.Id)
		if err != nil {
			return
		}
		natsData, err = json.Marshal(channel)
		if err != nil {
			return
		}
		break
	case "GUILD_MEMBER_ADD":
		log.Println("Adding guild member to the cache")
		var member GuildMember
		if err = json.Unmarshal(e.Data, &member); err != nil {
			log.Printf("error unmarshalling %s event: %s", e.Event, err)
			return
		}
		err = s.Cache.PutMember(member.GuildID, member.User.Id, member)
		if err != nil {
			return
		}
		natsData, err = json.Marshal(e.Event)
		if err != nil {
			return
		}
		break
	case "GUILD_MEMBER_UPDATE":
		log.Println("Updating member in cache")
		var member GuildMember
		if err = json.Unmarshal(e.Data, &member); err != nil {
			log.Printf("error unmarshalling %s event: %s", e.Event, err)
			return
		}
		member, err = s.Cache.UpdateMember(member.GuildID, member.User.Id, member)
		if err != nil {
			return
		}
		natsData, err = json.Marshal(member)
		if err != nil {
			return
		}
		break
	case "GUILD_MEMBER_REMOVE":
		log.Println("Removing member from cache")
		var member GuildMember
		if err = json.Unmarshal(e.Data, &member); err != nil {
			log.Printf("error unmarshalling %s event: %s", e.Event, err)
			return
		}
		member, err = s.Cache.DeleteMember(member.GuildID, member.User.Id)
		if err != nil {
			return
		}
		natsData, err = json.Marshal(member)
		if err != nil {
			return
		}
		break
	case "GUILD_ROLE_CREATE":
		log.Println("Adding role to the cache")
		var role GuildRole
		if err = json.Unmarshal(e.Data, &role); err != nil {
			log.Printf("error unmarshalling %s event: %s", e.Event, err)
			return
		}
		err = s.Cache.PutRole(role.GuildID, role.Role.Id, *role.Role)
		if err != nil {
			return
		}
		natsData, err = json.Marshal(role)
		if err != nil {
			return
		}
		break
	case "GUILD_ROLE_UPDATE":
		log.Println("Updating role in cache")
		var role GuildRole
		if err = json.Unmarshal(e.Data, &role); err != nil {
			log.Printf("error unmarshalling %s event: %s", e.Event, err)
			return
		}
		var tempRole Role
		tempRole, err = s.Cache.UpdateRole(role.GuildID, role.Role.Id, *role.Role)
		if err != nil {
			log.Printf("error saving role to cache: %s", err)
			return
		}
		role.Role = &tempRole
		natsData, err = json.Marshal(role)
		if err != nil {
			return
		}
		break
	case "GUILD_ROLE_DELETE":
		log.Println("Removing role from cache")
		var role GuildRoleDelete
		if err = json.Unmarshal(e.Data, &role); err != nil {
			log.Printf("error unmarshalling %s event: %s", e.Event, err)
			return
		}
		var tempRole Role
		tempRole, err = s.Cache.DeleteRole(role.GuildID, role.RoleID)
		if err != nil {
			return
		}
		var outRole GuildRole
		outRole.GuildID = role.GuildID
		outRole.Role = &tempRole
		natsData, err = json.Marshal(outRole)
		if err != nil {
			return
		}
		break
	case "GUILD_MEMBERS_CHUNK":
		var guildMembersChunk GuildMembersChunk
		if err = json.Unmarshal(e.Data, &guildMembersChunk); err != nil {
			log.Printf("error unmarshalling %s event: %s", e.Event, err)
			return
		}
		log.Printf("Adding %d members to the cache from %s", len(guildMembersChunk.Members), guildMembersChunk.GuildID)
		for _, member := range guildMembersChunk.Members {
			_ = s.Cache.PutMember(guildMembersChunk.GuildID, member.User.Id, member)
		}
		return
	case "GUILD_EMOJIS_UPDATE":
		log.Println("Updating guild roles in cache")
		var emojis GuildEmojisUpdate
		if err = json.Unmarshal(e.Data, &emojis); err != nil {
			log.Printf("error unmarshalling %s event: %s", e.Event, err)
			return
		}
		var oldGuild Guild
		oldGuild, err = s.Cache.GetGuild(emojis.GuildID)
		if err != nil {
			return
		}
		added := difference(emojis.Emojis, oldGuild.Emojis)
		log.Printf("Adding %d emojis", len(added))
		for _, emoji := range added {
			_ = s.Cache.PutEmoji(emoji)
		}
		removed := difference(oldGuild.Emojis, emojis.Emojis)
		log.Printf("Removing %d emojis", len(removed))
		for _, emoji := range removed {
			_, _ = s.Cache.DeleteEmoji(emoji.Id)
		}

		// Check for emoji renames because dumb
		for i := 0; i < len(emojis.Emojis); i++ {
			for j := 0; j < len(oldGuild.Emojis); j++ {
				if (oldGuild.Emojis[j].Id == emojis.Emojis[i].Id) && (oldGuild.Emojis[j].Name != emojis.Emojis[i].Name) {
					err = s.Cache.PutEmoji(emojis.Emojis[i])
					if err != nil {
						return
					}
				}
			}
		}

		oldGuild.Emojis = emojis.Emojis

		err = s.Cache.PutGuild(oldGuild.Id, oldGuild)
		if err != nil {
			return
		}
	default:
		natsData, err = e.Data.MarshalJSON()
		if err != nil {
			return
		}

	}
	err = s.NC.Publish(fmt.Sprintf("discord.event.%s", natsSubject), natsData)
	if err != nil {
		fmt.Printf("failed to publish to NATS: %s", err)
	}
	return
}

func (s *Shard) dispatch(messageType int, message []byte) (*GatewayPayload, error) {
	var buffer io.Reader
	var err error
	var e *GatewayPayload
	buffer = bytes.NewBuffer(message)
	log.Printf("Got event on shard %d", s.ShardId)
	if messageType == websocket.BinaryMessage {
		decompressor, zerr := zlib.NewReader(buffer)
		if zerr != nil {
			log.Printf("error decompressing message: %s", zerr)
			return nil, zerr
		}

		defer func() {
			zerr := decompressor.Close()
			if zerr != nil {
				log.Printf("error closing zlib: %s", zerr)
				return
			}
		}()

		buffer = decompressor
	}

	log.Printf("Decompressed message on shard %d", s.ShardId)

	decoder := json.NewDecoder(buffer)
	if err = decoder.Decode(&e); err != nil {
		log.Printf("error decoding message: %s", err)
		return e, err
	}

	log.Printf("Op: %d, Seq: %d, Type: %s, Data: %s\n\n", e.Op, e.Sequence, e.Event, string(e.Data))

	switch e.Op {
	case OP_HEARTBEAT:
		s.wsLock.Lock()
		err = s.Conn.WriteJSON(HeartBeatOp{OP_HEARTBEAT, atomic.LoadInt64(s.Sequence)})
		s.wsLock.Unlock()
		if err != nil {
			log.Println("error sending heartbeat")
			return e, err
		}

		return e, nil
	case OP_RECONNECT:
		_ = s.Close()
		s.reconnect()
		return e, nil
	case OP_INVALID_SESSION:
		err = s.identify()
		if err != nil {
			log.Printf("error identifying with gateway: %s", err)
			return e, err
		}
		return e, nil
	case OP_HELLO:
		return e, nil
	case OP_HEARTBEAT_ACK:
		s.Lock()
		s.LastHeartbeatAck = time.Now().UTC()
		s.Unlock()
		log.Println("got heartbeat ACK")
		return e, nil
	case OP_DISPATCH:
		// Dispatch the message
		atomic.StoreInt64(s.Sequence, e.Sequence)
		log.Printf("Got event %s on shard %d", e.Event, s.ShardId)
		err = s.forwardEvent(e)
		return e, nil
	default:
		log.Printf("Unknown Op: %d", e.Op)
		return e, nil
	}
}

func (s *Shard) identify() error {
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

func (s *Shard) reconnect() {
	var err error

	wait := time.Duration(1)

	for {
		err = s.Open()
		if err == nil {
			log.Printf("reconnected shard %d to gateway", s.ShardId)
			s.Metrics.reconnects.Inc()
			return
		}

		if err == ErrWSAlreadyOpen {
			return
		}

		log.Printf("error reconnecting to gateway: %s", err)

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
			log.Printf("error closing websocket: %s", err)
		}

		time.Sleep(1 * time.Second)

		err = s.Conn.Close()
		if err != nil {
			log.Printf("error closing websocket: %s", err)
		}

		s.Conn = nil
	}

	s.Unlock()

	return
}

func (s *Shard) RequestGuildMembers(guildID, query string, limit int) (err error) {
	s.RLock()
	defer s.RUnlock()
	if s.Conn == nil {
		return ErrWSNotFound
	}

	data := requestGuildMembersData{
		GuildID: guildID,
		Query:   query,
		Limit:   limit,
	}
	s.wsLock.Lock()
	err = s.Conn.WriteJSON(requestGuildMembersOp{8, data})
	s.wsLock.Unlock()

	return
}
