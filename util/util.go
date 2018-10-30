package util

import (
	"math"
	"net"
)

func SafeConvertToUint32(float64Value float64) uint32 {
	if float64Value > math.MaxUint32 {
		return math.MaxUint32
	} else if float64Value < 0 {
		return 0
	} else {
		return uint32(float64Value)
	}
}

func ParseIP4s(ipStrs []string) []net.IP {
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
