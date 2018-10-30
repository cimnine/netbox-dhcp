package dhcp

import (
	"github.com/ninech/nine-dhcp2/configuration"
	"github.com/ninech/nine-dhcp2/resolver"
	"log"
	"net"
)

type Daemon struct {
	Configuration *configuration.Configuration

	dhcpv4Servers map[string]*ServerV4
	//dhcpv6Servers map[string]*ServerV6
}

func NewDaemon(config *configuration.Configuration, res resolver.Resolver) Daemon {
	d := Daemon{
		Configuration: config,
		dhcpv4Servers: make(map[string]*ServerV4),
	}

	for ifaceString, ifaceConfig := range config.Daemon.ListenV4 {
		iface, err := net.InterfaceByName(ifaceString)
		if err != nil {
			log.Printf("Can't find iface '%s' because of %s", ifaceString, err)
		}

		server, err := NewServerV4(&config.DHCP, res, *iface, &ifaceConfig)
		if err != nil {
			log.Printf("Can't listen on iface '%s' because of %s", ifaceString, err)
			continue
		}

		d.dhcpv4Servers[ifaceString] = &server
	}

	return d
}

func (d *Daemon) Shutdown() {
	log.Println("Stopping daemon.")

	for _, dhcpV4Server := range d.dhcpv4Servers {
		dhcpV4Server.Stop()
	}

	log.Println("Stopped daemon.")
}

func (d *Daemon) Start() {
	log.Println("Starting daemon.")

	for _, serverOnInterface := range d.dhcpv4Servers {
		go serverOnInterface.Start()
	}

	log.Println("Started daemon.")
}
