package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"

	"github.com/go-redis/redis"

	redisCache "github.com/cimnine/netbox-dhcp/cache/redis"
	"github.com/cimnine/netbox-dhcp/configuration"
	"github.com/cimnine/netbox-dhcp/dhcp"
	"github.com/cimnine/netbox-dhcp/netbox"
	"github.com/cimnine/netbox-dhcp/resolver"
)

var netboxClient netbox.Client
var redisClient redis.Client
var stopped chan bool

func main() {
	fmt.Println("netbox-dhcp v0.0.0")

	var configFileName string
	flag.StringVar(&configFileName, "config", "/etc/netbox-dhcp.conf.yaml", "where to load the config from")
	flag.Parse()

	config, err := configuration.ReadConfig(configFileName)
	if err != nil {
		log.Fatalln("Unable to load configuration.", configFileName, err)
	} else {
		log.Printf("Config loaded successfully from '%s'.", configFileName)
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
