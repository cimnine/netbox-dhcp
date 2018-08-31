package v4

import (
	"errors"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/insomniacslk/dhcp/dhcpv4"
	"github.com/mdlayher/raw"
	"log"
	"net"
)

// This is the aprox. minimal size of a DHCP packet
const MinPackSize = 5*4 + // minimal TCP header size
	2*4 + // minimal UDP header size
	13*4 // minimal DHCP header size

type DHCPConn struct {
	conn  *raw.Conn
	iface net.Interface
	laddr net.IP
}

func ListenDHCPv4(iface net.Interface, laddr net.IP) (*DHCPConn, error) {
	conn, err := raw.ListenPacket(&iface, uint16(layers.EthernetTypeIPv4), &raw.Config{})

	return &DHCPConn{conn: conn, iface: iface, laddr: laddr}, err
}

func (c *DHCPConn) ReadFrom() (dhcpv4.DHCPv4, net.IP, net.HardwareAddr, error) {
	eth, ip4, _, p, err := c.readFrom()
	if err != nil {
		return dhcpv4.DHCPv4{}, nil, nil, err
	}

	dstIP := ip4.DstIP
	srcMAC := eth.SrcMAC
	pack, err := dhcpv4.FromBytes(p)

	if err != nil {
		return dhcpv4.DHCPv4{}, dstIP, srcMAC, err
	}

	return *pack, dstIP, srcMAC, nil
}

func (c *DHCPConn) WriteTo(pack dhcpv4.DHCPv4, dstIP net.IP, dstMAC net.HardwareAddr) error {
	serverIdentifier, ok := pack.GetOneOption(dhcpv4.OptionServerIdentifier).(*dhcpv4.OptServerIdentifier)
	if !ok {
		return errors.New("option ServerIdentifier undefined, illegal dhcp packet")
	}
	srcIP := serverIdentifier.ServerID
	srcMAC := c.iface.HardwareAddr

	p := pack.ToBytes()

	log.Printf("Sending DHCP%s (%d bytes) to %s (%s) from %s (%s)", pack.MessageType(), len(p), dstIP, dstMAC, srcIP, srcMAC)

	udp := layers.UDP{ // RFC 768
		SrcPort: dhcpv4.ServerPort,
		DstPort: dhcpv4.ClientPort,
		// Length is fixed by the serializer,
		// Checksum is fixed by the serializer,
	}

	ip4 := layers.IPv4{ // RFC 760
		Version: 4,
		// HeaderLength is fixed by the serializer,
		TOS: 0x0,
		// TotalLength is fixed by the serializer,
		Id:         0x00,
		Flags:      0x0,
		FragOffset: 0x00,
		TTL:        0x80,
		Protocol:   layers.IPProtocolUDP,
		// HeaderChecksum is fixed by the serializer,
		DstIP: dstIP,
		SrcIP: srcIP,
		// No Options,
		// No padding,
	}

	eth := layers.Ethernet{ // IEEE 802.3
		DstMAC:       dstMAC,
		SrcMAC:       srcMAC,
		EthernetType: layers.EthernetTypeIPv4,
		// Length is fixed by the serializer,
	}

	addr := raw.Addr{HardwareAddr: dstMAC}

	_, err := c.writeTo(&eth, &ip4, &udp, p, &addr)
	return err
}

func (c *DHCPConn) Close() error {
	return c.conn.Close()
}

func (c *DHCPConn) readFrom() (*layers.Ethernet, *layers.IPv4, *layers.UDP, []byte, error) {
	p := make([]byte, dhcpv4.MaxUDPReceivedPacketSize)

	for {
		l, _, err := c.conn.ReadFrom(p)
		if err != nil {
			return nil, nil, nil, nil, err
		}

		if l < MinPackSize {
			continue
		}

		// PERF: explore Lazy or NoCopy decode options to make parsing faster
		pack := gopacket.NewPacket(p[:l], layers.LayerTypeEthernet, gopacket.Default)

		ethLayer, ok := pack.Layer(layers.LayerTypeEthernet).(*layers.Ethernet)
		if !ok || ethLayer == nil {
			continue
		}
		if ethLayer.EthernetType != layers.EthernetTypeIPv4 {
			continue
		}

		ip4Layer := pack.Layer(layers.LayerTypeIPv4).(*layers.IPv4)
		if !ok || ip4Layer == nil {
			continue
		}
		if ip4Layer.Protocol != layers.IPProtocolUDP {
			continue
		}

		udpLayer := pack.Layer(layers.LayerTypeUDP).(*layers.UDP)
		if udpLayer == nil {
			continue
		}
		if udpLayer.DstPort != dhcpv4.ServerPort {
			continue
		}

		payload := udpLayer.Payload

		return ethLayer, ip4Layer, udpLayer, payload, nil
	}
}

func (c *DHCPConn) writeTo(eth *layers.Ethernet, ip4 *layers.IPv4, udp *layers.UDP, payload []byte, addr *raw.Addr) (int, error) {
	buf := gopacket.NewSerializeBuffer()
	opts := gopacket.SerializeOptions{
		ComputeChecksums: true,
		FixLengths:       true,
	}

	gopacket.Payload(payload).SerializeTo(buf, opts)
	udp.SerializeTo(buf, opts)
	ip4.SerializeTo(buf, opts)
	eth.SerializeTo(buf, opts)

	pack := buf.Bytes()

	//writeToPcap(pack)

	i, err := c.conn.WriteTo(pack, addr)

	return i, err
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
