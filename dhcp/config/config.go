package config

type DaemonConfig struct {
	Daemonize bool
	Log       struct {
		Level string
		Path  string
	}
	ListenV4 map[string][]string `yaml:"listen_v4"`
	ListenV6 map[string][]string `yaml:"listen_v6"`
}

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
