package dhcp

import (
	"errors"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/ninech/nine-dhcp2/dhcp/config"
	"github.com/ninech/nine-dhcp2/dhcp/v4"
	"github.com/ninech/nine-dhcp2/resolver"
	"github.com/ninech/nine-dhcp2/util"
	"log"
	"net"
	"strconv"
)

type ServerV4 struct {
	Resolver          resolver.Resolver
	dhcpConfig        *config.DHCPConfig
	conn              *v4.DHCPConn
	broadcast         bool
	iface             net.Interface
	shutdown          bool
	replyFrom         net.IP
	replyFromHostname string
}

func NewServerV4(dhcpConfig *config.DHCPConfig, resolver resolver.Resolver, iface net.Interface, ifaceConfig *config.V4InterfaceConfig) (s ServerV4, err error) {
	s = ServerV4{
		Resolver:          resolver,
		dhcpConfig:        dhcpConfig,
		iface:             iface,
		replyFromHostname: ifaceConfig.ReplyHostname,
	}

	replyFromAddress := ifaceConfig.ReplyFromAddress()
	if net.IPv4zero.Equal(replyFromAddress) || net.IPv4bcast.Equal(replyFromAddress) {
		ifaceAddrs, err := iface.Addrs()
		if err != nil {
			log.Printf("Can't determine replyFrom address: %s", err)
			return s, err
		}

		found := false
		for _, ifaceAddr := range ifaceAddrs {
			ipAddr, ok := ifaceAddr.(*net.IPNet)
			if !ok {
				log.Printf("Unexpected ipAddr type: %v", ipAddr)
				continue
			}

			ip4 := ipAddr.IP.To4()
			if ip4 == nil {
				log.Printf("IP is not IPv4: %s", ipAddr)
				continue
			}

			replyFromAddress = ip4
			found = true
			break
		}

		if !found {
			log.Printf("No replyFrom address found.")
			return s, errors.New("no replyFrom address")
		}
	}

	s.replyFrom = replyFromAddress

	conn, err := v4.ListenDHCPv4(iface, replyFromAddress)
	if err != nil {
		return s, err
	}

	s.conn = conn

	return s, nil
}

func (s *ServerV4) Start() {
	log.Printf("Listening on on iface '%s' for packets.", s.iface.Name)
	for {
		dhcpPack, sourceIP, sourceMAC, err := s.conn.ReadFrom()

		if s.shutdown {
			break
		}

		if err != nil {
			log.Println("Failed to process a packet: ", err)
			continue
		}

		go s.handlePacket(dhcpPack, sourceIP, sourceMAC)
	}
}

func (s *ServerV4) Stop() {
	s.shutdown = true
	s.conn.Close()
}

func (s *ServerV4) handlePacket(dhcp dhcpv4.DHCPv4, srcIP net.IP, srcMAC net.HardwareAddr) {
	log.Printf("DHCP message type: %v (sourceMAC: %s sourceIP: %s)", dhcp.MessageType(), srcMAC, srcIP)

	switch *dhcp.MessageType() {
	case dhcpv4.MessageTypeDiscover:
		s.replyToDiscover(&dhcp, &srcIP, &srcMAC)
	case dhcpv4.MessageTypeRequest:
		s.replyToRequest(&dhcp, &srcIP, &srcMAC)
	case dhcpv4.MessageTypeDecline:
		s.handleDecline(&dhcp, &srcIP, &srcMAC)
	case dhcpv4.MessageTypeRelease:
		s.handleRelease(&dhcp, &srcIP, &srcMAC)
	case dhcpv4.MessageTypeInform:
		s.replyToInform(&dhcp, &srcIP, &srcMAC)
	default:
		log.Printf("Unknown message type: '%v'", dhcp.MessageType())
	}
}

