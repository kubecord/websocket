package main

import (
	"fmt"
	"github.com/labstack/gommon/log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	log.SetLevel(log.INFO)
	token := os.Getenv("TOKEN")
	client := Shard{Token: token}
	GatewayData, err := client.GatewayBot()
	if err != nil {
		log.Fatal("Unable to get GatewayBot data")
	}
	var shards []Shard
	for i := 0; i < GatewayData.Shards; i++ {
		shard := NewShard(GatewayData.URL, token, GatewayData.Shards, i)
		_ = shard.Open()
		shards = append(shards, shard)
		time.Sleep(5 * time.Second)
	}

	// Wait here until CTRL-C or other term signal is received.
	fmt.Println("Bot is now running.  Press CTRL-C to exit.")
	sc := make(chan os.Signal, 1)
	signal.Notify(sc, syscall.SIGINT, syscall.SIGTERM, os.Interrupt, os.Kill)
	<-sc

	// Cleanly close down the Discord session.
	_ = client.Close()
}
