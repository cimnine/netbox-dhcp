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

type ServerV6 struct {
	Resolver         resolver.Resolver
	dhcpConfig       *config.DHCPConfig
	listenerConfig   *config.V6ListenerConfig
	conn             *v6.DHCPV6Conn
	advertiseUnicast bool
	iface            net.Interface
	shutdown         bool
}

type dhcpv6OptMap map[layers.DHCPv6Opt]layers.DHCPv6Options

type iaNontemporaryAddress struct {
	iaid             [4]byte
	t1               uint32
	t2               uint32
	addressOptions   []iaAddress
	statusCodeOption statusCodeOption
	otherOptions     []iaOpts
}

type iaOpts struct {
	code uint16
	data []byte
}

type iaAddress struct {
	addr              net.IP
	preferredLifetime uint32
	validLifetime     uint32
	statusCodeOption  statusCodeOption
	otherOptions      []iaOpts
}

type statusCodeOption struct {
	len           uint16
	statusCode    uint16
	statusMessage string
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
			log.Println("Failed to process a DHCPv6 packet: ", err)
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
	log.Printf("DHCPv6 message type: %v (sourceMAC: %s sourceIP: %s)", dhcp.MsgType, srcMAC, srcIP)

	switch dhcp.MsgType {
	case layers.DHCPv6MsgTypeSolicit:
		s.replyToSolicit(dhcp, srcIP, srcMAC) // v4: "discover"
	case layers.DHCPv6MsgTypeRequest: // v4: "request"
		s.replyToRequest(dhcp, srcIP, srcMAC, false)
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
		log.Printf("DHCPv6 Unspecified message type: '%s'", dhcp.MsgType.String())
	default:
		log.Printf("DHCPv6 Unknown message type: '%x'", dhcp.MsgType)
	}
}

func (s *ServerV6) replyToSolicit(solicit layers.DHCPv6, srcIP net.IP, srcMAC net.HardwareAddr) {
	optMap := mapOpts(solicit.Options)

	if isClientIDMissing(optMap, srcIP) {
		return
	}

	rawClientDUID, clientDUID, err := extractClientDUID(optMap)
	if err != nil {
		log.Printf("Error while extracting DHCPv6 Client DUID from '%s' ('%s'): %s", srcIP, srcMAC, err)
		return
	}

	log.Printf("DHCPv6 SOLICIT message from '%s' with client ID '%s'.", srcIP, clientDUID)

	var clientMAC net.HardwareAddr

	if clientLLAddrOpt, found := optMap[layers.DHCPv6OptClientLinkLayerAddress]; found {
		firstClientMAC := s.getClientLLAddr(clientLLAddrOpt)

		clientMAC = firstClientMAC
	} else {
		clientMAC = srcMAC
	}

	clientInfo := v6.ClientInfoV6{}

	ok, err := s.Resolver.SolicitationV6(&clientInfo, clientDUID, clientMAC.String())
	if err != nil {
		log.Printf("DHCPv6 SOLICITATION failed for client ID '%s' / MAC '%s' because of an error: %s", clientDUID, srcMAC, err)
		return
	} else if !ok {
		log.Printf("Client with ID '%s' / MAC '%s' not found, ignoring DHCPv6 SOLICITATION", clientDUID, srcMAC)
		return
	}

	_, rapidCommit := optMap[layers.DHCPv6OptRapidCommit]
	if rapidCommit {
		log.Printf("DHCPv6 RAPID_COMMIT option detected for client DUID '%s' / MAC '%s'", clientDUID, srcMAC)

		s.replyToRequest(solicit, srcIP, srcMAC, true)
		return
	}

	// build response
	serverDUID, err := s.dhcpConfig.ServerDUID()
	if err != nil {
		log.Printf("DHCPv6 Server DUID improperly configured: %s", err)
		return
	}

	options := serverAndClientIDOptions(serverDUID, rawClientDUID)

	if s.listenerConfig.AdvertiseUnicast {
		allowUnicast := layers.DHCPv6Option{
			Code: layers.DHCPv6OptUnicast,
			// Length is fixed by the serializer,
			Data: s.listenerConfig.ReplyFromAddress(),
		}
		options = append(options, allowUnicast)
	}

	if _, hasIATA := optMap[layers.DHCPv6OptIATA]; hasIATA {
		// TODO support temporary address assignments
		options = append(options,
			statusOption(consts.DHCPv6StatusCodeNotOnLink,
				"Temporary Address detected, but that's not supported by this server"))
	} else if len(clientInfo.IPAddrs) == 0 {
		options = append(options,
			statusOption(consts.DHCPv6StatusCodeNoAddressesAvailable, "Not yet implemented"))
	} else if ianaOpts, hasIANA := optMap[layers.DHCPv6OptIANA]; hasIANA && !checkIANAs(ianaOpts, clientInfo) {
		options = append(options,
			statusOption(consts.DHCPv6StatusCodeNotOnLink,
				"According to this server's information some non-temporary IP addresses (IA_NA) are not designated for your machine."))
	} else {
		// TODO add IA and other options from clientInfo
	}

	advertise := layers.DHCPv6{
		MsgType:       layers.DHCPv6MsgTypeAdverstise,
		TransactionID: solicit.TransactionID,
		Options:       options,
	}

	fmt.Printf("%s", advertise.Options.String())

	var dstIP net.IP
	var dstMAC net.HardwareAddr

	dstIP = srcIP   // TODO handle relay case
	dstMAC = srcMAC // TODO handle relay case

	err = s.conn.WriteTo(advertise, dstIP, dstMAC)
	if err != nil {
		log.Printf("Can't send DHCPv6 ADVERTISE to '%s' ('%s'): %s", dstIP, dstMAC, err)
		return
	}

	log.Printf("Sent a DHCPv6 ADVERTISE for client ID '%s' / MAC '%s' msg to '%s' ('%s')",
		clientDUID, clientMAC, dstIP, dstMAC)
}

