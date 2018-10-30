package models

import (
	"net"
)

type Prefix struct {
	NetboxCustomFieldsObject
	Family    uint8        `json:"family"`
	RawPrefix string       `json:"prefix"`
	Site      EmbeddedSite `json:"site"`
	IsPool    bool         `json:"is_pool"`
	Status    Status       `json:"status"`
}

func (p Prefix) Resolve() string {
	return "ipam/prefixes/{id}/"
}

func (ip Prefix) Prefix() (net.IP, *net.IPNet, error) {
	return net.ParseCIDR(ip.RawPrefix)
}

type PrefixList struct {
	NetboxList
	Prefixes []Prefix `json:"results"`
}

func (PrefixList) Resolve() string {
	return "ipam/prefixes/"
}

type EmbeddedIP struct {
	EmbeddedNetboxObject
	Family     uint8  `json:"family"`
	RawAddress string `json:"address"`
}

func (ip EmbeddedIP) Address() (net.IP, *net.IPNet, error) {
	return net.ParseCIDR(ip.RawAddress)
}

type IP struct {
	NetboxCustomFieldsObject
	Family     uint8             `json:"family"`
	RawAddress string            `json:"address"`
	Interface  EmbeddedInterface `json:"interface"`
}

func (ip IP) Resolve() string {
	return "ipam/ip-addresses/{id}/"
}

type IPList struct {
	NetboxList
	IPs []IP `json:"results"`
}

func (IPList) Resolve() string {
	return "ipam/ip-addresses/"
}

func (ip IP) Address() (net.IP, *net.IPNet, error) {
	return net.ParseCIDR(ip.RawAddress)
}
