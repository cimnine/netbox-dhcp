package dhcpv4

import (
	"errors"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/ninech/nine-dhcp2/dhcp/config"
	"github.com/ninech/nine-dhcp2/resolver"
	"log"
	"net"
)

type ServerV4 struct {
	Resolver    resolver.Resolver
	dhcpConfig  *config.DHCPConfig
	ifaceConfig *config.V4InterfaceConfig
	inConn      *net.UDPConn
	outConn     *net.UDPConn
	broadcast   bool
	address     string
	shutdown    bool
}

func NewServer(dhcpConfig *config.DHCPConfig, resolver resolver.Resolver, address string, ifaceConfig *config.V4InterfaceConfig) (s ServerV4, err error) {
	s = ServerV4{
		Resolver:    resolver,
		dhcpConfig:  dhcpConfig,
		address:     address,
		ifaceConfig: ifaceConfig,
	}

	ipAddr := net.ParseIP(address)
	if ipAddr == nil {
		log.Printf("'%s' is not a valid IP address. Check the config.\n", address)
		return s, errors.New("invalid IP")
	}

	inAddr := net.UDPAddr{
		Port: dhcpv4.ServerPort,
		IP:   ipAddr,
	}

	inConn, err := net.ListenUDP("udp4", &inAddr)
	if err != nil {
		return s, err
	}

	s.inConn = inConn

	if address == ifaceConfig.ReplyFrom {
		s.outConn = inConn
		return s, nil
	}

	outIP := net.ParseIP(ifaceConfig.ReplyFrom)
	if outIP == nil {
		log.Printf("'%s' is not a valid IP address. Check the config.\n", ifaceConfig.ReplyFrom)
		return s, errors.New("invalid IP")
	}

	outAddr := net.UDPAddr{
		Port: dhcpv4.ServerPort,
		IP:   outIP,
	}

	outConn, err := net.ListenUDP("udp4", &outAddr)
	if err != nil {
		return s, err
	}

	s.outConn = outConn

	return s, nil
}

func (s *ServerV4) Start() {
	buf := make([]byte, 1024)

	log.Printf("Listening on on addr '%s' for packets.", s.address)
	for {
		bytesReceived, sourceAddr, err := s.inConn.ReadFromUDP(buf)

		if s.shutdown {
			break
		}

		if err != nil {
			log.Println("Timeout while waiting for packet.", err)
		}

		if bytesReceived > 0 { // e.g. when the socket was closed
			go s.handlePacket(buf[:bytesReceived], sourceAddr)
		}
	}
}

func (s *ServerV4) Stop() {
	s.shutdown = true
	s.inConn.Close()
}

func (s *ServerV4) handlePacket(rawPackage []byte, sourceAddr *net.UDPAddr) {
	log.Printf("Packet received (%d bytes)", len(rawPackage))

	// todo check if broadcast == true!
	log.Printf("sourceAddr: %v", sourceAddr)

	message, err := dhcpv4.FromBytes(rawPackage)
	if err != nil {
		log.Printf("Failed to parse DHCPv4 message from '%s'. Error: %s", sourceAddr, err)
	}

	log.Printf("DHCP message: %v", message)
	switch *message.MessageType() {
	case dhcpv4.MessageTypeDiscover:
		s.replyToDiscover(message, sourceAddr)
	case dhcpv4.MessageTypeRequest:
		s.replyToRequest(message, sourceAddr)
	default:
		log.Printf("Unknown message type: '%v'", message.MessageType())
	}
}

