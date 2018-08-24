package main

import (
	"fmt"
	"github.com/ninech/nine-dhcp2/configuration"
	"github.com/ninech/nine-dhcp2/dhcp"
	"github.com/ninech/nine-dhcp2/netbox"
	"log"
	"os"
	"os/signal"
)

var netboxClient netbox.Client
var stopped chan bool

func main() {
	fmt.Println("nine-dhcp2 v0.0.0")
	fmt.Println("(c) 2018 Nine Internet Solutions")

	configFilename := "nine-dhcp2.conf.yaml"
	config, err := configuration.ReadConfig(configFilename)
	if err != nil {
		log.Fatalln("Unable to load configuration.", configFilename, err)
	}

	netboxClient = netbox.Client{Config: &config.Netbox}

	if !netboxClient.CheckSites() {
		log.Fatalln("The config contains inactive or missing sites. Please check the log.")
	}

	d := dhcp.NewDaemon(&config)
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
