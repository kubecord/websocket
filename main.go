package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func main() {
	token := os.Getenv("TOKEN")
	client := Shard{Token: token}
	GatewayData, err := client.GatewayBot()
	if err != nil {
		log.Fatal("Unable to get GatewayBot data")
	}
	if GatewayData.Shards < 1 {
		log.Fatal("Failed to get recommended shard count from Discord")
	}
	log.Printf("Launching %d shards...", GatewayData.Shards)
	var shards []Shard
	for i := 0; i < GatewayData.Shards; i++ {
		shard := NewShard(GatewayData.URL, token, GatewayData.Shards, i)
		err = shard.Open()
		if err != nil {
			log.Fatal("Unable to connect to Discord: ", err)
		}
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