func (s *ServerV4) handleDecline(dhcpDecline *dhcpv4.DHCPv4, srcIP *net.IP, srcMAC *net.HardwareAddr) {
	mac, xid := getTransactionIDAndMAC(dhcpDecline)

	requestedIPOptions := dhcpDecline.GetOption(dhcpv4.OptionRequestedIPAddress)
	if len(requestedIPOptions) != 1 {
		log.Printf("%d IPs requested instead of one", len(requestedIPOptions))
		return
	}

	optRequestedIPAddress, err := dhcpv4.ParseOptRequestedIPAddress(requestedIPOptions[0].ToBytes())
	if err != nil {
		log.Printf("Can't decypher the requested IP from '%s'", requestedIPOptions[0].String())
	}

	requestedIP := optRequestedIPAddress.RequestedAddr.String()

	log.Printf("DHCPDECLINE from MAC '%s' and IP '%s' in transaction '%s'", mac, requestedIP, xid)

	s.Resolver.DeclineV4ByMAC(xid, mac, requestedIP)
}

func (s *ServerV4) handleRelease(dhcpRelease *dhcpv4.DHCPv4, srcIP *net.IP, srcMAC *net.HardwareAddr) {
	mac, xid := getTransactionIDAndMAC(dhcpRelease)

	ip := dhcpRelease.ClientIPAddr()
	if ip == nil {
		log.Printf("DHCPRELEASE from MAC '%s' with no client IP", mac)
		return
	}

	ip4 := ip.To4()
	if ip4 == nil || net.IPv4zero.Equal(ip4) || net.IPv4bcast.Equal(ip4) {
		log.Printf("DHCPRELEASE from MAC '%s' with invalid client IP '%s'", mac, ip)
		return
	}

	log.Printf("DHCPRELEASE from MAC '%s' and IP '%s' in transaction '%s'", mac, ip4, xid)

	s.Resolver.ReleaseV4ByMAC(xid, mac, ip4.String())
}

func (s *ServerV4) replyToInform(dhcpInform *dhcpv4.DHCPv4, srcIP *net.IP, srcMAC *net.HardwareAddr) {
	mac := dhcpInform.ClientHwAddrToString()
	log.Printf("DHCPINFORM for MAC '%s'", mac)

	// TODO implement
}

func (s *ServerV4) replyToDiscover(dhcpDiscover *dhcpv4.DHCPv4, srcIP *net.IP, srcMAC *net.HardwareAddr) {
	mac, xid := getTransactionIDAndMAC(dhcpDiscover)
	log.Printf("DHCPDISCOVER for MAC '%s' in transaction '%s'", mac, xid)

	clientInfo := resolver.NewClientInfoV4(s.dhcpConfig)

	err := s.Resolver.OfferV4ByMAC(clientInfo, xid, mac)
	if err != nil {
		log.Printf("Error finding IP for MAC '%s': %s", mac, err)
		return
	}

	dhcpOffer, err := s.prepareAnswer(dhcpDiscover, clientInfo, dhcpv4.MessageTypeOffer)
	if err != nil {
		return
	}

	dstIP, dstMAC := determineDstAddr(dhcpDiscover, dhcpOffer, srcMAC)

	log.Printf("Sending DHCPOFFER to '%s' ('%s') from '%s'", dstIP.String(), srcMAC, s.replyFrom)

	err = s.conn.WriteTo(*dhcpOffer, dstIP, dstMAC)
	if err != nil {
		log.Printf("Can't send DHCPOFFER to '%s' ('%s'): %s", dstIP.String(), srcMAC, err)
	}

	return
}

func (s *ServerV4) replyToRequest(dhcpRequest *dhcpv4.DHCPv4, srcIP *net.IP, srcMAC *net.HardwareAddr) {
	mac, xid := getTransactionIDAndMAC(dhcpRequest)

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
	log.Printf("DHCPREQUEST requesting IP '%s' for MAC '%s' and transaction '%s'", requestedIP, mac, xid)

	serverIPAddr := dhcpRequest.ServerIPAddr()
	if serverIPAddr != nil &&
		!serverIPAddr.Equal(net.IPv4zero) &&
		!serverIPAddr.Equal(net.IPv4bcast) &&
		!serverIPAddr.Equal(s.replyFrom) {
		log.Printf("DHCPREQUEST is not for us but for '%s'.", serverIPAddr.String())
		return
	}

	clientInfo := resolver.NewClientInfoV4(s.dhcpConfig)

	err = s.Resolver.AcknowledgeV4ByMAC(clientInfo, xid, mac, requestedIP)
	if err != nil {
		log.Printf("Error finding IP for MAC '%s': %s", mac, err)
		return
	}

	dhcpACK, err := s.prepareAnswer(dhcpRequest, clientInfo, dhcpv4.MessageTypeAck)
	if err != nil {
		return
	}

	dstIP, dstMAC := determineDstAddr(dhcpRequest, dhcpACK, srcMAC)

	log.Printf("Sending DHCPACK to '%s' from '%s'", dstIP.String(), s.replyFrom)

	err = s.conn.WriteTo(*dhcpACK, dstIP, dstMAC)
	if err != nil {
		log.Printf("Can't send DHCPOFFER to '%s' ('%s'): %s", dstIP.String(), srcMAC, err)
	}

	return
}

