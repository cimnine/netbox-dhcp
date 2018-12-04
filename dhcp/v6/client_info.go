package v6

import (
	"net"
	"time"
)

type ClientInfoV6 struct {
	Temporary bool
	IPAddrs   []net.IP
	IPMasks   []net.IPMask
	//NextServer   net.IP
	//BootFileName string
	Timeouts struct {
		Reservation     time.Duration
		Lease           time.Duration
		T1RenewalTime   time.Duration
		T2RebindingTime time.Duration
	}
	Options struct {
		HostName          string
		DomainName        string
		Routers           []net.IP
		DomainNameServers []net.IP
		NTPServers        []net.IP
	}
}
