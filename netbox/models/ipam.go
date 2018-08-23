package models

import "net"

type Prefix struct {
	NetboxCustomFieldsObject
	Family    uint8        `json:"family"`
	RawPrefix string       `json:"prefix"`
	Site      EmbeddedSite `json:"site"`
	IsPool    bool         `json:"is_pool"`
	Status    Status       `json:"status"`
}

type EmbeddedIP struct {
	EmbeddedNetboxObject
	Family     uint8  `json:"family"`
	RawAddress string `json:"address"`
}

type IP struct {
	NetboxCustomFieldsObject
	Family     uint8             `json:"family"`
	RawAddress string            `json:"address"`
	Interface  EmbeddedInterface `json:"interface"`
}

func (ip IP) Address() (net.IP, *net.IPNet, error) {
	return net.ParseCIDR(ip.RawAddress)
}

func (ip EmbeddedIP) Address() (net.IP, *net.IPNet, error) {
	return net.ParseCIDR(ip.RawAddress)
}

func (ip Prefix) Prefix() (net.IP, *net.IPNet, error) {
	return net.ParseCIDR(ip.RawPrefix)
}
