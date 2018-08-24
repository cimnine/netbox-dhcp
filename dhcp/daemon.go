package dhcp

import (
	"github.com/ninech/nine-dhcp2/configuration"
	"github.com/ninech/nine-dhcp2/dhcp/dhcpv4"
	"log"
)

type Daemon struct {
	Configuration *configuration.Configuration

	dhcpv4Server map[string][]dhcpv4.ServerV4
	//dhcpv6Server map[string][]dhcpv6.ServerV6
}

func NewDaemon(config *configuration.Configuration) Daemon {
	d := Daemon{
		Configuration: config,
		dhcpv4Server:  make(map[string][]dhcpv4.ServerV4),
	}

	for iface, addresses := range config.Daemon.ListenV4 {
		for _, addr := range addresses {
			server, err := dhcpv4.NewServer(&config.DHCP, iface, addr)
			if err != nil {
				log.Printf("Can't listen on iface '%s' on addr '%s' because of %s\n", iface, addr, err)
				continue
			}

			d.dhcpv4Server[iface] = append(d.dhcpv4Server[iface], server)
		}
	}

	return d
}

func (d *Daemon) Shutdown() {
	// TODO implement graceful shutdown
}

func (d *Daemon) Start() {
	for _, addrs := range d.dhcpv4Server {
		for _, addr := range addrs {
			go addr.Start()
		}
	}

	log.Println("Running daemon.")
}
