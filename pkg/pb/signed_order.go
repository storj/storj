package pb

import (
	"crypto"

	proto "github.com/gogo/protobuf/proto"

	"storj.io/storj/pkg/storj"
)

//Order hides the protocol buffer nature of RenterBandwidthAllocation
type Order RenterBandwidthAllocation

//Message returns the base message of this signed type
func (m *Order) Message() proto.Message {
	return (*RenterBandwidthAllocation)(m)
}

//Signed returns the signing data for this signed type
func (m *Order) Signed() *SignedMessage {
	return &m.SignedMessage
}

// Marshal serializes a Signed
func (m *Order) Marshal() (b []byte, err error) {
	return Marshal(m)
}

// MarshalTo serializes a Signed into the passed byte slice
func (m *Order) MarshalTo(b []byte) (n int, err error) {
	return MarshalTo(m, b)
}

// Unmarshal deserializes a Signed
func (m *Order) Unmarshal(b []byte) error {
	return Unmarshal(m, b)
}

// Size returns the length of a Signed (implements gogo's custom type interface)
func (m *Order) Size() int {
	return Size(m)
}

// MarshalJSON serializes a Signed to a json string as bytes
func (m *Order) MarshalJSON() ([]byte, error) {
	return MarshalJSON(m)
}

// UnmarshalJSON deserializes a json string (as bytes) to a Signed
func (m *Order) UnmarshalJSON(b []byte) error {
	return UnmarshalJSON(m, b)
}

//Sign adds the crypto-related aspects of signed message
func (m *Order) Sign(key crypto.PrivateKey) (err error) {
	return Sign(m, key)
}

//Verify checks the crypto-related aspects of signed message
func (m *Order) Verify(signer storj.NodeID) error {
	return Verify(m, signer)
}
