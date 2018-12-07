package dhcp

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"

	"github.com/google/gopacket/layers"
	"github.com/satori/go.uuid"

	"github.com/cimnine/netbox-dhcp/dhcp/config"
	"github.com/cimnine/netbox-dhcp/dhcp/v6"
	"github.com/cimnine/netbox-dhcp/dhcp/v6/consts"
	"github.com/cimnine/netbox-dhcp/resolver"
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
	_ = s.conn.Close()
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

	// TODO handle relay case, i.e. extract original message and such
	dstIP := srcIP
	dstMAC := srcMAC

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

	if _, rapidCommitRequested := optMap[layers.DHCPv6OptRapidCommit]; rapidCommitRequested {
		log.Printf("DHCPv6 RAPID_COMMIT option detected for client DUID '%s' / MAC '%s'", clientDUID, clientMAC)

		s.replyToRequest(solicit, srcIP, srcMAC, true)
		return
	}

	// TODO support temporary address assignments
	if _, hasIATA := optMap[layers.DHCPv6OptIATA]; hasIATA {
		statusCode := layers.DHCPv6StatusCodeNotOnLink
		statusMessage := "Temporary Address detected, but that's not supported by this server"
		status := statusOption(statusCode, statusMessage)

		err = s.sendAdvertise(rawClientDUID, layers.DHCPv6Options{status}, solicit.TransactionID, dstIP, dstMAC)
		if err != nil {
			log.Printf(
				"Can't send DHCPv6 ADVERTISE with status %d ('%s') for client ID '%s' / MAC '%s' to '%s' ('%s'): %s",
				statusCode, statusCode.String(), clientDUID, clientMAC, dstIP, dstMAC, err)
		}
		return
	}

	inIANAOpts, hasIANA := optMap[layers.DHCPv6OptIANA]
	outIANAOpts := make(layers.DHCPv6Options, len(inIANAOpts))
	if hasIANA {
		for _, inIanaOpt := range inIANAOpts {
			clientInfo := resolver.NewClientInfoV6(s.dhcpConfig)

			iana := v6.ParseIANAOption(inIanaOpt)
			iaid := iana.IAID.String()

			ok, err := s.Resolver.SolicitationV6(&clientInfo, clientDUID, clientMAC.String(), iaid)

			if err != nil {
				log.Printf(
					"DHCPv6 SOLICITATION failed for client ID '%s' / MAC '%s' and IAID '%s' because of an error: %s",
					clientDUID, clientMAC, iaid, err)
				continue
			} else if !ok {
				log.Printf(
					"Client with ID '%s' / MAC '%s' and IAID '%s' not found.",
					clientDUID, clientMAC, iaid)
				continue
			} else if len(clientInfo.IPAddrs) == 0 {
				log.Printf(
					"No IPs for the Client with ID '%s' / MAC '%s' and IAID '%s' not found.",
					clientDUID, clientMAC, iaid)
				continue
			}

			outIanaOpt, statusOpt, err := s.handleIANA(iana, clientInfo, clientDUID, clientMAC)

			if err != nil {
				err = s.sendAdvertise(rawClientDUID, layers.DHCPv6Options{statusOpt}, solicit.TransactionID, dstIP, dstMAC)
				if err != nil {
					log.Printf(
						"Can't send DHCPv6 ADVERTISE with status != success for client ID '%s' / MAC '%s' to '%s' ('%s'): %s",
						clientDUID, clientMAC, dstIP, dstMAC, err)
				}

				log.Printf(
					"Can't match IPs to the IA_NA with IAID '%s' for the client with ID '%s' / MAC '%s': %s",
					iaid, clientDUID, clientMAC, err)
				return
			}

			outIANAOpts = append(outIANAOpts, outIanaOpt)
		}
	}

	if len(outIANAOpts) == 0 {
		statusCode := layers.DHCPv6StatusCodeNoAddrsAvail
		statusMessage := "No addresses found for your machine."
		status := statusOption(statusCode, statusMessage)
		err = s.sendAdvertise(rawClientDUID, layers.DHCPv6Options{status}, solicit.TransactionID, dstIP, dstMAC)

		if err != nil {
			log.Printf(
				"Can't send DHCPv6 ADVERTISE with status %d ('%s') for client ID '%s' / MAC '%s' to '%s' ('%s'): %s",
				statusCode, statusCode.String(), clientDUID, clientMAC, dstIP, dstMAC, err)
		}
		return
	}

	successOption := statusOption(layers.DHCPv6StatusCodeSuccess, "")

	err = s.sendAdvertise(rawClientDUID, append(outIANAOpts, successOption), solicit.TransactionID, dstIP, dstMAC)
	if err != nil {
		log.Printf("Can't send DHCPv6 ADVERTISE for client ID '%s' / MAC '%s' to '%s' ('%s'): %s",
			clientDUID, clientMAC, dstIP, dstMAC, err)
		return
	}

	log.Printf("Sent a DHCPv6 ADVERTISE for client ID '%s' / MAC '%s' msg to '%s' ('%s')",
		clientDUID, clientMAC, dstIP, dstMAC)
}

