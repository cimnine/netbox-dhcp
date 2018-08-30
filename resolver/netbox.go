package resolver

import (
	"fmt"
	"github.com/ninech/nine-dhcp2/netbox"
	"github.com/ninech/nine-dhcp2/netbox/models"
	"log"
	"net"
)

type Netbox struct {
	Client *netbox.Client
}

func (n Netbox) OfferV4ByMAC(mac string) (*ClientInfoV4, error) {
	address, netmask, device, err := n.findByInterfaceMAC(mac)
	if err != nil {
		log.Printf("Can't find IP via Interface for MAC '%s'. Trying via Device.", mac)
	}

	address, netmask, device, err = n.findByDeviceMAC(mac)
	if err != nil {
		log.Printf("Can't find IP via Device for MAC '%s'. Giving up.", mac)
		return nil, fmt.Errorf("no result for MAC '%s' in Netbox", mac)
	}

	info := ClientInfoV4{
		IPAddr: address,
		IPMask: netmask,
	}
	info.Options.HostName = device.Name
	info.Options.Routers = parseIP4s(device.ConfigContext.DHCP.Routers)
	info.Options.DomainName = device.ConfigContext.DHCP.DomainName
	info.Options.DomainNameServers = parseIP4s(device.ConfigContext.DHCP.DNSServers)
	info.Options.NTPServers = parseIP4s(device.ConfigContext.DHCP.NTPServers)

	return &info, nil
}

func parseIP4s(ipStrs []string) []net.IP {
	ips := make([]net.IP, len(ipStrs))

	for _, router := range ipStrs {
		ip := net.ParseIP(router)
		if ip == nil {
			continue
		}

		ip4 := ip.To4()
		if ip == nil {
			continue
		}

		ips = append(ips, ip4)
	}
	return ips
}

func (n Netbox) findByDeviceMAC(mac string) (net.IP, net.IPMask, models.Device, error) {
	emptyDevice := models.Device{}

	device, err := n.findDeviceByMAC(mac)
	if err != nil {
		log.Printf("Can't find Device for MAC '%s'", mac)
		return nil, nil, emptyDevice, err
	}

	if device.PrimaryIP4.ID == 0 { // empty object
		log.Printf("The Device '%d' does not defined a primary IPv4.", device.ID)
		return nil, nil, emptyDevice, err
	}

	address, network, err := device.PrimaryIP4.Address()
	if err != nil {
		return nil, nil, emptyDevice, err
	}

	return address, network.Mask, device, nil
}

func (n Netbox) findByInterfaceMAC(mac string) (net.IP, net.IPMask, models.Device, error) {
	emptyDevice := models.Device{}

	iface, err := n.findInterfacesByMAC(mac)
	if err != nil {
		log.Printf("Can't find interface for MAC '%s'", mac)
		return nil, nil, emptyDevice, err
	}

	ip, err := n.findIPAddressByInterfaceID(iface.ID)
	if err != nil {
		log.Printf("Can't find IP address for interface '%d' with MAC '%s'", iface.ID, mac)
		return nil, nil, emptyDevice, err
	}

	address, network, err := ip.Address()
	if err != nil {
		return nil, nil, emptyDevice, err
	}

	device, err := n.findDeviceByID(iface.Device.ID)
	if err != nil {
		return nil, nil, emptyDevice, err
	}

	return address, network.Mask, device, nil
}

func (n Netbox) findInterfacesByMAC(mac string) (iface models.Interface, err error) {
	ifaces, err := n.Client.FindInterfacesByMAC(mac)
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

func (n Netbox) findIPAddressByID(ipID uint64) (ip models.IP, err error) {
	ipPtr, err := n.Client.GetIPAddressByID(ipID)

	if err != nil {
		log.Printf("Error while receiving IP with ID '%d'", ipID)
		return
	}

	if ipPtr == nil {
		log.Printf("IP with ID %d not found", ipID)
		return ip, fmt.Errorf("IP %d not found", ipID)
	}

	return *ipPtr, nil
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

func (n Netbox) findDeviceByMAC(mac string) (device models.Device, err error) {
	devices, err := n.Client.FindDevicesByMAC(mac)

	if err != nil {
		log.Printf("Error while receiving devices with the MAC '%s'", mac)
		return
	}

	if len(devices) != 1 {
		log.Printf("Expected exactly one Device with the MAC '%s', but found %d.", mac, len(devices))
		return device, fmt.Errorf("found %d devices for the MAC '%s', expected one", len(devices), mac)
	}

	return devices[0], nil
}

func (n Netbox) findDeviceByID(deviceID uint64) (device models.Device, err error) {
	devicePtr, err := n.Client.GetDeviceByID(deviceID)

	if err != nil {
		log.Printf("Error while receiving Device with ID '%d'", deviceID)
		return
	}

	if devicePtr == nil {
		log.Printf("Device with ID %d not found", deviceID)
		return device, fmt.Errorf("device %d not found", deviceID)
	}

	return *devicePtr, nil
}

func (n Netbox) OfferV4ByID(duid, iaid string) (*ClientInfoV4, error) {
	panic("not implemented")
}
