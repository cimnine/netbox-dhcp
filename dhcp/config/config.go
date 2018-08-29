package config

import "net"

type DHCPConfig struct {
	LeaseTimeout   string `yaml:"lease_timeout"`
	T1Timeout      string `yaml:"t1_timeout"`
	T2Timeout      string `yaml:"t2_timeout"`
	DefaultOptions struct {
		DomainName string   `yaml:"domain_name"`
		DNSServers []string `yaml:"dns_servers"`
		NTPServers []string `yaml:"ntp_servers"`
	} `yaml:"default_options"`
}

type DaemonConfig struct {
	Daemonize bool
	Log       struct {
		Level string
		Path  string
	}
	ListenV4 map[string]V4InterfaceConfig `yaml:"listen_v4"`
	ListenV6 map[string]V6InterfaceConfig `yaml:"listen_v6"`
}

type V4InterfaceConfig struct {
	ReplyFrom     string `yaml:"reply_from"`
	ReplyHostname string `yaml:"reply_hostname"`
}

func (v *V4InterfaceConfig) ReplyFromAddress() net.IP {
	return net.ParseIP(v.ReplyFrom)
}

type V6InterfaceConfig struct {
	AdvertiseUnicast bool   `yaml:"advertise_unicast"`
	ReplyFrom        string `yaml:"reply_from"`
}

func (v *V6InterfaceConfig) ReplyFromAddress() net.IP {
	return net.ParseIP(v.ReplyFrom)
}