func (s *ServerV4) replyToDiscover(dhcpDiscover *dhcpv4.DHCPv4, sourceAddr *net.UDPAddr) {
	mac := dhcpDiscover.ClientHwAddrToString()
	log.Printf("DHCPDISCOVER for MAC '%s'", mac)

	clientInfo, err := s.Resolver.OfferV4ByMAC(mac)
	if err != nil {
		log.Printf("Error finding IP for MAC '%s': %s", mac, err)
	}

	clientHwAddr := make([]byte, 16)
	for i, hwAddrByte := range dhcpDiscover.ClientHwAddr() {
		clientHwAddr[i] = hwAddrByte
	}

	response, err := dhcpv4.New()
	if err != nil {
		log.Print("Can't create response.", err)
	}

	response.SetOpcode(dhcpv4.OpcodeBootReply)
	response.SetTransactionID(dhcpDiscover.TransactionID())
	response.SetClientIPAddr(net.IPv4zero)
	response.SetYourIPAddr(clientInfo.IPAddr)
	response.SetFlags(dhcpDiscover.Flags())
	response.SetGatewayIPAddr(dhcpDiscover.GatewayIPAddr())
	response.SetClientHwAddr(clientHwAddr)
	response.SetServerHostName([]byte(s.ifaceConfig.ReplyHostname))
	response.SetBootFileName([]byte(clientInfo.BootFileName))

	response.AddOption(&dhcpv4.OptIPAddressLeaseTime{LeaseTime: 600})
	response.AddOption(&dhcpv4.OptHostName{HostName: clientInfo.Options.HostName})
	response.AddOption(&dhcpv4.OptDomainName{DomainName: clientInfo.Options.DomainName})
	response.AddOption(&dhcpv4.OptDomainNameServer{NameServers: clientInfo.Options.DomainNameServers})
	response.AddOption(&dhcpv4.OptRouter{Routers: clientInfo.Options.Routers})
	response.AddOption(&dhcpv4.OptNTPServers{NTPServers: clientInfo.Options.TimeServers})
	response.AddOption(&dhcpv4.OptMessageType{MessageType: dhcpv4.MessageTypeOffer})
	response.AddOption(&dhcpv4.OptServerIdentifier{ServerID: s.ifaceConfig.ReplyFromAddress()})
	// TODO maybe add T1 & T2

	response.SetServerIPAddr(net.ParseIP(s.ifaceConfig.ReplyFrom))

	var dstIP net.IP

	if dhcpDiscover.GatewayIPAddr() != nil && // 'giaddr' is non-zero
		!dhcpDiscover.GatewayIPAddr().Equal(net.IPv4zero) {
		log.Println("'giaddr' is non-zero")
		dstIP = dhcpDiscover.GatewayIPAddr()
		response.SetBroadcast()
	} else if dhcpDiscover.ClientIPAddr() != nil && // 'giaddr' is zero, 'ciaddr' is non-zero
		!dhcpDiscover.ClientIPAddr().Equal(net.IPv4zero) {
		log.Println("'giaddr' is zero, 'ciaddr' is non-zero")
		dstIP = dhcpDiscover.ClientIPAddr()
	} else if dhcpDiscover.IsBroadcast() { // 'giaddr' and 'ciaddr' are zero, but broadcast flag
		log.Println("'giaddr' and 'ciaddr' are zero, but broadcast flag")
		dstIP = net.IPv4bcast
	} else {
		log.Println("unicast therefore")
		// TODO implement unicast to MAC
		// Must avoid ARP (because client does not have IP yet)
		// Therefore must use a raw socket
		//dstIP = clientInfo.IPAddr

		// send to broadcast for now
		dstIP = net.IPv4bcast
	}

	dstAddr := net.UDPAddr{
		IP:   dstIP,
		Port: dhcpv4.ClientPort,
	}

	log.Printf("Sending DHCPOFFER to '%s' from '%s'", dstAddr.String(), s.outConn.LocalAddr().String())

	s.outConn.WriteToUDP(response.ToBytes(), &dstAddr)

	return
}

func (s *ServerV4) replyToRequest(dhcpDiscover *dhcpv4.DHCPv4, sourceAddr *net.UDPAddr) {
	mac := dhcpDiscover.ClientHwAddrToString()

	requestedIPOptions := dhcpDiscover.GetOption(dhcpv4.OptionRequestedIPAddress)
	if len(requestedIPOptions) != 1 {
		log.Printf("%d IPs requested instead of one", len(requestedIPOptions))
		return
	}

	requestedIP := requestedIPOptions[0].String()

	log.Printf("DHCPREQUEST for IP '%s' from MAC '%s'", requestedIP, mac)

	if dhcpDiscover.ServerIPAddr() != nil &&
		!dhcpDiscover.ServerIPAddr().Equal(net.ParseIP(s.address)) {
		log.Printf("DHCPREQUEST is not for us but for '%s'.", dhcpDiscover.ServerIPAddr().String())
		return
	}

	clientInfo, err := s.Resolver.AcknowledgeV4ByMAC(mac, requestedIP)
	if err != nil {
		log.Printf("Error finding IP for MAC '%s': %s", mac, err)
	}

	log.Printf("Accepting '%s'", clientInfo.IPAddr)

	// TODO implement rest
}
