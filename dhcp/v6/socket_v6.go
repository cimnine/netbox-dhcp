package v6

import (
	"errors"
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/mdlayher/raw"
	"log"
	"net"
	"regexp"
)

// This is the aprox. minimal size of a DHCP packet
const MinPackSizeDHCPv6 = 10*4 + // minimal IPv6 header size
	2*4 + // minimal UDP header size
	2*4 // minimal DHCPv6 header size

const DHCPv6ServerPort = 547
const DHCPv6ClientPort = 546

type DHCPV6Conn struct {
	//MulticastV6Conn
	conn   *raw.Conn
	iface  net.Interface
	daddrs []net.IP
	laddr  net.IP
}

// ListenDHCPv6 creates a connection that listens on the given interface for dhcpv6 traffic
// destined to the given address daddr,
// and sends replies from the given address laddr.
//
// daddr may be a multicast address (or any other valid IPv6 address).
// If daddr is ::0, then we'll listen to all DHCPv6 message on the interface.
//
// laddr must not be a multicast address, but may be any other valid IPv6 address (e.g. unicast or link-local address).
func ListenDHCPv6(iface net.Interface, daddrs []net.IP, laddr net.IP) (*DHCPV6Conn, error) {
	if laddr.IsMulticast() {
		return nil, fmt.Errorf("'%s' is a multicast address, but that's not allowed", laddr)
	}
	if laddr == nil { // reply from link local
		laddr = firstLinkLocalIPv6(iface)
	}
	if laddr == nil { // interface not ready
		return nil, fmt.Errorf("the interface '%s' does not have an IPv6 link local address", iface.Name)
	}
	laddr = laddr.To16()
	if laddr == nil {
		return nil, fmt.Errorf("'%s' is not a valid ipv6 address", laddr)
	}
	if len(daddrs) == 0 {
		return nil, errors.New("no valid address to listen on was found")
	}

	conn, err := raw.ListenPacket(&iface, uint16(layers.EthernetTypeIPv6), &raw.Config{})

	//return &DHCPV6Conn{MulticastV6Conn: MulticastV6Conn{conn: conn}, iface: iface, daddrs: daddrs, laddr: laddr.To16()}, err
	return &DHCPV6Conn{conn: conn, iface: iface, daddrs: daddrs, laddr: laddr.To16()}, err
}

// ReadFrom returns the parsed packet, source IP, source MAC, error
func (c *DHCPV6Conn) ReadFrom() (layers.DHCPv6, net.IP, net.HardwareAddr, error) {
	eth, ip6, _, dhcpv6, err := c.readFrom()
	if err != nil {
		return layers.DHCPv6{}, nil, nil, err
	}

	srcIP := ip6.SrcIP
	srcMAC := eth.SrcMAC

	if err != nil {
		return layers.DHCPv6{}, srcIP, srcMAC, err
	}

	return *dhcpv6, srcIP, srcMAC, nil
}

func (c *DHCPV6Conn) WriteTo(pack layers.DHCPv6, dstIP net.IP, dstMAC net.HardwareAddr) error {
	//log.Printf("Sending DHCP%s (%d bytes) to %s (%s) from %s (%s)", pack.Type(), len(p), dstIP, dstMAC, srcIP, srcMAC)

	udp := layers.UDP{ // RFC 768
		SrcPort: DHCPv6ServerPort,
		DstPort: DHCPv6ClientPort,
		// Length is fixed by the serializer,
		// Checksum is fixed by the serializer,
	}

	ip6 := layers.IPv6{ // RFC 760
		Version:      6,
		TrafficClass: 0x00,
		FlowLabel:    0x00000,
		// PayloadLength is fixed by the serializer,
		NextHeader: layers.IPProtocolUDP,
		HopLimit:   0x80,

		SrcIP: c.laddr,
		DstIP: dstIP,
	}

	eth := layers.Ethernet{ // IEEE 802.3
		DstMAC:       dstMAC,
		SrcMAC:       c.iface.HardwareAddr,
		EthernetType: layers.EthernetTypeIPv6,
		// Length is fixed by the serializer,
	}

	addr := raw.Addr{HardwareAddr: dstMAC}

	_, err := c.writeTo(&eth, &ip6, &udp, &pack, &addr)
	return err
}

