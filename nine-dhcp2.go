package main

import (
	"fmt"
	"github.com/ninech/nine-dhcp2/configuration"
	"log"
)

func main() {
	fmt.Println("nine-dhcp2 v0.0.0")
	fmt.Println("(c) 2018 Nine Internet Solutions")

	configFilename := "nine-dhcp2.conf.yaml"
	c, err := configuration.ReadConfig(configFilename)
	if err != nil {
		log.Fatalln("Unable to load configuration.", configFilename, err)
	}

	fmt.Printf("%v\n", c)
}
