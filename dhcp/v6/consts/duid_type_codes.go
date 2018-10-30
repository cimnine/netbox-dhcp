package consts

// RFC 3315 DUID Type Code
// https://tools.ietf.org/html/rfc3315#section-9.1
type DHCPv6DUIDTypeCode uint16

// https://www.iana.org/assignments/dhcpv6-parameters/dhcpv6-parameters.xhtml#dhcpv6-parameters-6
const (
	// RFC 3315
	// https://tools.ietf.org/html/rfc3315#section-9.2
	DHCPv6DUIDTypeLinkLayerAddressPlusTime DHCPv6DUIDTypeCode = 1
	// RFC 3315
	// https://tools.ietf.org/html/rfc3315#section-9.3
	DHCPv6DUIDTypeVendorBasedOnEnterpriseNumber DHCPv6DUIDTypeCode = 2
	// RFC 3315
	// https://tools.ietf.org/html/rfc3315#section-9.4
	DHCPv6DUIDTypeLinkLayerAddress DHCPv6DUIDTypeCode = 3
	// RFC 6355
	// https://tools.ietf.org/html/rfc6355#section-4
	DHCPv6DUIDTypeUUID DHCPv6DUIDTypeCode = 4
)
