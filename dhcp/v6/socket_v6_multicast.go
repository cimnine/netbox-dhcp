package v6

import (
	"fmt"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/mdlayher/raw"
	"net"
)

const packetBufferSize = 1500

type MulticastV6Conn struct {
	conn   *raw.Conn
	active map[string]net.IP
}

//join a multicast group
func (c *MulticastV6Conn) JoinMulticast(multicastAddr net.IP) {
	if _, found := c.active[multicastAddr.String()]; found {
		// already joined this multicast group
		return
	}

	c.active[multicastAddr.String()] = multicastAddr

	//c.sendMLDv2(multicastAddr)
}

// leave a multicast group
func (c *MulticastV6Conn) LeaveMulticast(multicastAddr net.IP) {
	//c.sendMLDv2()

	if _, found := c.active[multicastAddr.String()]; !found {
		// not part of this multicast group
		return
	}

	delete(c.active, multicastAddr.String())
}

const MinPackSizeMLDv6 = 48

func (c *MulticastV6Conn) ReadFrom() (l int, pack gopacket.Packet, err error) {
	p := make([]byte, packetBufferSize)

	l, _, err = c.conn.ReadFrom(p)

	if l < MinPackSizeMLDv6 {
		return
	}

	ethLayer, ok := pack.Layer(layers.LayerTypeEthernet).(*layers.Ethernet)
	if !ok || ethLayer == nil {
		return
	}
	if ethLayer.EthernetType != layers.EthernetTypeIPv6 {
		return
	}

	ip6Layer := pack.Layer(layers.LayerTypeIPv6).(*layers.IPv6)
	if !ok || ip6Layer == nil {
		return
	}
	if ip6Layer.NextHeader != layers.IPProtocolICMPv6 {
		return
	}
	if !ip6Layer.DstIP.IsMulticast() {
		return
	}

	icmp6Layer, ok := pack.Layer(layers.LayerTypeICMPv6).(*layers.ICMPv6)
	if !ok || icmp6Layer == nil {
		return
	}
	if icmp6Layer.TypeCode != layers.ICMPv6TypeMLDv2MulticastListenerReportMessageV2 {
		return
	}

	//mldv2Layer := pack.Layer(layers.LayerTypeMLDv2MulticastListenerReport)

	// PERF: explore Lazy or NoCopy decode options to make parsing faster
	pack = gopacket.NewPacket(p[:l], layers.LayerTypeEthernet, gopacket.Default)

	return l, pack, err
}

func multicastDstMAC(multicastAddr net.IP) (net.HardwareAddr, error) {
	longIP := multicastAddr.To16()
	if longIP == nil {
		return nil, fmt.Errorf("'%s' is no valid IPv6", multicastAddr)
	}

	return net.HardwareAddr{0x33, 0x33, 0x00, longIP[13], longIP[14], longIP[15], longIP[16]}, nil
}
