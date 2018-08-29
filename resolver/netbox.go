package resolver

import (
	"errors"
	"fmt"
	"github.com/ninech/nine-dhcp2/netbox"
	"github.com/ninech/nine-dhcp2/netbox/models"
	"log"
)

type Netbox struct {
	Client *netbox.Client
}

func (n Netbox) OfferV4ByMAC(mac string) (info ClientInfoV4, err error) {
	iface, err := n.findInterfacesByMAC(mac)
	if err != nil {
		log.Printf("Can't find interface for MAC '%s'.", mac)
		return
	}

	ip, err := n.findIPAddressByInterfaceID(iface.ID)
	if err != nil {
		log.Printf("Can't find IP address for interface '%d' with MAC '%s'.", iface.ID, mac)
		return
	}

	address, net, err := ip.Address()
	if err != nil {
		return
	}

	ones, bits := net.Mask.Size()
	if bits == 0 {
		return info, fmt.Errorf("the netmask '%s' is skewed", net.Mask.String())
	}

	// TODO get further info from the device, to which the interface belongs
	// e.g. DNS server, router, etc.

	info.IPAddr = address
	info.PrefixLen = uint8(ones)

	return
}

func (n Netbox) findInterfacesByMAC(mac string) (iface models.Interface, err error) {
	ifaces, err := n.Client.FindInterfacesByMac(mac)
	if err != nil {
		log.Printf("Error while receiving interfaces for MAC '%s': %s", mac, err)
		return
	}

	if len(ifaces) == 0 {
		log.Printf("No interface with MAC '%s' found.", mac)
		return iface, fmt.Errorf("interface for MAC '%s' not found", mac)
	}

	if len(ifaces) > 1 {
		log.Printf("More than one interface with MAC '%s' found.", mac)
		return iface, fmt.Errorf("more than one interface with MAC '%s' found", mac)
	}

	return ifaces[0], nil
}

func (n Netbox) findIPAddressByInterfaceID(ifaceID uint64) (ip models.IP, err error) {
	ips, err := n.Client.FindIPAddressesByInterfaceID(ifaceID)

	if err != nil {
		log.Printf("Error while receiving ips for the interface '%d': %s", ifaceID, err)
		return
	}

	if len(ips) == 0 {
		log.Printf("No ip is associated with the interface '%d'.", ifaceID)
		return ip, fmt.Errorf("ip for interface '%d' not found", ifaceID)
	}

	if len(ips) > 1 {
		log.Printf("More than one ip is associated with the interface '%d'.", ifaceID)
		return ip, fmt.Errorf("more than one ip for interface '%d' found", ifaceID)
	}

	return ips[0], nil
}

func (n Netbox) OfferV4ByID(duid, iaid string) (ClientInfoV4, error) {
	return ClientInfoV4{}, errors.New("not yet implemented")
}
