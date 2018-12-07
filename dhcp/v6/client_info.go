package v6

import (
	"net"
	"time"
)

type ClientInfoV6 struct {
	Temporary bool
	IPAddrs   []net.IP
	//NextServer   net.IP
	//BootFileName string
	Timeouts struct {
		ValidLifetime     time.Duration
		PreferredLifetime time.Duration
		T1RenewalTime     time.Duration
		T2RebindingTime   time.Duration
	}
	Options struct {
		HostName          string
		DomainName        string
		DomainNameServers []net.IP
		NTPServers        []net.IP
	}
}
