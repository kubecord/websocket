package main

import (
	"encoding/json"
	"fmt"
	"github.com/go-redis/redis"
	"github.com/imdario/mergo"
	"os"
)

type RedisCache struct {
	client *redis.Client
}

func difference(a, b []Emoji) []Emoji {
	mb := map[string]bool{}
	for _, x := range b {
		mb[x.Id] = true
	}
	var ab []Emoji
	for _, x := range a {
		if _, ok := mb[x.Id]; !ok {
			ab = append(ab, x)
		}
	}
	return ab
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
	for _, role := range data.Roles {
		err = c.PutRole(GuildID, role.Id, role)
		if err != nil {
			return
		}
	}
	for _, channel := range data.Channels {
		err = c.PutChannel(GuildID, channel.Id, channel)
		if err != nil {
			return
		}
	}
	for _, member := range data.Members {
		err = c.PutMember(GuildID, member.User.Id, member)
		if err != nil {
			return
		}
	}
	for _, emoji := range data.Emojis {
		err = c.PutEmoji(emoji)
		if err != nil {
			return
		}
	}

	data.Roles, data.Channels, data.Members, data.Presences = nil, nil, nil, nil
	output, err := json.Marshal(data)
	if err != nil {
		return
	}
	_, err = c.client.HSet("cache:guild", GuildID, string(output)).Result()
	return
}

func (c *RedisCache) UpdateGuild(GuildID string, data Guild) (resp Guild, err error) {
	redisData, err := c.client.HGet("cache:guild", GuildID).Result()
	var oldGuild Guild
	err = json.Unmarshal([]byte(redisData), &oldGuild)
	if err != nil {
		return
	}

	resp = oldGuild

	// Check if emoji's changed and record them in the cache

	// Check for added first
	added := difference(oldGuild.Emojis, data.Emojis)
	for _, emoji := range added {
		_ = c.PutEmoji(emoji)
	}

	// Check for removed emojis
	removed := difference(data.Emojis, oldGuild.Emojis)
	for _, emoji := range removed {
		_ = c.DeleteEmoji(emoji.Id)
	}

	err = mergo.Merge(&oldGuild, data)
	if err != nil {
		return
	}
	output, err := json.Marshal(oldGuild)
	if err != nil {
		return
	}
	_, err = c.client.HSet("cache:guild", GuildID, string(output)).Result()
	return
}

func (c *RedisCache) DeleteGuild(GuildID string) (err error) {
	result, err := c.client.HGet("cache:guild", GuildID).Result()
	var oldGuild Guild
	err = json.Unmarshal([]byte(result), &oldGuild)
	if err != nil {
		return
	}
	for _, emoji := range oldGuild.Emojis {
		_ = c.DeleteEmoji(emoji.Id)
	}
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

func (c *RedisCache) UpdateChannel(GuildID string, ChannelID string, data Channel) (resp Channel, err error) {
	redisData, err := c.client.HGet(fmt.Sprintf("cache:channel:%s", GuildID), ChannelID).Result()
	var oldChannel Channel
	err = json.Unmarshal([]byte(redisData), &oldChannel)
	if err != nil {
		return
	}

	resp = oldChannel

	err = mergo.Merge(&oldChannel, data)
	if err != nil {
		return
	}
	output, err := json.Marshal(oldChannel)
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

func (c *RedisCache) PutRole(GuildID string, RoleID string, data Role) (err error) {
	output, err := json.Marshal(data)
	if err != nil {
		return
	}
	_, err = c.client.HSet(fmt.Sprintf("cache:role:%s", GuildID), RoleID, output).Result()
	return
}

func (c *RedisCache) DeleteRole(GuildID string, RoleID string) (err error) {
	_, err = c.client.HDel(fmt.Sprintf("cache:role:%s", GuildID), RoleID).Result()
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

func (c *RedisCache) PutEmoji(data Emoji) (err error) {
	output, err := json.Marshal(data)
	if err != nil {
		return
	}
	_, err = c.client.HSet("cache:emoji", data.Id, output).Result()
	return
}

func (c *RedisCache) DeleteEmoji(EmojiId string) (err error) {
	_, err = c.client.HDel("cache:emoji", EmojiId).Result()
	return
}
