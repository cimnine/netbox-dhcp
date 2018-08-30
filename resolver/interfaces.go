package resolver

import (
	"net"
	"time"
)

type Offerer interface {
	OfferV4ByMAC(clientInfo *ClientInfoV4, mac string) error
	OfferV4ByID(clientInfo *ClientInfoV4, duid, iaid string) error
}

type Acknowledger interface {
	AcknowledgeV4ByMAC(clientInfo *ClientInfoV4, mac, ip string) error
	AcknowledgeV4ByID(clientInfo *ClientInfoV4, duid, iaid, ip string) error
}

type CachingRequester interface {
	Acknowledger
	ReserveV4ByMAC(info *ClientInfoV4, mac string) error
	ReserveV4ByID(info *ClientInfoV4, duid, iaid string) error
}

type Resolver interface {
	Offerer
	Acknowledger
}

type ClientInfoV4 struct {
	IPAddr       net.IP
	IPMask       net.IPMask
	NextServer   net.IP
	BootFileName string
	Timeouts     struct {
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
