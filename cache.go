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

func (c *RedisCache) GetGuild(GuildID string) (guild Guild, err error) {
	redisData, err := c.client.HGet("cache:guild", GuildID).Result()
	err = json.Unmarshal([]byte(redisData), &guild)
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
	// Because discord doesn't send member count on updates
	data.MemberCount = oldGuild.MemberCount

	data.Roles, data.Channels, data.Members, data.Presences = nil, nil, nil, nil
	output, err := json.Marshal(data)
	if err != nil {
		return
	}
	_, err = c.client.HSet("cache:guild", GuildID, string(output)).Result()
	return
}

func (c *RedisCache) DeleteGuild(GuildID string) (resp Guild, err error) {
	result, err := c.client.HGet("cache:guild", GuildID).Result()
	var oldGuild Guild
	err = json.Unmarshal([]byte(result), &oldGuild)
	if err != nil {
		return
	}
	resp = oldGuild
	for _, emoji := range oldGuild.Emojis {
		_, _ = c.DeleteEmoji(emoji.Id)
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

	output, err := json.Marshal(data)
	if err != nil {
		return
	}
	_, err = c.client.HSet(fmt.Sprintf("cache:channel:%s", GuildID), ChannelID, string(output)).Result()
	return
}

func (c *RedisCache) DeleteChannel(GuildID string, ChannelID string) (resp Channel, err error) {
	redisData, err := c.client.HGet(fmt.Sprintf("cache:channel:%s", GuildID), ChannelID).Result()
	var oldChannel Channel
	err = json.Unmarshal([]byte(redisData), &oldChannel)
	if err != nil {
		return
	}

	resp = oldChannel
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

func (c *RedisCache) UpdateMember(GuildID, MemberID string, data GuildMember) (resp GuildMember, err error) {
	redisData, err := c.client.HGet(fmt.Sprintf("cache:member:%s", GuildID), MemberID).Result()
	var oldMember GuildMember
	err = json.Unmarshal([]byte(redisData), &oldMember)
	if err != nil {
		return
	}

	resp = oldMember
	data.JoinedAt = oldMember.JoinedAt
	output, err := json.Marshal(data)
	if err != nil {
		return
	}

	_, err = c.client.HSet(fmt.Sprintf("cache:member:%s", GuildID), MemberID, string(output)).Result()
	return
}

func (c *RedisCache) DeleteMember(GuildID string, MemberID string) (resp GuildMember, err error) {
	redisData, err := c.client.HGet(fmt.Sprintf("cache:member:%s", GuildID), MemberID).Result()
	var oldMember GuildMember
	err = json.Unmarshal([]byte(redisData), &oldMember)
	if err != nil {
		return
	}

	resp = oldMember
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

func (c *RedisCache) UpdateRole(GuildID string, RoleID string, data Role) (resp Role, err error) {
	redisData, err := c.client.HGet(fmt.Sprintf("cache:role:%s", GuildID), RoleID).Result()
	var oldRole Role
	err = json.Unmarshal([]byte(redisData), &oldRole)
	if err != nil {
		return
	}

	resp = oldRole

	output, err := json.Marshal(data)
	if err != nil {
		return
	}

	_, err = c.client.HSet(fmt.Sprintf("cache:role:%s", GuildID), RoleID, string(output)).Result()
	return
}

func (c *RedisCache) DeleteRole(GuildID string, RoleID string) (resp Role, err error) {
	redisData, err := c.client.HGet(fmt.Sprintf("cache:role:%s", GuildID), RoleID).Result()
	var oldRole Role
	err = json.Unmarshal([]byte(redisData), &oldRole)
	if err != nil {
		return
	}

	resp = oldRole

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

func (c *RedisCache) UpdateUser(UserID string, data User) (resp User, err error) {
	redisData, err := c.client.HGet("cache:user", UserID).Result()
	var oldUser User
	err = json.Unmarshal([]byte(redisData), &oldUser)
	if err != nil {
		return
	}

	resp = oldUser

	err = mergo.Merge(&oldUser, data)
	if err != nil {
		return
	}
	output, err := json.Marshal(oldUser)
	if err != nil {
		return
	}

	_, err = c.client.HSet("cache:user", UserID, string(output)).Result()
	return
}

func (c *RedisCache) DeleteUser(UserID string) (resp User, err error) {
	redisData, err := c.client.HGet("cache:user", UserID).Result()
	var oldUser User
	err = json.Unmarshal([]byte(redisData), &oldUser)
	if err != nil {
		return
	}

	resp = oldUser

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

func (c *RedisCache) UpdateEmoji(EmojiID string, data Emoji) (resp Emoji, err error) {
	redisData, err := c.client.HGet("cache:emoji", EmojiID).Result()
	var oldEmoji Emoji
	err = json.Unmarshal([]byte(redisData), &oldEmoji)
	if err != nil {
		return
	}

	resp = oldEmoji

	output, err := json.Marshal(data)
	if err != nil {
		return
	}

	_, err = c.client.HSet("cache:emoji", EmojiID, string(output)).Result()
	return
}

func (c *RedisCache) DeleteEmoji(EmojiID string) (resp Emoji, err error) {
	redisData, err := c.client.HGet("cache:emoji", EmojiID).Result()
	var oldEmoji Emoji
	err = json.Unmarshal([]byte(redisData), &oldEmoji)
	if err != nil {
		return
	}

	resp = oldEmoji

	_, err = c.client.HDel("cache:emoji", EmojiID).Result()
	return
}
