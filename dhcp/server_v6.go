package dhcp

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"

	"github.com/cimnine/netbox-dhcp/dhcp/config"
	"github.com/cimnine/netbox-dhcp/dhcp/v6"
	"github.com/cimnine/netbox-dhcp/dhcp/v6/consts"
	"github.com/cimnine/netbox-dhcp/resolver"
	"github.com/google/gopacket/layers"
	"github.com/satori/go.uuid"
)

const DHCPv6OptClientLinkLayerAddress = 79

type ServerV6 struct {
	Resolver         resolver.Resolver
	dhcpConfig       *config.DHCPConfig
	listenerConfig   *config.V6ListenerConfig
	conn             *v6.DHCPV6Conn
	advertiseUnicast bool
	iface            net.Interface
	shutdown         bool
}

func NewServerV6(dhcpConfig *config.DHCPConfig, resolver resolver.Resolver, iface net.Interface, listenerConfig *config.V6ListenerConfig) (s ServerV6, err error) {
	conn, err := v6.ListenDHCPv6(iface, listenerConfig.ListenToAddresses(), listenerConfig.ReplyFromAddress())
	if err != nil {
		return s, err
	}

	s = ServerV6{
		advertiseUnicast: listenerConfig.AdvertiseUnicast,
		conn:             conn,
		dhcpConfig:       dhcpConfig,
		listenerConfig:   listenerConfig,
		iface:            iface,
		Resolver:         resolver,
	}

	return s, nil
}

