package resolver

import (
	"net"
	"time"
)

type Offerer interface {
	OfferV4ByMAC(mac string) (*ClientInfoV4, error)
	OfferV4ByID(duid, iaid string) (*ClientInfoV4, error)

	//OfferV6ByMac(mac string) (ClientInfoV6, error)
	//OfferV6ById(duid string, iuid string) (ClientInfoV6, error)
}

type Acknowledger interface {
	AcknowledgeV4ByMAC(mac, ip string) (*ClientInfoV4, error)
	AcknowledgeV4ByID(duid, iaid, ip string) (*ClientInfoV4, error)
}

type CachingRequester interface {
	Acknowledger
	ReserveV4ByMAC(mac string, info *ClientInfoV4) error
	ReserveV4ByID(duid, iaid string, info *ClientInfoV4) error
}

type Resolver interface {
	Offerer
	Acknowledger
}

type ClientInfoV4 struct {
	IPAddr       net.IP
	PrefixLen    uint8
	BootFileName string
	Timeouts     struct {
		Reservation time.Duration
		Lease       time.Duration
		T1          time.Duration
		T2          time.Duration
	}
	Options struct {
		HostName          string
		DomainName        string
		Routers           []net.IP
		DomainNameServers []net.IP
		NTPServers        []net.IP
	}
}
