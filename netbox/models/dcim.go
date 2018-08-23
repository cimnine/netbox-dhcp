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

type Device struct {
	NetboxObject
	Name string `json:"name"`
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

type EmbeddedInterface struct {
	EmbeddedNetboxObject
	Name           string                 `json:"name"`
	Device         EmbeddedDevice         `json:"device"`
	VirtualMachine EmbeddedVirtualMachine `json:"virtual_machine"`
}