func (c *DHCPV6Conn) Close() error {
	return c.conn.Close()
}

func (c *DHCPV6Conn) readFrom() (*layers.Ethernet, *layers.IPv6, *layers.UDP, *layers.DHCPv6, error) {
	p := make([]byte, 1500)

	for {
		//l, pack, err := c.MulticastV6Conn.ReadFrom()
		l, _, err := c.conn.ReadFrom(p)
		if err != nil {
			return nil, nil, nil, nil, err
		}

		if l < MinPackSizeDHCPv6 {
			continue
		}

		// PERF: explore Lazy or NoCopy decode options to make parsing faster
		pack := gopacket.NewPacket(p[:l], layers.LayerTypeEthernet, gopacket.Default)

		ethLayer, ok := pack.Layer(layers.LayerTypeEthernet).(*layers.Ethernet)
		if !ok || ethLayer == nil {
			continue
		}
		if ethLayer.EthernetType != layers.EthernetTypeIPv6 {
			continue
		}

		ip6Layer := pack.Layer(layers.LayerTypeIPv6).(*layers.IPv6)
		if !ok || ip6Layer == nil {
			continue
		}
		if ip6Layer.NextHeader != layers.IPProtocolUDP {
			continue
		}
		if !c.matchesDaddr(ip6Layer.DstIP) {
			continue
		}

		udpLayer := pack.Layer(layers.LayerTypeUDP).(*layers.UDP)
		if udpLayer == nil {
			continue
		}
		if udpLayer.DstPort != DHCPv6ServerPort {
			continue
		}

		dhcpv6Layer := pack.Layer(layers.LayerTypeDHCPv6).(*layers.DHCPv6)
		if dhcpv6Layer == nil {
			continue
		}

		return ethLayer, ip6Layer, udpLayer, dhcpv6Layer, nil
	}
}

// matchesDaddr checks if the given addr is in the list of
// all expected daddrs or if c.daddrs contains `::0`.
func (c *DHCPV6Conn) matchesDaddr(dstIP net.IP) bool {
	for _, daddr := range c.daddrs {
		if net.IPv6zero.Equal(daddr) {
			return true
		}

		return daddr.Equal(dstIP)
	}
	return false
}

func (c *DHCPV6Conn) writeTo(eth *layers.Ethernet, ip4 *layers.IPv6, udp *layers.UDP, dhcpv6 *layers.DHCPv6, addr *raw.Addr) (int, error) {
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}

	dhcpv6.SerializeTo(buf, opts)
	udp.SerializeTo(buf, opts)
	ip4.SerializeTo(buf, opts)
	eth.SerializeTo(buf, opts)

	pack := buf.Bytes()

	//writeToPcap(pack)

	i, err := c.conn.WriteTo(pack, addr)

	return i, err
}

// returns nil if no link-local IP is found or if there was an error in parsing it
func firstLinkLocalIPv6(iface net.Interface) net.IP {
	addrs, err := iface.Addrs()
	if err != nil {
		log.Printf("Error while getting IPs from interface '%s': %s", iface.Name, err)
	}

	for _, addr := range addrs {
		// who knows why, but the addr is string and not []byte 😪
		matched, err := regexp.MatchString("^(fe80::[a-z0-9:]+)", addr.String())
		if err == nil && matched {
			return net.ParseIP(addr.String())
		}
	}
	return nil
}

//func writeToPcap(pack []byte) {
//  f, _ := os.Create("/tmp/pack")
//  w := pcapgo.NewWriter(f)
//  w.WriteFileHeader(math.MaxUint32, layers.LinkTypeEthernet)
//  w.WritePacket(gopacket.CaptureInfo{
//    Length: len(pack),
//    CaptureLength: len(pack),
//    Timestamp: time.Now(),
//  }, pack)
//  f.Close()
//}