// checkIANAs returns true if all addresses of the given IANA option are also in the clientInfo
func checkIANAs(ianaOpts layers.DHCPv6Options, clientInfo v6.ClientInfoV6) bool {
	ianas := parseIANAOptions(ianaOpts)

	for _, ipRequestedByClient := range ianas {
		for _, ipOption := range ipRequestedByClient.addressOptions {
			if !isIpInList(ipOption.addr, clientInfo.IPAddrs) {
				return false
			}
		}
	}
	return true
}

// isIpInList checks if a given IP is in the given list of IPs and returns true if that's the case, false otherwise.
func isIpInList(ip net.IP, list []net.IP) bool {
	for _, ipOfList := range list {
		if ip.Equal(ipOfList) {
			return true
		}
	}
	return false
}

// parseIANAOptions parses an IA Non-Temporary Address DHCPv6 Option
func parseIANAOptions(ianaOpts layers.DHCPv6Options) []iaNontemporaryAddress {
	var nontemporaryAddresses []iaNontemporaryAddress

	for _, ianaOpt := range ianaOpts {
		code := layers.DHCPv6Opt(binary.BigEndian.Uint16(ianaOpt.Data[:2]))

		nontemporaryAddress := iaNontemporaryAddress{}
		switch code {
		case layers.DHCPv6OptIAAddr:
			addrOpt := iaAddress{
				addr:              ianaOpt.Data[4:20],
				preferredLifetime: binary.BigEndian.Uint32(ianaOpt.Data[20:24]),
				validLifetime:     binary.BigEndian.Uint32(ianaOpt.Data[24:28]),
			}

			nontemporaryAddress.addressOptions = append(nontemporaryAddress.addressOptions, addrOpt)

			statusCodeOption, ok := findStatusCodeOpt(ianaOpt.Data[28:])
			if ok {
				nontemporaryAddress.statusCodeOption = statusCodeOption
			}
		case layers.DHCPv6OptStatusCode:
			statusCode := statusCodeOption{
				len:           binary.BigEndian.Uint16(ianaOpt.Data[2:4]),
				statusCode:    binary.BigEndian.Uint16(ianaOpt.Data[4:6]),
				statusMessage: string(ianaOpt.Data[6:]),
			}

			nontemporaryAddress.statusCodeOption = statusCode
		}

		nontemporaryAddresses = append(nontemporaryAddresses, nontemporaryAddress)
	}

	return nontemporaryAddresses
}

