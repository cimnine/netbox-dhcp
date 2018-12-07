package resolver

import (
	"github.com/cimnine/netbox-dhcp/dhcp/v6"
	"net"
	"time"

	"github.com/cimnine/netbox-dhcp/dhcp/config"
	"github.com/cimnine/netbox-dhcp/dhcp/v4"
	"github.com/cimnine/netbox-dhcp/util"
)

func NewClientInfoV4(dhcpConfig *config.DHCPConfig) *v4.ClientInfoV4 {
	info := v4.ClientInfoV4{
		NextServer:   net.ParseIP(dhcpConfig.DefaultOptions.NextServer),
		BootFileName: dhcpConfig.DefaultOptions.BootFileName,
	}

	d, err := time.ParseDuration(dhcpConfig.ReservationDuration)
	if err != nil {
		info.Timeouts.Reservation = 1 * time.Minute
	} else {
		info.Timeouts.Reservation = d
	}

	d, err = time.ParseDuration(dhcpConfig.LeaseDuration)
	if err != nil {
		info.Timeouts.Lease = 6 * time.Hour
	} else {
		info.Timeouts.Lease = d
	}

	d, err = time.ParseDuration(dhcpConfig.T2Duration)
	if err != nil {
		info.Timeouts.T2RebindingTime = info.Timeouts.Lease / 2
	} else {
		info.Timeouts.T2RebindingTime = d
	}

	d, err = time.ParseDuration(dhcpConfig.T1Duration)
	if err != nil {
		info.Timeouts.T1RenewalTime = info.Timeouts.T2RebindingTime / 2
	} else {
		info.Timeouts.T1RenewalTime = d
	}

	info.Options.DomainName = dhcpConfig.DefaultOptions.DomainName
	info.Options.DomainNameServers = util.ParseIP4s(dhcpConfig.DefaultOptions.DomainNameServers)
	info.Options.NTPServers = util.ParseIP4s(dhcpConfig.DefaultOptions.NTPServers)
	info.Options.Routers = util.ParseIP4s(dhcpConfig.DefaultOptions.Routers)

	return &info
}

func NewClientInfoV6(dhcpConfig *config.DHCPConfig) v6.ClientInfoV6 {
	info := v6.ClientInfoV6{
		//NextServer:   net.ParseIP(dhcpConfig.DefaultOptions.NextServer),
		//BootFileName: dhcpConfig.DefaultOptions.BootFileName,
	}

	// TODO figure the correct default values out
	d, err := time.ParseDuration(dhcpConfig.ReservationDuration)
	if err != nil {
		info.Timeouts.PreferredLifetime = 1 * time.Minute
	} else {
		info.Timeouts.PreferredLifetime = d
	}

	// TODO figure the correct default values out
	d, err = time.ParseDuration(dhcpConfig.LeaseDuration)
	if err != nil {
		info.Timeouts.ValidLifetime = 6 * time.Hour
	} else {
		info.Timeouts.ValidLifetime = d
	}

	d, err = time.ParseDuration(dhcpConfig.T2Duration)
	if err != nil {
		info.Timeouts.T2RebindingTime = info.Timeouts.ValidLifetime / 2
	} else {
		info.Timeouts.T2RebindingTime = d
	}

	d, err = time.ParseDuration(dhcpConfig.T1Duration)
	if err != nil {
		info.Timeouts.T1RenewalTime = info.Timeouts.T2RebindingTime / 2
	} else {
		info.Timeouts.T1RenewalTime = d
	}

	return info
}
