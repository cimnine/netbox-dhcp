package main

import (
	"flag"
	"fmt"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"log"
	"net"
)

var server, iface string
var port uint
var family uint

func main() {
	fmt.Println("dhcpdebug v0.0.0")
	fmt.Println("(c) 2018 Nine Internet Solutions")

	flag.StringVar(&server, "server", "255.255.255.255", "The destination server.")
	flag.StringVar(&iface, "iface", "eth0", "The interface to use.")
	flag.UintVar(&port, "port", 67, "The destination port.")
	flag.UintVar(&family, "ip", 4, "The IP family.")
	flag.Parse()

	log.Printf("Connecting to '%s' on port '%d' via '%s' with IPv%d...\n", server, port, iface, family)

	if family == 4 {
		clientV4()
	} else {
		clientV6()
	}
}

func clientV4() {
	pkg, err := dhcpv4.NewDiscoveryForInterface(iface)
	if err != nil {
		log.Fatalln("Can't build DHCPDISCOVER pkg", err)
	}

	if server == "broadcast" || server == "255.255.255.255" {
		useDhcpClient(pkg)
	} else {
		udpDstAddr, err := net.ResolveUDPAddr("udp4", fmt.Sprintf("%s:%d", server, port))
		if err != nil {
			log.Fatalln("Can't resolve server.", err)
		}

		pkg.SetUnicast()
		pkg.SetServerIPAddr(udpDstAddr.IP)

		udpSrcAddr, err := net.ResolveUDPAddr("udp4", ":68")
		if err != nil {
			log.Fatalln(err)
		}

		conn, err := net.ListenUDP("udp4", udpSrcAddr)
		if err != nil {
			log.Fatalln(err)
		}

		log.Printf("Sending DHCPDISCOVER: %v", pkg)
		written, err := conn.WriteToUDP(pkg.ToBytes(), udpDstAddr)
		if err != nil {
			log.Fatalln(err)
		} else {
			log.Printf("%d bytes sent.", written)
		}
	}
}

func useDhcpClient(pkg *dhcpv4.DHCPv4) {
	client := dhcpv4.NewClient()
	conversation, err := client.Exchange(iface, pkg)
	for i, message := range conversation {
		fmt.Printf("Here %d\n", i)
		log.Print(message.Summary())
	}
	if err != nil {
		log.Fatalln(err)
	}
}

func clientV6() {
	log.Fatalln("Not yet implemented.")
}
