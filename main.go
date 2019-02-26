package main

import (
	"fmt"
	"github.com/labstack/gommon/log"
	"os"
	"os/signal"
	"syscall"
)

func main() {
	log.SetLevel(log.INFO)
	token := os.Getenv("TOKEN")
	client := Shard{Token: token}
	GatewayData, err := client.GatewayBot()
	if err != nil {
		log.Fatal("Unable to get GatewayBot data")
	}
	shards := make([]Shard, GatewayData.Shards)
	initSequence := int64(0)
	for sid, shard := range shards {
		shard.Sequence = &initSequence
		shard.SessionID = ""
		shard.Token = token
		shard.ShardCount = GatewayData.Shards
		shard.ShardId = sid
		_ = shard.Open(GatewayData.URL)
	}

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	_ = client.Close()
}