// findStatusCodeOpt browses through the given data to find a DHCPv6 Status Option. It does this recursively.
// It returns the parsed option and true, if an option was found in the data.
// It returns an empty option and false, if no option was found in the data.
func findStatusCodeOpt(data []byte) (statusCodeOption, bool) {
	if len(data) < 6 {
		return statusCodeOption{}, false
	}

	endOfOption := binary.BigEndian.Uint16(data[2:4]) + 4 // 4 = status_code_len + opt_len,

	optCode := layers.DHCPv6Opt(binary.BigEndian.Uint16(data[0:2]))
	if optCode == layers.DHCPv6OptStatusCode {
		option := statusCodeOption{
			len:           binary.BigEndian.Uint16(data[2:4]),
			statusCode:    binary.BigEndian.Uint16(data[4:6]),
			statusMessage: string(data[6 : 4+endOfOption]),
		}

		return option, true
	}

	return findStatusCodeOpt(data[endOfOption:])
}

func (s *ServerV6) getClientLLAddr(clientLLAddrOpt layers.DHCPv6Options) net.HardwareAddr {
	if len(clientLLAddrOpt) == 0 {
		log.Printf(
			"Unable to parse DHCPv6 Client Link Layer Address option. Setting clientMAC to 00:00:00:00:00:00")
		return net.HardwareAddr{0, 0, 0, 0, 0, 0}
	}

	optData := clientLLAddrOpt[0].Data

	// llType := optData[:2]
	llAddr := net.HardwareAddr(optData[2:])

	if len(clientLLAddrOpt) > 1 {
		log.Printf("More than one DHCPv6 Client Link Layer Address option present. Using first: %s", llAddr)
	}

	return llAddr
}

func (s *ServerV6) replyToRequest(request layers.DHCPv6, srcIP net.IP, srcMAC net.HardwareAddr, rapidCommit bool) {
	optMap := mapOpts(request.Options)

	rawClientDUID, clientDUID, err := extractClientDUID(optMap)
	if err != nil {
		log.Printf("Error while extracting the DHCPv6 Client DUID of '%s' ('%s'): %s", srcIP, srcMAC, err)
		return
	}

	serverDUID, err := s.dhcpConfig.ServerDUID()
	if err != nil {
		log.Printf("DHCPv6 Server DUID improperly configured: %s", err)
		return
	}

	// When the server receives a Request message via unicast from a client
	// to which the server has not sent a unicast option, the server
	// discards the Request message and responds with a Reply message
	// containing a Status Code option with the value UseMulticast, a Server
	// Identifier option containing the server's DUID, the Client Identifier
	// option from the client message, and no other options.
	// https://tools.ietf.org/html/rfc3315#section-18.2.1
	if !s.listenerConfig.AdvertiseUnicast && srcIP.IsGlobalUnicast() {
		options := serverAndClientIDOptions(serverDUID, rawClientDUID)

		options = append(options, statusOption(
			consts.DHCPv6StatusCodeUseMulticast, "the anycast option is not enabled"))

		reply := layers.DHCPv6{
			MsgType:       layers.DHCPv6MsgTypeReply,
			TransactionID: request.TransactionID,
			Options:       options,
		}

		var dstIP net.IP
		var dstMAC net.HardwareAddr

		dstIP = srcIP   // TODO handle relay case
		dstMAC = srcMAC // TODO handle relay case

		err = s.conn.WriteTo(reply, dstIP, dstMAC)
		if err != nil {
			log.Printf("Can't send DHCPv6 ADVERTISE to '%s' ('%s'): %s", dstIP, dstMAC, err)
			return
		}

		log.Printf(
			"Sent a DHCPv6 REPLY with a status of 'UseMulticast' for client ID '%s' / MAC '%s' / IP '%s' msg to '%s' ('%s')",
			clientDUID, srcMAC, srcIP, dstIP, dstMAC)

		return
	}



}

