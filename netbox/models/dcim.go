package models

type EmbeddedSite struct {
	EmbeddedNetboxObject
	Name string `json:"name"`
	Slug string `json:"slug"`
}

type Site struct {
	NetboxCustomFieldsObject
	Name     string `json:"name"`
	Slug     string `json:"slug"`
	TimeZone string `json:"time_zone"`
	Status   Status `json:"status"`
}

func (s Site) Resolve() string {
	return "dcim/sites/{id}/"
}

type SiteList struct {
	NetboxList
	Sites []Site `json:"results"`
}

func (SiteList) Resolve() string {
	return "dcim/sites/"
}

type Device struct {
	NetboxObject
	Name          string     `json:"name"`
	PrimaryIP4    EmbeddedIP `json:"primary_ip4"`
	PrimaryIP6    EmbeddedIP `json:"primary_ip6"`
	ConfigContext struct {
		DHCP DHCPConfigContext `json:"dhcp"`
	} `json:"config_context"`
}

func (d Device) Resolve() string {
	return "dcim/devices/{id}/"
}

type DeviceList struct {
	NetboxList
	Devices []Device `json:"results"`
}

func (DeviceList) Resolve() string {
	return "dcim/devices/"
}

type EmbeddedVirtualMachine EmbeddedDevice
type EmbeddedDevice struct {
	EmbeddedNetboxObject
	Name        string `json:"name"`
	DisplayName string `json:"display_name"`
}

type Interface struct {
	NetboxObject
	Device EmbeddedDevice `json:"device"`
	Name   string         `json:"name"`
}

func (i Interface) Resolve() string {
	return "dcim/interfaces/{id}/"
}

type InterfaceList struct {
	NetboxList
	Interfaces []Interface `json:"results"`
}

func (InterfaceList) Resolve() string {
	return "dcim/interfaces/"
}

type EmbeddedInterface struct {
	EmbeddedNetboxObject
	Name           string                 `json:"name"`
	Device         EmbeddedDevice         `json:"device"`
	VirtualMachine EmbeddedVirtualMachine `json:"virtual_machine"`
}

type DHCPConfigContext struct {
	Routers    []string `json:"routers"`
	DomainName string   `json:"domain_name"`
	DNSServers []string `json:"dns_servers"`
	NTPServers []string `json:"ntp_servers"`
}
