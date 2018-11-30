package resolver

import (
	"fmt"
	"log"
	"net"
	"time"

	"github.com/cimnine/netbox-dhcp/dhcp/v4"
	"github.com/cimnine/netbox-dhcp/netbox"
	"github.com/cimnine/netbox-dhcp/netbox/models"
	"github.com/cimnine/netbox-dhcp/util"
)

type Netbox struct {
	Client *netbox.Client
}

func (n Netbox) SolicitationV6(clientID, clientMAC string) (bool, error) {
	_, err := n.findDeviceByDUID(clientID)
	if err == nil {
		return true, nil
	}

	log.Printf("Can't find a Device for client ID '%s'. Trying with MAC.", clientID)

	_, _, _, err = n.findByInterfaceMAC(clientMAC)
	if err == nil {
		return true, nil
	}

	log.Printf("Can't find an Interface for MAC '%s'. Trying via Device.", clientMAC)

	_, _, _, err = n.findByDeviceMAC(clientMAC)
	if err == nil {
		return true, nil
	}

	log.Printf("Can't find an Interface or a Device for client ID '%s' / MAC '%s'. Giving up.", clientID, clientMAC)
	return false, fmt.Errorf("no result for client ID '%s' / MAC '%s' in Netbox", clientID, clientMAC)
}

func (n Netbox) OfferV4ByMAC(info *v4.ClientInfoV4, transactionID, mac string) error {
	address, netmask, device, err := n.findByInterfaceMAC(mac)
	if err == nil {
		fillClientInfo(info, address, netmask, device)
		return nil
	}

	log.Printf("Can't find IPv4 via Interface for MAC '%s'. Trying via Device.", mac)

	address, netmask, device, err = n.findByDeviceMAC(mac)
	if err == nil {
		fillClientInfo(info, address, netmask, device)
		return nil
	}

	log.Printf("Can't find IPv4 via Device for MAC '%s'. Giving up.", mac)
	return fmt.Errorf("no result for MAC '%s' in Netbox", mac)
}

func (n Netbox) OfferV4ByID(info *v4.ClientInfoV4, transactionID, duid, iaid string) error {
	panic("please implement")
}

func fillClientInfo(info *v4.ClientInfoV4, address net.IP, netmask net.IPMask, device models.Device) {
	info.IPAddr = address
	info.IPMask = netmask

	bootFileName := device.ConfigContext.DHCP.BootFileName
	if bootFileName != "" {
		info.BootFileName = bootFileName
	}

	nextServer := net.ParseIP(device.ConfigContext.DHCP.NextServer)
	if nextServer != nil {
		info.NextServer = nextServer
	}

	hostName := device.Name
	if hostName != "" {
		info.Options.HostName = hostName
	}

	routers := util.ParseIP4s(device.ConfigContext.DHCP.Routers)
	if len(routers) > 0 {
		info.Options.Routers = routers
	}

	domainName := device.ConfigContext.DHCP.DomainName
	if domainName != "" {
		info.Options.DomainName = domainName
	}

	dnsServers := util.ParseIP4s(device.ConfigContext.DHCP.DNSServers)
	if len(dnsServers) > 0 {
		info.Options.DomainNameServers = dnsServers
	}

	ntpServers := util.ParseIP4s(device.ConfigContext.DHCP.NTPServers)
	if len(ntpServers) > 0 {
		info.Options.NTPServers = ntpServers
	}

	leaseDurationStr := device.ConfigContext.DHCP.LeaseDuration
	leaseDuration, err := time.ParseDuration(leaseDurationStr)
	if err != nil && leaseDurationStr != "" {
		info.Timeouts.Lease = leaseDuration
	}
}

func (n Netbox) findByDeviceMAC(mac string) (net.IP, net.IPMask, models.Device, error) {
	emptyDevice := models.Device{}

	device, err := n.findDeviceByMAC(mac)
	if err != nil {
		log.Printf("Can't find Device for MAC '%s'", mac)
		return nil, nil, emptyDevice, err
	}

	if device.PrimaryIP4.ID == 0 { // empty object
		log.Printf("The Device with ID %d does not defined a primary IPv4.", device.ID)
		return nil, nil, emptyDevice, fmt.Errorf("device %d has no primary IPv4", device.ID)
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

func (n Netbox) findDeviceByID(id uint64) (device models.Device, err error) {
	devicePtr, err := n.Client.GetDeviceByID(id)

	if err != nil {
		log.Printf("Error while receiving Device with ID '%d'", id)
		return
	}

	if devicePtr == nil {
		log.Printf("Device with ID '%d' not found", id)
		return device, fmt.Errorf("device not found by ID '%d'", id)
	}

	return *devicePtr, nil
}

func (n Netbox) findDeviceByDUID(duid string) (device models.Device, err error) {
	devicePtr, err := n.Client.GetDeviceByDUID(duid)

	if err != nil {
		log.Printf("Error while receiving Device with DUID '%s'", duid)
		return
	}

	if devicePtr == nil {
		log.Printf("Device with DUID '%s' not found", duid)
		return device, fmt.Errorf("device not found by DUID '%s'", duid)
	}

	return *devicePtr, nil
}
