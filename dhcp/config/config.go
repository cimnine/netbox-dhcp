package config

import "net"

type DHCPConfig struct {
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
