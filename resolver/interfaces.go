package resolver

import (
	"github.com/cimnine/netbox-dhcp/dhcp/v4"
	"github.com/cimnine/netbox-dhcp/dhcp/v6"
)

type Solicitationer interface {
	SolicitationV6(info *v6.ClientInfoV6, clientID, clientMAC string) (bool, error)
}

type Offerer interface {
	OfferV4ByMAC(clientInfo *v4.ClientInfoV4, xid, mac string) error
	OfferV4ByID(clientInfo *v4.ClientInfoV4, xid, duid, iaid string) error
}

type Acknowledger interface {
	AcknowledgeV4ByMAC(clientInfo *v4.ClientInfoV4, xid, mac, ip string) error
	AcknowledgeV4ByID(clientInfo *v4.ClientInfoV4, xid, duid, iaid, ip string) error
}

type Decliner interface {
	DeclineV4ByMAC(xid, mac, ip string) error
	DeclineV4ByID(xid, duid, iaid, ip string) error
}

type Releaser interface {
	ReleaseV4ByMAC(xid, mac, ip string) error
	ReleaseV4ByID(xid, duid, iaid, ip string) error
}

type Resolver interface {
	Offerer
	Acknowledger
	Releaser
	Decliner
	Solicitationer
}
