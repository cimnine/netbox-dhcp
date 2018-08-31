package v4

import (
	"encoding/binary"
	"fmt"
	"github.com/insomniacslk/dhcp/dhcpv4"
)

// This option implements the Renewal (T1) Time Value option
// https://tools.ietf.org/html/rfc2132

// OptRenewalTime represents the Renewal (T1) Time Value option.
type OptRenewalTime struct {
	RenewalTime uint32
}

// ParseOptRenewalTime constructs an OptRenewalTime struct from a
// sequence of bytes and returns it, or an error.
func ParseOptRenewalTime(data []byte) (*OptRenewalTime, error) {
	// Should at least have code, length, and lease time.
	if len(data) < 6 {
		return nil, dhcpv4.ErrShortByteStream
	}
	code := dhcpv4.OptionCode(data[0])
	if code != dhcpv4.OptionRenewTimeValue {
		return nil, fmt.Errorf("expected option %v, got %v instead", dhcpv4.OptionRenewTimeValue, code)
	}
	length := int(data[1])
	if length != 4 {
		return nil, fmt.Errorf("expected length 4, got %v instead", length)
	}
	leaseTime := binary.BigEndian.Uint32(data[2:6])
	return &OptRenewalTime{RenewalTime: leaseTime}, nil
}

// Code returns the option code.
func (o *OptRenewalTime) Code() dhcpv4.OptionCode {
	return dhcpv4.OptionRenewTimeValue
}

// ToBytes returns a serialized stream of bytes for this option.
func (o *OptRenewalTime) ToBytes() []byte {
	serializedTime := make([]byte, 4)
	binary.BigEndian.PutUint32(serializedTime, o.RenewalTime)
	serializedOpt := []byte{byte(o.Code()), byte(o.Length())}
	return append(serializedOpt, serializedTime...)
}

// String returns a human-readable string for this option.
func (o *OptRenewalTime) String() string {
	return fmt.Sprintf("Renewal (T1) Time Value -> %v", o.RenewalTime)
}

// Length returns the length of the data portion (excluding option code and byte
// for length, if any).
func (o *OptRenewalTime) Length() int {
	return 4
}