func (s *ServerV6) Start() {
	log.Printf("Listening on on iface '%s' for DHCPv6 requests.", s.iface.Name)
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

func (s *ServerV6) Stop() {
	s.shutdown = true
	s.conn.Close()
}

func (s *ServerV6) handlePacket(dhcp layers.DHCPv6, srcIP net.IP, srcMAC net.HardwareAddr) {
	log.Printf("DHCP message type: %v (sourceMAC: %s sourceIP: %s)", dhcp.MsgType, srcMAC, srcIP)

	switch dhcp.MsgType {
	case layers.DHCPv6MsgTypeSolicit:
		s.replyToSolicit(dhcp, srcIP, srcMAC) // v4: "discover"
	case layers.DHCPv6MsgTypeRequest: // v4: "request"
		//s.replyToRequest(dhcp, srcIP, srcMAC)
	case layers.DHCPv6MsgTypeConfirm:
		//s.replyToConfirm(dhcp, srcIP, srcMAC)
	case layers.DHCPv6MsgTypeRenew:
		//s.replyToRenew(dhcp, srcIP, srcMAC)
	case layers.DHCPv6MsgTypeRebind:
		//s.replyToRebind(dhcp, srcIP, srcMAC)
	case layers.DHCPv6MsgTypeDecline:
		//s.replyToDecline(dhcp, srcIP, srcMAC)
	case layers.DHCPv6MsgTypeRelease:
		//s.replyToRelease(dhcp, srcIP, srcMAC)
	case layers.DHCPv6MsgTypeInformationRequest:
		//s.replyToInformation(dhcp, srcIP, srcMAC)
	case layers.DHCPv6MsgTypeRelayForward:
		//s.replyToRelayForward(dhcp, srcIP, srcMAC)
	case layers.DHCPv6MsgTypeUnspecified:
		log.Printf("Unspecified message type: '%s'", dhcp.MsgType.String())
	default:
		log.Printf("Unknown message type: '%x'", dhcp.MsgType)
	}
}

func (s *ServerV6) replyToSolicit(solicit layers.DHCPv6, srcIP net.IP, srcMAC net.HardwareAddr) {
	optmap := make(map[layers.DHCPv6Opt]layers.DHCPv6Option)
	for _, opt := range solicit.Options {
		optmap[opt.Code] = opt
	}

	if _, found := optmap[layers.DHCPv6OptClientID]; !found {
		log.Printf("DHCPv6 SOLICIT message from '%s' does not contain a client ID option. Discarding the message.", srcIP)
		return
	}
	if _, found := optmap[layers.DHCPv6OptServerID]; found {
		log.Printf("DHCPv6 SOLICIT message from '%s' contains a server ID option.", srcIP)
	}

	rawClientDUID := optmap[layers.DHCPv6OptClientID].Data
	clientDUID, err := extractClientDUID(rawClientDUID)
	if err != nil {
		log.Printf("WARN: The client's DUID was not correctly parsed: %s", err)
	}
	log.Printf("DHCPv6 SOLICIT message from '%s' with client ID '%s'.", srcIP, clientDUID)

	var clientMAC net.HardwareAddr
	//if clientLLAddrOpt, found := optmap[layers.DHCPv6OptClientLinkLayerAddress]; found {

	if clientLLAddrOpt, found := optmap[DHCPv6OptClientLinkLayerAddress]; found {
		optContent := clientLLAddrOpt.Data
		//llType := binary.BigEndian.Uint16(optContent[:2])
		clientMAC = optContent[2:]
	} else {
		clientMAC = srcMAC
	}

	err = s.Resolver.SolicitationV6(clientDUID, clientMAC.String())
	if err != nil {
		log.Printf("SOLICITATION failed for client ID '%s' / MAC '%s'; ignoring request", clientDUID, srcMAC)
		return
	}

	_, rapidCommit := optmap[layers.DHCPv6OptRapidCommit]
	if rapidCommit {
		log.Printf("RAPID_COMMIT option detected for client DUID '%s' / MAC '%s'", clientDUID, srcMAC)

		// TODO switch to REPLY flow
		//clientInfo := resolver.NewClientInfoV6(s.dhcpConfig)
		//
		//err := s.Resolver.OfferV6ByID(clientInfo, rawClientDUID, srcMAC)
		//if err != nil {
		//  log.Printf("Error finding IPv6 for client DUID '%s' / MAC '%s': %s", rawClientDUID, srcMAC, err)
		//  return
		//}
		//return
	}

	// build response
	serverDUID, err := s.dhcpConfig.ServerDUID()
	if err != nil {
		log.Printf("Server DUID improperly configured: %s", err)
		return
	}

	serverIdentifier := layers.DHCPv6Option{
		Code: layers.DHCPv6OptServerID,
		// Length is fixed by the serializer,
		Data: serverDUID,
	}

	clientIdentifier := layers.DHCPv6Option{
		Code: layers.DHCPv6OptClientID,
		// Length is fixed by the serializer,
		Data: rawClientDUID,
	}

	options := layers.DHCPv6Options{
		serverIdentifier,
		clientIdentifier,
	}

	if s.listenerConfig.AdvertiseUnicast {
		allowUnicast := layers.DHCPv6Option{
			Code: layers.DHCPv6OptUnicast,
			// Length is fixed by the serializer,
			Data: s.listenerConfig.ReplyFromAddress(),
		}
		options = append(options, allowUnicast)
	}

	// TODO implement address fetching
	statusData := make([]byte, 2)
	binary.BigEndian.PutUint16(statusData, uint16(consts.DHCPv6StatusCodeNoAddressesAvailable))
	statusData = append(statusData, "Not yet implemented"...)
	status := layers.DHCPv6Option{
		Code: layers.DHCPv6OptStatusCode,
		// Length is fixed by the serializer,
		Data: statusData,
	}
	options = append(options, status)

	advertise := layers.DHCPv6{
		MsgType:       layers.DHCPv6MsgTypeAdverstise,
		TransactionID: solicit.TransactionID,
		Options:       options,
	}

	var dstIP net.IP            // TODO
	var dstMAC net.HardwareAddr // TODO

	dstIP = srcIP
	dstMAC = srcMAC

	err = s.conn.WriteTo(advertise, dstIP, dstMAC)
	if err != nil {
		log.Printf("Can't send ADVERTISE to '%s' ('%s'): %s", dstIP, dstMAC, err)
	}

	//
	//
	//mac, xid := s.getTransactionIDAndMAC(dhcpDiscover)
	//log.Printf("DHCPDISCOVER for MAC '%s' in transaction '%s'", mac, xid)
	//
	//clientInfo := resolver.NewClientInfoV6(s.dhcpConfig)
	//
	//err := s.Resolver.OfferV6ByMAC(clientInfo, xid, mac)
	//if err != nil {
	//  log.Printf("Error finding IP for MAC '%s': %s", mac, err)
	//  return
	//}
	//
	//dhcpOffer, err := s.prepareAnswer(dhcpDiscover, clientInfo, dhcpv4.MessageTypeOffer)
	//if err != nil {
	//  return
	//}
	//
	//dstIP, dstMAC := s.determineDstAddr(dhcpDiscover, dhcpOffer, srcMAC)
	//
	//log.Printf("Sending DHCPOFFER to '%s' ('%s') from '%s'", dstIP.String(), srcMAC, s.replyFrom)
	//
	//err = s.conn.WriteTo(*dhcpOffer, dstIP, dstMAC)
	//if err != nil {
	//  log.Printf("Can't send DHCPOFFER to '%s' ('%s'): %s", dstIP.String(), srcMAC, err)
	//}
	//
	//return
}

// extractClientDUID spreads the DUID type code from the content.
// If the DUID is of type UUID, it parses the UUID.
// It will always return a string based on the content after the DUID type code.
// It will return an error if the content should be interpreted, but this was unsuccessful.
func extractClientDUID(duid []byte) (string, error) {
	duidTypeCode := consts.DHCPv6DUIDTypeCode(binary.BigEndian.Uint16(duid[:2]))
	switch duidTypeCode {
	case consts.DHCPv6DUIDTypeUUID:
		u, err := uuid.FromBytes(duid[2:18])
		if err != nil {
			return fmt.Sprintf("%x", duid[2:]),
				fmt.Errorf("'%x' was expected to be an UUID, but parsing was not successful", duid[2:18])
		}
		return u.String(), nil
	case consts.DHCPv6DUIDTypeLinkLayerAddressPlusTime,
		consts.DHCPv6DUIDTypeVendorBasedOnEnterpriseNumber,
		consts.DHCPv6DUIDTypeLinkLayerAddress:
		return fmt.Sprintf("%x", duid[2:]), nil
	default:
		return fmt.Sprintf("%x", duid[2:]),
			fmt.Errorf("unrecognized DUID type code '%d'", duidTypeCode)
	}
}

//func (s *ServerV6) handleDecline(dhcpDecline *dhcpv4.DHCPv4, srcIP *net.IP, srcMAC *net.HardwareAddr) {
//  mac, xid := s.getTransactionIDAndMAC(dhcpDecline)
//
//  requestedIPOptions := dhcpDecline.GetOption(dhcpv4.OptionRequestedIPAddress)
//  if len(requestedIPOptions) != 1 {
//    log.Printf("%d IPs requested instead of one", len(requestedIPOptions))
//    return
//  }
//
//  optRequestedIPAddress, err := dhcpv4.ParseOptRequestedIPAddress(requestedIPOptions[0].ToBytes())
//  if err != nil {
//    log.Printf("Can't decypher the requested IP from '%s'", requestedIPOptions[0].String())
//  }
//
//  requestedIP := optRequestedIPAddress.RequestedAddr.String()
//
//  log.Printf("DHCPDECLINE from MAC '%s' and IP '%s' in transaction '%s'", mac, requestedIP, xid)
//
//  s.Resolver.DeclineV6ByMAC(xid, mac, requestedIP)
//}
//
//func (s *ServerV6) handleRelease(dhcpRelease *dhcpv4.DHCPv4, srcIP *net.IP, srcMAC *net.HardwareAddr) {
//  mac, xid := s.getTransactionIDAndMAC(dhcpRelease)
//
//  ip := dhcpRelease.ClientIPAddr()
//  if ip == nil {
//    log.Printf("DHCPRELEASE from MAC '%s' with no client IP", mac)
//    return
//  }
//
//  ip4 := ip.To4()
//  if ip4 == nil || net.IPv4zero.Equal(ip4) || net.IPv4bcast.Equal(ip4) {
//    log.Printf("DHCPRELEASE from MAC '%s' with invalid client IP '%s'", mac, ip)
//    return
//  }
//
//  log.Printf("DHCPRELEASE from MAC '%s' and IP '%s' in transaction '%s'", mac, ip4, xid)
//
//  s.Resolver.ReleaseV6ByMAC(xid, mac, ip4.String())
//}
//
//func (s *ServerV6) replyToInform(dhcpInform *dhcpv4.DHCPv4, srcIP *net.IP, srcMAC *net.HardwareAddr) {
//  mac := dhcpInform.ClientHwAddrToString()
//  log.Printf("DHCPINFORM for MAC '%s'", mac)
//
//  // TODO implement
//}
//
//func (s *ServerV6) replyToRequest(dhcpRequest *dhcpv4.DHCPv4, srcIP *net.IP, srcMAC *net.HardwareAddr) {
//  mac, xid := s.getTransactionIDAndMAC(dhcpRequest)
//
//  requestedIPOptions := dhcpRequest.GetOption(dhcpv4.OptionRequestedIPAddress)
//  if len(requestedIPOptions) != 1 {
//    log.Printf("%d IPs requested instead of one", len(requestedIPOptions))
//    return
//  }
//
//  optRequestedIPAddress, err := dhcpv4.ParseOptRequestedIPAddress(requestedIPOptions[0].ToBytes())
//  if err != nil {
//    log.Printf("Can't decypher the requested IP from '%s'", requestedIPOptions[0].String())
//  }
//
//  requestedIP := optRequestedIPAddress.RequestedAddr.String()
//  log.Printf("DHCPREQUEST requesting IP '%s' for MAC '%s' and transaction '%s'", requestedIP, mac, xid)
//
//  serverIPAddr := dhcpRequest.ServerIPAddr()
//  if serverIPAddr != nil &&
//    !serverIPAddr.Equal(net.IPv4zero) &&
//    !serverIPAddr.Equal(net.IPv4bcast) &&
//    !serverIPAddr.Equal(s.replyFrom) {
//    log.Printf("DHCPREQUEST is not for us but for '%s'.", serverIPAddr.String())
//    return
//  }
//
//  clientInfo := resolver.NewClientInfoV6(s.dhcpConfig)
//
//  err = s.Resolver.AcknowledgeV6ByMAC(clientInfo, xid, mac, requestedIP)
//  if err != nil {
//    log.Printf("Error finding IP for MAC '%s': %s", mac, err)
//    return
//  }
//
//  dhcpACK, err := s.prepareAnswer(dhcpRequest, clientInfo, dhcpv4.MessageTypeAck)
//  if err != nil {
//    return
//  }
//
//  dstIP, dstMAC := s.determineDstAddr(dhcpRequest, dhcpACK, srcMAC)
//
//  log.Printf("Sending DHCPACK to '%s' from '%s'", dstIP.String(), s.replyFrom)
//
//  err = s.conn.WriteTo(*dhcpACK, dstIP, dstMAC)
//  if err != nil {
//    log.Printf("Can't send DHCPOFFER to '%s' ('%s'): %s", dstIP.String(), srcMAC, err)
//  }
//
//  return
//}
//
//func (s *ServerV6) getTransactionIDAndMAC(dhcpMsg *dhcpv4.DHCPv4) (string, string) {
//  mac := dhcpMsg.ClientHwAddrToString()
//  xid := strconv.FormatUint(uint64(dhcpMsg.TransactionID()), 16)
//  return mac, xid
//}
//
//func (s *ServerV6) determineDstAddr(in *dhcpv4.DHCPv4, out *dhcpv4.DHCPv4, srcMAC *net.HardwareAddr) (net.IP, net.HardwareAddr) {
//  /*
//       From the RFC2131, Page 23:
//
//       If the 'giaddr' field in a DHCP message from a client is non-zero,
//       the server sends any return messages to the 'DHCP server' port on the
//       BOOTP relay agent whose address appears in 'giaddr'. If the 'giaddr'
//       field is zero and the 'ciaddr' field is nonzero, then the server
//       unicasts DHCPOFFER and DHCPACK messages to the address in 'ciaddr'.
//       If 'giaddr' is zero and 'ciaddr' is zero, and the broadcast bit is
//       set, then the server broadcasts DHCPOFFER and DHCPACK messages to
//       0xffffffff. If the broadcast bit is not set and 'giaddr' is zero and
//       'ciaddr' is zero, then the server unicasts DHCPOFFER and DHCPACK
//       messages to the client's hardware address and 'yiaddr' address.  In
//       all cases, when 'giaddr' is zero, the server broadcasts any DHCPNAK
//       messages to 0xffffffff.
//  */
//
//  if in.GatewayIPAddr() != nil && // 'giaddr' is non-zero
//    !in.GatewayIPAddr().Equal(net.IPv4zero) {
//    out.SetBroadcast()
//
//    // TODO srcMAC should be resolved using ARP
//    return in.GatewayIPAddr(), *srcMAC
//  } else if in.ClientIPAddr() != nil && // 'giaddr' is zero, 'ciaddr' is non-zero
//    !in.ClientIPAddr().Equal(net.IPv4zero) {
//
//    // TODO srcMAC should be resolved using ARP
//    return in.ClientIPAddr(), *srcMAC
//  } else if in.IsBroadcast() { // 'giaddr' and 'ciaddr' are zero, but broadcast flag
//    return net.IPv4bcast, net.HardwareAddr{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF}
//  } else {
//    // Must avoid ARP (because client does not have IP yet)
//    addr := out.ClientHwAddr()
//    return out.YourIPAddr(), net.HardwareAddr(addr[:6])
//  }
//}
//
//func (s *ServerV6) prepareAnswer(in *dhcpv4.DHCPv4, clientInfo *v4.ClientInfoV6, messageType dhcpv4.MessageType) (*dhcpv4.DHCPv4, error) {
//  out, err := dhcpv4.New()
//  if err != nil {
//    log.Print("Can't create response.", err)
//    return nil, err
//  }
//
//  siaddr := net.IPv4zero
//  if clientInfo.NextServer != nil {
//    siaddr = clientInfo.NextServer
//  }
//
//  hwAddr := in.ClientHwAddr()
//  out.SetOpcode(dhcpv4.OpcodeBootReply)
//  out.SetHopCount(0)
//  out.SetTransactionID(in.TransactionID())
//  out.SetNumSeconds(0)
//  out.SetClientIPAddr(net.IPv4zero)
//  out.SetYourIPAddr(clientInfo.IPAddr)
//  out.SetServerIPAddr(siaddr)
//  out.SetFlags(in.Flags())
//  out.SetGatewayIPAddr(in.GatewayIPAddr())
//  out.SetClientHwAddr(hwAddr[:])
//  out.SetServerHostName([]byte(s.replyFromHostname))
//
//  out.AddOption(&dhcpv4.OptMessageType{MessageType: messageType})
//  out.AddOption(&dhcpv4.OptServerIdentifier{ServerID: s.replyFrom})
//  out.AddOption(&dhcpv4.OptSubnetMask{SubnetMask: clientInfo.IPMask})
//
//  if clientInfo.Timeouts.Lease > 0 {
//    leaseTime := util.SafeConvertToUint32(clientInfo.Timeouts.Lease.Seconds())
//    log.Printf("Lease Time: %s -> %d", clientInfo.Timeouts.Lease.String(), leaseTime)
//    out.AddOption(&dhcpv4.OptIPAddressLeaseTime{LeaseTime: leaseTime})
//  }
//  if clientInfo.Timeouts.T1RenewalTime > 0 {
//    renewalTime := util.SafeConvertToUint32(clientInfo.Timeouts.T1RenewalTime.Seconds())
//    log.Printf("Renewal T1 Time: %s -> %d", clientInfo.Timeouts.T1RenewalTime.String(), renewalTime)
//    out.AddOption(&v4.OptRenewalTime{RenewalTime: renewalTime})
//  }
//  if clientInfo.Timeouts.T2RebindingTime > 0 {
//    rebindingTime := util.SafeConvertToUint32(clientInfo.Timeouts.T2RebindingTime.Seconds())
//    log.Printf("Rebinding T2 Time: %s -> %d", clientInfo.Timeouts.T2RebindingTime.String(), rebindingTime)
//    out.AddOption(&v4.OptRebindingTime{RebindingTime: rebindingTime})
//  }
//  if clientInfo.Options.HostName != "" {
//    out.AddOption(&dhcpv4.OptHostName{HostName: clientInfo.Options.HostName})
//  }
//  if clientInfo.Options.DomainName != "" {
//    out.AddOption(&dhcpv4.OptDomainName{DomainName: clientInfo.Options.DomainName})
//  }
//  if len(clientInfo.Options.DomainNameServers) > 0 {
//    out.AddOption(&dhcpv4.OptDomainNameServer{NameServers: clientInfo.Options.DomainNameServers})
//  }
//  if len(clientInfo.Options.Routers) > 0 {
//    out.AddOption(&dhcpv4.OptRouter{Routers: clientInfo.Options.Routers})
//  }
//  if len(clientInfo.Options.NTPServers) > 0 {
//    out.AddOption(&dhcpv4.OptNTPServers{NTPServers: clientInfo.Options.NTPServers})
//  }
//  if clientInfo.BootFileName != "" {
//    out.AddOption(&dhcpv4.OptBootfileName{BootfileName: []byte(clientInfo.BootFileName)})
//  }
//
//  return out, nil
//}
