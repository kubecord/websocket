package main

import (
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis"
	"os"
)

type RedisCache struct {
	client *redis.Client
}

func NewCache() (cache RedisCache, err error) {
	client := redis.NewClient(&redis.Options{
		Addr: os.Getenv("REDIS_ADDR"),
		DB:   0,
	})
	_, err = client.Ping().Result()
	cache = RedisCache{
		client: client,
	}
	return
}

func (c *RedisCache) Clear() (err error) {
	_, err = c.client.FlushDB().Result()
	return
}

func (c *RedisCache) PutGuild(GuildID string, data Guild) (err error) {
	data.Roles, data.Channels, data.Members = nil, nil, nil
	output, err := json.Marshal(data)
	if err != nil {
		return
	}
	_, err = c.client.HSet("cache:guild", GuildID, string(output)).Result()
	return
}

func (c *RedisCache) DeleteGuild(GuildID string) (err error) {
	_, err = c.client.HDel("cache:guild", GuildID).Result()
	return
}

func (c *RedisCache) PutChannel(GuildID string, ChannelID string, data Channel) (err error) {
	output, err := json.Marshal(data)
	if err != nil {
		return
	}
	_, err = c.client.HSet(fmt.Sprintf("cache:channel:%s", GuildID), ChannelID, string(output)).Result()
	return
}

func (c *RedisCache) DeleteChannel(GuildID string, ChannelID string) (err error) {
	_, err = c.client.HDel(fmt.Sprintf("cache:channel:%s", GuildID), ChannelID).Result()
	return
}

func (c *RedisCache) PutMember(GuildID string, MemberID string, data GuildMember) (err error) {
	output, err := json.Marshal(data)
	if err != nil {
		return
	}
	_, err = c.client.HSet(fmt.Sprintf("cache:member:%s", GuildID), MemberID, output).Result()
	return
}

func (c *RedisCache) DeleteMember(GuildID string, MemberID string) (err error) {
	_, err = c.client.HDel(fmt.Sprintf("cache:member:%s", GuildID), MemberID).Result()
	return
}

func (c *RedisCache) PutUser(UserID string, data User) (err error) {
	output, err := json.Marshal(data)
	if err != nil {
		return
	}
	_, err = c.client.HSet("cache:user", UserID, output).Result()
	return
}

func (c *RedisCache) DeleteUser(UserID string) (err error) {
	_, err = c.client.HDel("cache:user", UserID).Result()
	return
}

func (c *RedisCache) PutClientUser(data User) (err error) {
	output, err := json.Marshal(data)
	if err != nil {
		return
	}
	_, err = c.client.HSet("cache:user", "@me", output).Result()
	return
}
