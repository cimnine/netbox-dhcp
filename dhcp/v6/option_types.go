package v6

import (
	"encoding/binary"
	"net"

	"github.com/google/gopacket/layers"
)

type iaNontemporaryAddress struct {
	iaid             [4]byte
	t1               uint32
	t2               uint32
	addressOptions   []iaAddress
	statusCodeOption statusCodeOption
	otherOptions     []iaOptions
}

type iaOptions struct {
	code uint16
	data []byte
}

type iaAddress struct {
	addr              net.IP
	preferredLifetime uint32
	validLifetime     uint32
	statusCodeOption  statusCodeOption
	otherOptions      []iaOptions
}

type statusCodeOption struct {
	len           uint16
	statusCode    uint16
	statusMessage string
}

// checkIANAs returns true if all addresses of the given IANA option are also in the clientInfo
func CheckIANAs(ianaOpts layers.DHCPv6Options, clientInfo ClientInfoV6) bool {
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