// extractClientDUID returns the rawClientDUID for use in the response, and the clientDUID for use in a lookup
func extractClientDUID(optMap dhcpv6OptMap) ([]byte, string, error) {
	rawClientDUIDS, found := optMap[layers.DHCPv6OptClientID]
	if !found || len(rawClientDUIDS) == 0 {
		return nil, "", fmt.Errorf("internal error while extracting the DHCPv6 client DUID")
	}

	rawClientDUID := rawClientDUIDS[0].Data
	clientDUID, err := parseClientDUID(rawClientDUID)
	if err != nil {
		log.Printf("WARN: The client's DHCPv6 DUID was not correctly parsed: %s", err)
	}

	if len(rawClientDUIDS) > 1 {
		log.Printf("WARN: DHCPv6 message contains %d client ID options. Using first: '%s'",
			len(rawClientDUIDS), clientDUID)
	}

	return rawClientDUID, clientDUID, nil
}

// isClientIDMissing returns `true` if the request did not contain a client DUID.
func isClientIDMissing(optMap dhcpv6OptMap, srcIP net.IP) bool {
	if opts, found := optMap[layers.DHCPv6OptServerID]; found || len(opts) > 1 {
		log.Printf("DHCPv6 message from '%s' contains %d server ID option(s).", srcIP, len(opts))
	}
	_, found := optMap[layers.DHCPv6OptClientID];
	if !found {
		log.Printf("DHCPv6 message from '%s' does not contain a client ID option. Discarding the message.", srcIP)
		return true
	}
	return false
}

// mapOpts builds a map of option IDs to the corresponding option from a list of options
func mapOpts(options layers.DHCPv6Options) dhcpv6OptMap {
	optMap := make(dhcpv6OptMap)
	for _, opt := range options {
		if _, existing := optMap[opt.Code]; existing {
			optMap[opt.Code] = append(optMap[opt.Code], opt)
		} else {
			optMap[opt.Code] = layers.DHCPv6Options{opt}
		}
	}
	return optMap
}

// statusOption creates a status option from the given parameters
func statusOption(statusCode consts.DHCPv6StatusCode, statusMessage string) layers.DHCPv6Option {
	statusData := make([]byte, 2)
	binary.BigEndian.PutUint16(statusData, uint16(statusCode))
	statusData = append(statusData, statusMessage...)
	status := layers.DHCPv6Option{
		Code: layers.DHCPv6OptStatusCode,
		// Length is fixed by the serializer,
		Data: statusData,
	}
	return status
}

// serverAndClientIDOptions creates the server id option and the client id option and returns them together
func serverAndClientIDOptions(serverDUID []byte, rawClientDUID []byte) layers.DHCPv6Options {
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
	return options
}

// parseClientDUID spreads the DUID type code from the content.
// If the DUID is of type UUID, it parses the UUID.
// It will always return a string based on the content after the DUID type code.
// It will return an error if the content should be interpreted, but this was unsuccessful.
func parseClientDUID(duid []byte) (string, error) {
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
//  out.SetYourIPAddr(clientInfo.IPAddrs)
//  out.SetServerIPAddr(siaddr)
//  out.SetFlags(in.Flags())
//  out.SetGatewayIPAddr(in.GatewayIPAddr())
//  out.SetClientHwAddr(hwAddr[:])
//  out.SetServerHostName([]byte(s.replyFromHostname))
//
//  out.AddOption(&dhcpv4.OptMessageType{MessageType: messageType})
//  out.AddOption(&dhcpv4.OptServerIdentifier{ServerID: s.replyFrom})
//  out.AddOption(&dhcpv4.OptSubnetMask{SubnetMask: clientInfo.IPMasks})
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
