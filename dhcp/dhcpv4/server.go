package dhcpv4

import (
	"errors"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/ninech/nine-dhcp2/dhcp/config"
	"log"
	"net"
	"strconv"
)

type ServerV4 struct {
	DHCPConfig *config.DHCPConfig
	conn       *net.UDPConn
	broadcast  bool
	iface      string
	address    string
}

func NewServer(dhcpConfig *config.DHCPConfig, iface, address string) (s ServerV4, err error) {
	s = ServerV4{
		DHCPConfig: dhcpConfig,
		iface:      iface,
		address:    address,
	}

	// TODO restrict binds to interfaces!
	if iface != "*" {
		log.Println("This tool currently only supports binding to '*'.")
		return s, errors.New("unknown interface")
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		log.Printf("'%s' is an invalid address:port value. Check the config.\n", address)
		return s, err
	}

	if host == "broadcast" {
		s.broadcast = true
		host = "0.0.0.0"
	}

	ipAddr := net.ParseIP(host)
	if ipAddr == nil {
		log.Printf("'%s' is not a valid IP address. Check the config.\n", host)
		return s, errors.New("invalid IP")
	}

	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 0 || portNumber > 65535 {
		log.Printf("'%s' is not a valid port. (No number, Out of range 0...65535).", port)
	}

	addr := net.UDPAddr{
		Port: portNumber,
		IP:   ipAddr,
	}

	conn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		return s, err
	}

	s.conn = conn

	return s, nil
}

func (s *ServerV4) Start() {
	buf := make([]byte, 1024)

	log.Printf("Listening on iface '%s' on addr '%s' for packets.", s.iface, s.address)
	for {
		// TODO graceful shutdown

		bytesReceived, sourceAddr, err := s.conn.ReadFromUDP(buf)
		if err != nil {
			log.Println("Timeout while waiting for packet.")
		}

		go handlePacket(buf[:bytesReceived], sourceAddr)
	}
}

func handlePacket(rawPackage []byte, sourceAddr *net.UDPAddr) {
	// todo check if broadcast == true!

	message, err := dhcpv4.FromBytes(rawPackage)
	if err != nil {
		log.Printf("Failed to parse DHCPv4 message from '%s'. Error: %s", sourceAddr, err)
	}

	log.Println("DHCP msg:", message)
}
