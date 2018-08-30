package main

import (
	"fmt"
	"github.com/go-redis/redis"

	redisCache "github.com/ninech/nine-dhcp2/cache/redis"
	"github.com/ninech/nine-dhcp2/configuration"
	"github.com/ninech/nine-dhcp2/dhcp"
	"github.com/ninech/nine-dhcp2/netbox"
	"github.com/ninech/nine-dhcp2/resolver"
	"log"
	"os"
	"os/signal"
)

var netboxClient netbox.Client
var redisClient redis.Client
var stopped chan bool

func main() {
	fmt.Println("nine-dhcp2 v0.0.0")
	fmt.Println("(c) 2018 Nine Internet Solutions")

	configFilename := "nine-dhcp2.conf.yaml"
	config, err := configuration.ReadConfig(configFilename)
	if err != nil {
		log.Fatalln("Unable to load configuration.", configFilename, err)
	}

	redisClient = *redisCache.NewClient(&config.Cache.Redis)
	netboxClient = netbox.Client{Config: &config.Netbox}

	if !netboxClient.CheckSites() {
		log.Fatalln("The config contains inactive or missing sites. Please check the log.")
	}

	netboxOfferer := resolver.Netbox{Client: &netboxClient}
	redisCachingRequester := resolver.Redis{Client: &redisClient}

	requester := resolver.CachingResolver{Source: netboxOfferer, Cache: redisCachingRequester}

	d := dhcp.NewDaemon(&config, requester)
	setupShutdownHandler(d.Shutdown)

	d.Start()

	<-stopped
}

func setupShutdownHandler(shutdown func()) {
	stopped = make(chan bool)

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt)
	go func() {
		<-c
		log.Println("Shutting down.")

		shutdown()

		log.Println("Bye 👋")
		stopped <- true
	}()

	log.Println("Quit with CTRL+C.")
}