func getTransactionIDAndMAC(dhcpMsg *dhcpv4.DHCPv4) (string, string) {
	mac := dhcpMsg.ClientHwAddrToString()
	xid := strconv.FormatUint(uint64(dhcpMsg.TransactionID()), 16)
	return mac, xid
}

func determineDstAddr(in *dhcpv4.DHCPv4, out *dhcpv4.DHCPv4, srcMAC *net.HardwareAddr) (net.IP, net.HardwareAddr) {
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

	if in.GatewayIPAddr() != nil && // 'giaddr' is non-zero
		!in.GatewayIPAddr().Equal(net.IPv4zero) {
		out.SetBroadcast()

		// TODO srcMAC should be resolved using ARP
		return in.GatewayIPAddr(), *srcMAC
	} else if in.ClientIPAddr() != nil && // 'giaddr' is zero, 'ciaddr' is non-zero
		!in.ClientIPAddr().Equal(net.IPv4zero) {

		// TODO srcMAC should be resolved using ARP
		return in.ClientIPAddr(), *srcMAC
	} else if in.IsBroadcast() { // 'giaddr' and 'ciaddr' are zero, but broadcast flag
		return net.IPv4bcast, net.HardwareAddr{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
	} else {
		// Must avoid ARP (because client does not have IP yet)
		addr := out.ClientHwAddr()
		return out.YourIPAddr(), net.HardwareAddr(addr[:6])
	}
}

func (s *ServerV4) prepareAnswer(in *dhcpv4.DHCPv4, clientInfo *v4.ClientInfoV4, messageType dhcpv4.MessageType) (*dhcpv4.DHCPv4, error) {
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
	out.SetServerHostName([]byte(s.replyFromHostname))

	out.AddOption(&dhcpv4.OptMessageType{MessageType: messageType})
	out.AddOption(&dhcpv4.OptServerIdentifier{ServerID: s.replyFrom})
	out.AddOption(&dhcpv4.OptSubnetMask{SubnetMask: clientInfo.IPMask})

	if clientInfo.Timeouts.Lease > 0 {
		leaseTime := util.SafeConvertToUint32(clientInfo.Timeouts.Lease.Seconds())
		log.Printf("Lease Time: %s -> %d", clientInfo.Timeouts.Lease.String(), leaseTime)
		out.AddOption(&dhcpv4.OptIPAddressLeaseTime{LeaseTime: leaseTime})
	}
	if clientInfo.Timeouts.T1RenewalTime > 0 {
		renewalTime := util.SafeConvertToUint32(clientInfo.Timeouts.T1RenewalTime.Seconds())
		log.Printf("Renewal T1 Time: %s -> %d", clientInfo.Timeouts.T1RenewalTime.String(), renewalTime)
		out.AddOption(&v4.OptRenewalTime{RenewalTime: renewalTime})
	}
	if clientInfo.Timeouts.T2RebindingTime > 0 {
		rebindingTime := util.SafeConvertToUint32(clientInfo.Timeouts.T2RebindingTime.Seconds())
		log.Printf("Rebinding T2 Time: %s -> %d", clientInfo.Timeouts.T2RebindingTime.String(), rebindingTime)
		out.AddOption(&v4.OptRebindingTime{RebindingTime: rebindingTime})
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
