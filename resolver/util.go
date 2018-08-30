package resolver

import (
	"github.com/ninech/nine-dhcp2/dhcp/config"
	"github.com/ninech/nine-dhcp2/util"
	"net"
	"time"
)

func NewClientInfoV4(dhcpConfig *config.DHCPConfig) *ClientInfoV4 {
	info := ClientInfoV4{
		NextServer:   net.ParseIP(dhcpConfig.DefaultOptions.NextServer),
		BootFileName: dhcpConfig.DefaultOptions.BootFileName,
	}

	d, err := time.ParseDuration(dhcpConfig.ReservationTimeout)
	if err != nil {
		info.Timeouts.Reservation = 1 * time.Minute
	} else {
		info.Timeouts.Reservation = d
	}

	d, err = time.ParseDuration(dhcpConfig.LeaseTimeout)
	if err != nil {
		info.Timeouts.Lease = 6 * time.Hour
	} else {
		info.Timeouts.Lease = d
	}

	d, err = time.ParseDuration(dhcpConfig.T2Timeout)
	if err != nil {
		info.Timeouts.T2RebindingTime = info.Timeouts.Lease / 2
	} else {
		info.Timeouts.T2RebindingTime = d
	}

	d, err = time.ParseDuration(dhcpConfig.T1Timeout)
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
