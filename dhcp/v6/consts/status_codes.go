package consts

type DHCPv6StatusCode uint16

const (
	DHCPv6StatusCodeSuccess              DHCPv6StatusCode = 0
	DHCPv6StatusCodeUnspecificFail       DHCPv6StatusCode = 1
	DHCPv6StatusCodeNoAddressesAvailable DHCPv6StatusCode = 2
	DHCPv6StatusCodeNoBinding            DHCPv6StatusCode = 3
	DHCPv6StatusCodeNotOnLink            DHCPv6StatusCode = 4
	DHCPv6StatusCodeUseMulticast         DHCPv6StatusCode = 5
)
