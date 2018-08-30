package dhcpv4

import (
	"encoding/binary"
	"fmt"
	"github.com/insomniacslk/dhcp/dhcpv4"
)

// This option implements the Rebinding (T1) Time Value option
// https://tools.ietf.org/html/rfc2132

// OptRebindingTime represents the Rebinding (T1) Time Value option.
type OptRebindingTime struct {
	RebindingTime uint32
}

// ParseOptRebindingTime constructs an OptRebindingTime struct from a
// sequence of bytes and returns it, or an error.
func ParseOptRebindingTime(data []byte) (*OptRebindingTime, error) {
	// Should at least have code, length, and lease time.
	if len(data) < 6 {
		return nil, dhcpv4.ErrShortByteStream
	}
	code := dhcpv4.OptionCode(data[0])
	if code != dhcpv4.OptionRebindingTimeValue {
		return nil, fmt.Errorf("expected option %v, got %v instead", dhcpv4.OptionRebindingTimeValue, code)
	}
	length := int(data[1])
	if length != 4 {
		return nil, fmt.Errorf("expected length 4, got %v instead", length)
	}
	leaseTime := binary.BigEndian.Uint32(data[2:6])
	return &OptRebindingTime{RebindingTime: leaseTime}, nil
}

// Code returns the option code.
func (o *OptRebindingTime) Code() dhcpv4.OptionCode {
	return dhcpv4.OptionRebindingTimeValue
}

// ToBytes returns a serialized stream of bytes for this option.
func (o *OptRebindingTime) ToBytes() []byte {
	serializedTime := make([]byte, 4)
	binary.BigEndian.PutUint32(serializedTime, o.RebindingTime)
	serializedOpt := []byte{byte(o.Code()), byte(o.Length())}
	return append(serializedOpt, serializedTime...)
}

// String returns a human-readable string for this option.
func (o *OptRebindingTime) String() string {
	return fmt.Sprintf("Rebinding (T1) Time Value -> %v", o.RebindingTime)
}

// Length returns the length of the data portion (excluding option code and byte
// for length, if any).
func (o *OptRebindingTime) Length() int {
	return 4
}
