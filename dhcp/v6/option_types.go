package v6

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"net"

	"github.com/google/gopacket/layers"
)

type iaOptions []iaOption

func (iaos iaOptions) encodeTo(buf *bytes.Buffer) (int, error) {
	n := 0

	for _, iao := range iaos {
		m, err := iao.encodeTo(buf)
		n += m
		if err != nil {
			return n, err
		}
	}

	return n, nil
}

type iaOption struct {
	code layers.DHCPv6Opt
	data []byte
}

func (iao iaOption) encodeTo(buf *bytes.Buffer) (int, error) {
	b := make([]byte, 4)
	binary.BigEndian.PutUint16(b[0:2], uint16(iao.code))
	binary.BigEndian.PutUint16(b[2:4], uint16(len(iao.data)))

	n, err := buf.Write(b)
	if err != nil {
		return n, err
	}

	m, err := buf.Write(iao.data)
	return n + m, err
}

type statusCodeOption struct {
	code    layers.DHCPv6StatusCode
	message string
}

func (sco statusCodeOption) encodeTo(buf *bytes.Buffer) (int, error) {
	b := make([]byte, 4)
	binary.BigEndian.PutUint16(b[0:2], uint16(sco.code))
	binary.BigEndian.PutUint16(b[2:4], uint16(len(sco.message)))

	n, err := buf.Write(b)
	if err != nil {
		return n, err
	}

	m, err := buf.WriteString(sco.message)
	return n + m, err
}

type iaAddresses []iaAddress

func (iaas iaAddresses) encodeTo(buf *bytes.Buffer) (int, error) {
	n := 0

	for _, iaa := range iaas {
		m, err := iaa.encodeTo(buf)
		n += m
		if err != nil {
			return n, err
		}
	}

	return n, nil
}

type iaAddress struct {
	addr              net.IP
	preferredLifetime uint32
	validLifetime     uint32
	statusCodeOption  statusCodeOption
	otherOptions      iaOptions
}

func (iaa iaAddress) encodeTo(buf *bytes.Buffer) (int, error) {
	optionBuf := new(bytes.Buffer)
	_, err := iaa.statusCodeOption.encodeTo(optionBuf)
	if err != nil {
		return 0, err
	}
	_, err = iaa.otherOptions.encodeTo(optionBuf)
	if err != nil {
		return 0, err
	}

	b := make([]byte, 28)
	binary.BigEndian.PutUint16(b[0:2], uint16(layers.DHCPv6OptIAAddr))
	binary.BigEndian.PutUint16(b[2:4], uint16(optionBuf.Len()))

	n, err := buf.Write(b)
	if err != nil {
		return n, err
	}

	m, err := buf.Write(optionBuf.Bytes())
	return n + m, err
}

type IANontemporaryAddress struct {
	IAID             IAID
	T1               uint32
	T2               uint32
	AddressOptions   iaAddresses
	StatusCodeOption statusCodeOption
	OtherOptions     iaOptions
}

func (iana IANontemporaryAddress) encodeDataTo(buf *bytes.Buffer) (int, error) {
	b := make([]byte, 12)
	copy(b[0:4], iana.IAID[:])
	binary.BigEndian.PutUint32(b[4:8], iana.T1)
	binary.BigEndian.PutUint32(b[8:12], iana.T2)

	n, err := buf.Write(b)
	if err != nil {
		return n, err
	}

	m, err := iana.AddressOptions.encodeTo(buf)
	n += m
	if err != nil {
		return n, err
	}

	m, err = iana.StatusCodeOption.encodeTo(buf)
	n += m
	if err != nil {
		return n, err
	}

	m, err = iana.OtherOptions.encodeTo(buf)
	n += m
	if err != nil {
		return n, err
	}

	return n, nil
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
		log.Printf("Client asked for IP '%s'", ipOption.addr) // TODO remove
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

// EncodeOptions encodes the IP
func EncodeOptions(iaid IAID, info ClientInfoV6) ([]byte, error) {
	iaas := make(iaAddresses, len(info.IPAddrs))

	for i, ip := range info.IPAddrs {
		iaas[i] = iaAddress{
			addr:              ip,
			preferredLifetime: uint32(info.Timeouts.PreferredLifetime.Seconds()),
			validLifetime:     uint32(info.Timeouts.ValidLifetime.Seconds()),
			statusCodeOption:  statusCodeOption{code: layers.DHCPv6StatusCodeSuccess},
		}
	}

	iana := IANontemporaryAddress{
		IAID: iaid,
		T1: uint32(info.Timeouts.T1RenewalTime.Seconds()),
		T2: uint32(info.Timeouts.T2RebindingTime.Seconds()),
		AddressOptions: iaas,
		StatusCodeOption: statusCodeOption{code: layers.DHCPv6StatusCodeSuccess},
	}

	buf := new(bytes.Buffer)
	_, err := iana.encodeDataTo(buf)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
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
			code:    layers.DHCPv6StatusCode(binary.BigEndian.Uint16(thisOptData[4:6])),
			message: string(thisOptData[6:]),
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
			code:    layers.DHCPv6StatusCode(binary.BigEndian.Uint16(data[4:6])),
			message: string(data[6 : 4+endOfOption]),
		}

		return option, true
	}

	return findStatusCodeOpt(data[endOfOption:])
}