// handleIANA returns the IPs returned for the client as DHCPv6 IA_NA Option.
// See https://tools.ietf.org/html/rfc8415#section-21.4
// It also checks if the IPs requested by the client match the IPs reserved for the client and returns
// an error and a STATUS_CODE NOT_ON_LINK if a missmatch is detected.
func (s *ServerV6) handleIANA(iana v6.IANontemporaryAddress, clientInfo v6.ClientInfoV6, clientDUID string, clientMAC net.HardwareAddr) (layers.DHCPv6Option, layers.DHCPv6Option, error) {
	var outIanaOpt layers.DHCPv6Option
	var outStatusOpt layers.DHCPv6Option

	log.Printf("%d IPv6s to assign to the client ID '%s' / MAC '%s'.", len(clientInfo.IPAddrs), clientDUID, clientMAC) // TODO remove

	if !v6.CheckIANA(iana, clientInfo) {
		statusCode := layers.DHCPv6StatusCodeNotOnLink
		statusMessage := "According to this server's information some non-temporary IP addresses (IA_NA) are not designated for your machine."

		outStatusOpt = statusOption(statusCode, statusMessage)
		return outIanaOpt, outStatusOpt,
			fmt.Errorf("some IA_NA the client requested are not reserved for that client")
	}

	options, err := v6.EncodeOptions(iana.IAID, clientInfo)
	if err != nil {
		log.Printf(
			"DHCPv6 SOLICITAION failed for the client ID '%s' / MAC '%s' because of an error while building the response: %s",
			clientDUID, clientMAC)
		return outIanaOpt, outStatusOpt, err
	}

	outIanaOpt = layers.DHCPv6Option{
		Code: layers.DHCPv6OptIANA,
		// Length: 0, fixed by the serializer
		Data: options,
	}

	return outIanaOpt, outStatusOpt, nil
}

func (s *ServerV6) sendAdvertise(rawClientDUID []byte, incomingOpts layers.DHCPv6Options, transactionID []byte, dstIP net.IP, dstMAC net.HardwareAddr) error {
	options, err := s.serverAndClientIDOptions(rawClientDUID)
	if err != nil {
		log.Printf("Error while construction DHCPv6 ADVERTISE: Can't create Server DUID or Client DUID: %s", err)
		return err
	}

	options = append(options, incomingOpts...)

	if s.listenerConfig.AdvertiseUnicast {
		allowUnicast := layers.DHCPv6Option{
			Code: layers.DHCPv6OptUnicast,
			// Length is fixed by the serializer,
			Data: s.listenerConfig.ReplyFromAddress(),
		}
		options = append(options, allowUnicast)
	}

	advertise := layers.DHCPv6{
		MsgType:       layers.DHCPv6MsgTypeAdverstise,
		TransactionID: transactionID,
		HopCount:      0,
		Options:       options,
	}

	err = s.conn.WriteTo(advertise, dstIP, dstMAC)

	if err != nil {
		log.Printf("Can't send DHCPv6 ADVERTISE to '%s' ('%s'): %s", dstIP, dstMAC, err)
		return err
	}

	log.Printf("Sent a DHCPv6 ADVERTISE to '%s' ('%s')", dstIP, dstMAC)
	return nil
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

	// When the server receives a Request message via unicast from a client
	// to which the server has not sent a unicast option, the server
	// discards the Request message and responds with a Reply message
	// containing a Status Code option with the value UseMulticast, a Server
	// Identifier option containing the server's DUID, the Client Identifier
	// option from the client message, and no other options.
	// https://tools.ietf.org/html/rfc3315#section-18.2.1
	if !s.listenerConfig.AdvertiseUnicast && srcIP.IsGlobalUnicast() {
		options, err := s.serverAndClientIDOptions(rawClientDUID)
		if err != nil {
			log.Printf("Error constructing server DUID and/or client DUID: %s", err)
			return
		}

		options = append(options, statusOption(
			layers.DHCPv6StatusCodeUseMulticast, "the anycast option is not enabled"))

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
	_, found := optMap[layers.DHCPv6OptClientID]
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
func statusOption(statusCode layers.DHCPv6StatusCode, statusMessage string) layers.DHCPv6Option {
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
func (s *ServerV6) serverAndClientIDOptions(rawClientDUID []byte) (layers.DHCPv6Options, error) {
	serverDUID, err := s.dhcpConfig.ServerDUID()
	if err != nil {
		log.Printf("DHCPv6 Server DUID improperly configured: %s", err)
		return nil, err
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
	return options, nil
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
