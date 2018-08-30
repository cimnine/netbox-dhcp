package dhcpv4

import (
	"errors"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/ninech/nine-dhcp2/dhcp/config"
	"github.com/ninech/nine-dhcp2/resolver"
	"github.com/ninech/nine-dhcp2/util"
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

	log.Printf("DHCP message type: %v", message.GetOption(dhcpv4.OptionDHCPMessageType))
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

	clientInfo := resolver.NewClientInfoV4(s.dhcpConfig)

	err := s.Resolver.OfferV4ByMAC(clientInfo, mac)
	if err != nil {
		log.Printf("Error finding IP for MAC '%s': %s", mac, err)
	}

	dhcpOffer, err := s.prepareAnswer(dhcpDiscover, clientInfo, dhcpv4.MessageTypeOffer)
	if err != nil {
		return
	}

	dstAddr := determineDstAddr(dhcpDiscover, dhcpOffer)

	log.Printf("Sending DHCPOFFER to '%s' from '%s'", dstAddr.String(), s.outConn.LocalAddr().String())

	s.outConn.WriteToUDP(dhcpOffer.ToBytes(), &dstAddr)

	return
}

func determineDstAddr(in *dhcpv4.DHCPv4, out *dhcpv4.DHCPv4) net.UDPAddr {
	/*
			 From the RFC2131, Page 23:

			 If the 'giaddr' field in a DHCP message from a client is non-zero,
		   the server sends any return messages to the 'DHCP server' port on the
		   BOOTP relay agent whose address appears in 'giaddr'. If the 'giaddr'
		   field is zero and the 'ciaddr' field is nonzero, then the server
		   unicasts DHCPOFFER and DHCPACK messages to the address in 'ciaddr'.
		   If 'giaddr' is zero and 'ciaddr' is zero, and the broadcast bit is
		   set, then the server broadcasts DHCPOFFER and DHCPACK messages to
		   0xffffffff. If the broadcast bit is not set and 'giaddr' is zero and
		   'ciaddr' is zero, then the server unicasts DHCPOFFER and DHCPACK
		   messages to the client's hardware address and 'yiaddr' address.  In
		   all cases, when 'giaddr' is zero, the server broadcasts any DHCPNAK
		   messages to 0xffffffff.
	*/

	var dstIP net.IP
	if in.GatewayIPAddr() != nil && // 'giaddr' is non-zero
		!in.GatewayIPAddr().Equal(net.IPv4zero) {
		log.Println("'giaddr' is non-zero")
		dstIP = in.GatewayIPAddr()
		out.SetBroadcast()
	} else if in.ClientIPAddr() != nil && // 'giaddr' is zero, 'ciaddr' is non-zero
		!in.ClientIPAddr().Equal(net.IPv4zero) {
		log.Println("'giaddr' is zero, 'ciaddr' is non-zero")
		dstIP = in.ClientIPAddr()
	} else if in.IsBroadcast() { // 'giaddr' and 'ciaddr' are zero, but broadcast flag
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
	return dstAddr
}

func (s *ServerV4) replyToRequest(dhcpRequest *dhcpv4.DHCPv4, sourceAddr *net.UDPAddr) {
	mac := dhcpRequest.ClientHwAddrToString()

	requestedIPOptions := dhcpRequest.GetOption(dhcpv4.OptionRequestedIPAddress)
	if len(requestedIPOptions) != 1 {
		log.Printf("%d IPs requested instead of one", len(requestedIPOptions))
		return
	}

	optRequestedIPAddress, err := dhcpv4.ParseOptRequestedIPAddress(requestedIPOptions[0].ToBytes())
	if err != nil {
		log.Printf("Can't decypher the requested IP from '%s'", requestedIPOptions[0].String())
	}

	requestedIP := optRequestedIPAddress.RequestedAddr.String()
	log.Printf("DHCPREQUEST requesting IP '%s' for MAC '%s'", requestedIP, mac)

	serverIPAddr := dhcpRequest.ServerIPAddr()
	if serverIPAddr != nil &&
		!serverIPAddr.Equal(net.IPv4zero) &&
		!serverIPAddr.Equal(net.IPv4bcast) &&
		!serverIPAddr.Equal(net.ParseIP(s.address)) {
		log.Printf("DHCPREQUEST is not for us but for '%s'.", serverIPAddr.String())
		return
	}

	clientInfo := resolver.NewClientInfoV4(s.dhcpConfig)

	err = s.Resolver.AcknowledgeV4ByMAC(clientInfo, mac, requestedIP)
	if err != nil {
		log.Printf("Error finding IP for MAC '%s': %s", mac, err)
		return
	}

	dhcpACK, err := s.prepareAnswer(dhcpRequest, clientInfo, dhcpv4.MessageTypeAck)
	if err != nil {
		return
	}

	dstAddr := determineDstAddr(dhcpRequest, dhcpACK)

	log.Printf("Sending DHCPACK to '%s' from '%s'", dstAddr.String(), s.outConn.LocalAddr().String())

	s.outConn.WriteToUDP(dhcpACK.ToBytes(), &dstAddr)

	return
}

func (s *ServerV4) prepareAnswer(in *dhcpv4.DHCPv4, clientInfo *resolver.ClientInfoV4, messageType dhcpv4.MessageType) (*dhcpv4.DHCPv4, error) {
	out, err := dhcpv4.New()
	if err != nil {
		log.Print("Can't create response.", err)
		return nil, err
	}

	siaddr := net.IPv4zero
	if clientInfo.NextServer != nil {
		siaddr = clientInfo.NextServer
	}

	hwAddr := in.ClientHwAddr()
	out.SetOpcode(dhcpv4.OpcodeBootReply)
	out.SetHopCount(0)
	out.SetTransactionID(in.TransactionID())
	out.SetNumSeconds(0)
	out.SetClientIPAddr(net.IPv4zero)
	out.SetYourIPAddr(clientInfo.IPAddr)
	out.SetServerIPAddr(siaddr)
	out.SetFlags(in.Flags())
	out.SetGatewayIPAddr(in.GatewayIPAddr())
	out.SetClientHwAddr(hwAddr[:])
	out.SetServerHostName([]byte(s.ifaceConfig.ReplyHostname))

	out.AddOption(&dhcpv4.OptMessageType{MessageType: messageType})
	out.AddOption(&dhcpv4.OptServerIdentifier{ServerID: s.ifaceConfig.ReplyFromAddress()})
	out.AddOption(&dhcpv4.OptSubnetMask{SubnetMask: clientInfo.IPMask})

	if clientInfo.Timeouts.Lease > 0 {
		leaseTime := util.SafeConvertToUint32(clientInfo.Timeouts.Lease.Seconds())
		log.Printf("Lease Time: %s -> %d", clientInfo.Timeouts.Lease.String(), leaseTime)
		out.AddOption(&dhcpv4.OptIPAddressLeaseTime{LeaseTime: leaseTime})
	}
	if clientInfo.Timeouts.T1RenewalTime > 0 {
		renewalTime := util.SafeConvertToUint32(clientInfo.Timeouts.T1RenewalTime.Seconds())
		log.Printf("Renewal T1 Time: %s -> %d", clientInfo.Timeouts.T1RenewalTime.String(), renewalTime)
		out.AddOption(&OptRenewalTime{RenewalTime: renewalTime})
	}
	if clientInfo.Timeouts.T2RebindingTime > 0 {
		rebindingTime := util.SafeConvertToUint32(clientInfo.Timeouts.T2RebindingTime.Seconds())
		log.Printf("Rebinding T2 Time: %s -> %d", clientInfo.Timeouts.T2RebindingTime.String(), rebindingTime)
		out.AddOption(&OptRebindingTime{RebindingTime: rebindingTime})
	}
	if clientInfo.Options.HostName != "" {
		out.AddOption(&dhcpv4.OptHostName{HostName: clientInfo.Options.HostName})
	}
	if clientInfo.Options.DomainName != "" {
		out.AddOption(&dhcpv4.OptDomainName{DomainName: clientInfo.Options.DomainName})
	}
	if len(clientInfo.Options.DomainNameServers) > 0 {
		out.AddOption(&dhcpv4.OptDomainNameServer{NameServers: clientInfo.Options.DomainNameServers})
	}
	if len(clientInfo.Options.Routers) > 0 {
		out.AddOption(&dhcpv4.OptRouter{Routers: clientInfo.Options.Routers})
	}
	if len(clientInfo.Options.NTPServers) > 0 {
		out.AddOption(&dhcpv4.OptNTPServers{NTPServers: clientInfo.Options.NTPServers})
	}
	if clientInfo.BootFileName != "" {
		out.AddOption(&dhcpv4.OptBootfileName{BootfileName: []byte(clientInfo.BootFileName)})
	}

	return out, nil
}
