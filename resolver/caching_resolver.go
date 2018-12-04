package resolver

import (
	"github.com/cimnine/netbox-dhcp/dhcp/v6"
	"log"

	"github.com/cimnine/netbox-dhcp/dhcp/v4"
)

// A Sourcer assigns IPs based on a request
type Sourcer interface {
	Offerer
	Solicitationer
}

// A Cacher keeps records of leased IPs
type Cacher interface {
	Acknowledger
	Releaser
	ReserveV4(info *v4.ClientInfoV4, xid string) error
}

// Source and Cache are two independent implementations and are interchangeable
type CachingResolver struct {
	Source Sourcer
	Cache  Cacher
}

func (r CachingResolver) SolicitationV6(info *v6.ClientInfoV6, clientID, clientMAC string) (bool, error) {
	ok, err := r.Source.SolicitationV6(info, clientID, clientMAC)

	if err != nil {
		return false, err
	}

	// TODO cache

	return ok, nil
}

func (r CachingResolver) DeclineV4ByMAC(xid, mac, ip string) error {
	// This strictly speaking violates RFC2131 Section 4.3.3.
	// But the source should only hand out IPs that are not yet taken anyway.
	// At least as long as ip pools are not yet implemented.
	log.Printf("The CachingResolver can't handle declines.")
	return nil
}

func (r CachingResolver) DeclineV4ByID(xid, duid, iaid, ip string) error {
	// This strictly speaking violates violates RFC2131 Section 4.3.3.
	// But the source should only hand out IPs that are not yet taken anyway.
	// At least as long as ip pools are not yet implemented.
	log.Printf("The CachingResolver can't handle declines according to the RFC.")
	return nil
}

func (r CachingResolver) ReleaseV4ByMAC(xid, mac, ip string) error {
	return r.Cache.ReleaseV4ByMAC(xid, mac, ip)
}

func (r CachingResolver) ReleaseV4ByID(xid, duid, iaid, ip string) error {
	return r.Cache.ReleaseV4ByID(xid, duid, iaid, ip)
}

func (r CachingResolver) OfferV4ByMAC(info *v4.ClientInfoV4, xid, mac string) error {
	err := r.Source.OfferV4ByMAC(info, xid, mac)
	if err != nil {
		// TODO log message
		return err
	}

	err = r.Cache.ReserveV4(info, xid)
	if err != nil {
		// TODO log message
		return err
	}

	return nil
}

func (r CachingResolver) OfferV4ByID(info *v4.ClientInfoV4, xid, duid, iaid string) error {
	err := r.Source.OfferV4ByID(info, xid, duid, iaid)
	if err != nil {
		// TODO log message
		return err
	}

	err = r.Cache.ReserveV4(info, xid)
	if err != nil {
		// TODO log message
		return err
	}

	return nil
}

func (r CachingResolver) AcknowledgeV4ByMAC(info *v4.ClientInfoV4, xid, mac, ip string) error {
	return r.Cache.AcknowledgeV4ByMAC(info, xid, mac, ip)
}

func (r CachingResolver) AcknowledgeV4ByID(info *v4.ClientInfoV4, xid, duid, iaid, ip string) error {
	return r.Cache.AcknowledgeV4ByID(info, xid, duid, iaid, ip)
}
