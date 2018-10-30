package dhcp

import (
	"log"
	"net"

	"github.com/cimnine/netbox-dhcp/configuration"
	"github.com/cimnine/netbox-dhcp/resolver"
)

type Daemon struct {
	Configuration *configuration.Configuration
	Resolver      resolver.Resolver

	dhcpv4Servers map[string]*ServerV4
	dhcpv6Servers map[string]*ServerV6
}

func NewDaemon(config *configuration.Configuration, res resolver.Resolver) Daemon {
	d := Daemon{
		Configuration: config,
		Resolver:      res,
		dhcpv4Servers: make(map[string]*ServerV4),
		dhcpv6Servers: make(map[string]*ServerV6),
	}

	d.spawnV4Servers()
	d.spawnV6Servers()

	return d
}

func (d *Daemon) spawnV4Servers() {
	config := d.Configuration
	for ifaceString, ifaceConfig := range config.Daemon.ListenV4 {
		iface, err := net.InterfaceByName(ifaceString)
		if err != nil {
			log.Printf("Can't find iface '%s' because of %s", ifaceString, err)
			continue
		}

		server, err := NewServerV4(&config.DHCP, d.Resolver, *iface, &ifaceConfig)
		if err != nil {
			log.Printf("Can't listen on iface '%s' because of %s", ifaceString, err)
			continue
		}

		d.dhcpv4Servers[ifaceString] = &server
	}
}

func (d *Daemon) spawnV6Servers() {
	config := d.Configuration
	for ifaceString, ifaceConfig := range config.Daemon.ListenV6 {
		iface, err := net.InterfaceByName(ifaceString)
		if err != nil {
			log.Printf("Can't find iface '%s' because of %s", ifaceString, err)
			continue
		}

		server, err := NewServerV6(&config.DHCP, d.Resolver, *iface, &ifaceConfig)
		if err != nil {
			log.Printf("Can't listen on iface '%s' because of %s", ifaceString, err)
			continue
		}

		d.dhcpv6Servers[ifaceString] = &server
	}
}

func (d *Daemon) Shutdown() {
	log.Println("Stopping daemon.")

	for _, dhcpV4Server := range d.dhcpv4Servers {
		dhcpV4Server.Stop()
	}
	for _, dhcpV6Server := range d.dhcpv6Servers {
		dhcpV6Server.Stop()
	}

	log.Println("Stopped daemon.")
}

func (d *Daemon) Start() {
	log.Println("Starting daemon.")

	for _, serverOnInterface := range d.dhcpv4Servers {
		go serverOnInterface.Start()
	}
	for _, serverOnInterface := range d.dhcpv6Servers {
		go serverOnInterface.Start()
	}

	log.Println("Started daemon.")
}
