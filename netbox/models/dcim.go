package models

import "fmt"

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
	return fmt.Sprintf("dcim/sites/%d", s.ID)
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
	Name string `json:"name"`
}

func (d Device) Resolve() string {
	return fmt.Sprintf("dcim/devices/%d/", d.ID)
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
	Name string `json:"name"`
}

func (i Interface) Resolve() string {
	return fmt.Sprintf("dcim/interfaces/%d/", i.ID)
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
