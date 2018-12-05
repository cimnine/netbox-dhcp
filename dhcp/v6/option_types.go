package v6

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net"

	"github.com/google/gopacket/layers"
)

type iaOption struct {
	code layers.DHCPv6Opt
	data []byte
}

type iaAddress struct {
	addr              net.IP
	preferredLifetime uint32
	validLifetime     uint32
	statusCodeOption  statusCodeOption
	otherOptions      []iaOption
}

type statusCodeOption struct {
	statusCode    uint16
	statusMessage string
}

type IANontemporaryAddress struct {
	IAID             IAID
	T1               uint32
	T2               uint32
	AddressOptions   []iaAddress
	StatusCodeOption statusCodeOption
	OtherOptions     []iaOption
}

type IAID [4]byte

func (iaid IAID) String() string {
	return hex.EncodeToString(iaid[:])
}

func NewIAID(iaid string) (IAID, error) {
	bytes, err := hex.DecodeString(iaid)

	if err != nil {
		return IAID{}, err
	}

	if len(bytes) != 4 {
		return IAID{}, fmt.Errorf("'%s' does not decode to exactly 4 bytes", iaid)
	}

	return IAID{bytes[0], bytes[1], bytes[2], bytes[3]}, nil
}

// checkIANAs returns true if all addresses of the given IANA option are also in the clientInfo
func CheckIANA(iana IANontemporaryAddress, clientInfo ClientInfoV6) bool {
	for _, ipOption := range iana.AddressOptions {
		if !isIpInList(ipOption.addr, clientInfo.IPAddrs) {
			return false
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
func ParseIANAOption(ianaOpt layers.DHCPv6Option) IANontemporaryAddress {
	iana := IANontemporaryAddress{}
	copy(iana.IAID[:], ianaOpt.Data[0:4])

	addrOpt, statusOpt, otherOpt := parseIANASubOptions(ianaOpt.Data[12:])

	iana.AddressOptions = addrOpt
	iana.StatusCodeOption = statusOpt
	iana.OtherOptions = otherOpt

	return iana
}

func parseIANASubOptions(data []byte) ([]iaAddress, statusCodeOption, []iaOption) {
	if len(data) < 4 {
		return []iaAddress{}, statusCodeOption{}, []iaOption{}
	}

	code := layers.DHCPv6Opt(binary.BigEndian.Uint16(data[:2]))
	len := binary.BigEndian.Uint16(data[2:4])

	thisOptData := data[:4+len]
	rest := data[4+len:]

	iaAddresses, statusCodeOpt, iaOptions := parseIANASubOptions(rest)

	switch code {
	case layers.DHCPv6OptIAAddr:
		addrOpt := iaAddress{
			addr:              thisOptData[4:20],
			preferredLifetime: binary.BigEndian.Uint32(thisOptData[20:24]),
			validLifetime:     binary.BigEndian.Uint32(thisOptData[24:28]),
		}

		statusCodeOption, ok := findStatusCodeOpt(thisOptData[28:])
		if ok {
			addrOpt.statusCodeOption = statusCodeOption
		}

		iaAddresses = append(iaAddresses, addrOpt)
	case layers.DHCPv6OptStatusCode:
		statusCodeOpt = statusCodeOption{
			statusCode:    binary.BigEndian.Uint16(thisOptData[4:6]),
			statusMessage: string(thisOptData[6:]),
		}
	default:
		otherOpt := iaOption{
			code: code,
			data: thisOptData,
		}

		iaOptions = append(iaOptions, otherOpt)
	}

	return iaAddresses, statusCodeOpt, iaOptions
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
			statusCode:    binary.BigEndian.Uint16(data[4:6]),
			statusMessage: string(data[6 : 4+endOfOption]),
		}

		return option, true
	}

	return findStatusCodeOpt(data[endOfOption:])
}
