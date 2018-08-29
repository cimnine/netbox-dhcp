package dhcp

import (
	"github.com/ninech/nine-dhcp2/configuration"
	"github.com/ninech/nine-dhcp2/dhcp/dhcpv4"
	"github.com/ninech/nine-dhcp2/resolver"
	"log"
)

type Daemon struct {
	Configuration *configuration.Configuration

	dhcpv4Servers map[string]*dhcpv4.ServerV4
	//dhcpv6Servers map[string]*dhcpv6.ServerV6
}

func NewDaemon(config *configuration.Configuration, res resolver.Resolver) Daemon {
	d := Daemon{
		Configuration: config,
		dhcpv4Servers: make(map[string]*dhcpv4.ServerV4),
	}

	for addr, ifaceConfig := range config.Daemon.ListenV4 {
		server, err := dhcpv4.NewServer(&config.DHCP, res, addr, &ifaceConfig)
		if err != nil {
			log.Printf("Can't listen on addr '%s' because of %s\n", addr, err)
			continue
		}

		d.dhcpv4Servers[addr] = &server
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
