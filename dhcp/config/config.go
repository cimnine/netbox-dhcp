package config

import (
	"encoding/binary"
	"fmt"
	"github.com/ninech/nine-dhcp2/dhcp/v6/consts"
	"github.com/satori/go.uuid"
	"log"
	"net"
	"strings"
)

type DHCPConfig struct {
	ServerUUID          string `yaml:"server_uuid"`
	ReservationDuration string `yaml:"reservation_duration"`
	LeaseDuration       string `yaml:"lease_duration"`
	T1Duration          string `yaml:"t1_duration"`
	T2Duration          string `yaml:"t2_duration"`
	DefaultOptions      struct {
		NextServer        string   `yaml:"next_server"`
		BootFileName      string   `yaml:"bootfile_name"`
		DomainName        string   `yaml:"domain_name"`
		DomainNameServers []string `yaml:"dns_servers"`
		NTPServers        []string `yaml:"ntp_servers"`
		Routers           []string `yaml:"routers"`
	} `yaml:"default_options"`
}

// ServerDUID returns the server's DUID formatted according to
// https://tools.ietf.org/html/rfc6355#section-4
func (d DHCPConfig) ServerDUID() ([]byte, error) {
	u, err := uuid.FromString(d.ServerUUID)
	if err != nil {
		log.Printf("Can't parse '%s' as UUID: %s", d.ServerUUID, err)
		return nil, fmt.Errorf("invalid UUID '%s'", d.ServerUUID)
	}

	buf := make([]byte, 18)
	binary.BigEndian.PutUint16(buf[:2], uint16(consts.DHCPv6DUIDTypeUUID))
	copy(buf[2:], u.Bytes())
	return buf, nil
}

type DaemonConfig struct {
	Daemonize bool
	Log       struct {
		Level string
		Path  string
	}
	ListenV4 map[string]V4ListenerConfig `yaml:"listen_v4"`
	ListenV6 map[string]V6ListenerConfig `yaml:"listen_v6"`
}

type V4ListenerConfig struct {
	ReplyFrom     string `yaml:"reply_from"`
	ReplyHostname string `yaml:"reply_hostname"`
}

func (v *V4ListenerConfig) ReplyFromAddress() net.IP {
	return net.ParseIP(v.ReplyFrom)
}

type V6ListenerConfig struct {
	AdvertiseUnicast bool     `yaml:"advertise_unicast"`
	ListenTo         []string `yaml:"listen_to"`
	ReplyFrom        string   `yaml:"reply_from"`
}

func (v *V6ListenerConfig) ReplyFromAddress() net.IP {
	return net.ParseIP(v.ReplyFrom)
}

func (v *V6ListenerConfig) ListenToAddresses() []net.IP {
	ipAddrs := make([]net.IP, 0)
	for _, addr := range v.ListenTo {
		if strings.EqualFold("All_DHCP_Relay_Agents_and_Servers", addr) {
			addr = "FF02::1:2"
		} else if strings.EqualFold("All_DHCP_Servers", addr) {
			addr = "FF05::1:3"
		}

		ip := net.ParseIP(addr).To16()
		if ip != nil {
			ipAddrs = append(ipAddrs, ip)
		} else {
			log.Printf("Can't parse ip '%s'. Make sure it's a valid IPv6 address.", addr)
		}
	}

	return ipAddrs
}
