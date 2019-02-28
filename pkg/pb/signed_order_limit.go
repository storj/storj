package pb

import (
	proto "github.com/gogo/protobuf/proto"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/storj"
)

//OrderLimit hides the protocol buffer nature of PayerBandwidthAllocation
type OrderLimit PayerBandwidthAllocation

//Message returns the base message of this signed type
func (m *OrderLimit) Message() proto.Message {
	return (*PayerBandwidthAllocation)(m)
}

//Signed returns the signing data for this signed type
func (m *OrderLimit) Signed() *SignedMessage {
	return &m.SignedMessage
}

// Marshal serializes a Signed
func (m *OrderLimit) Marshal() (b []byte, err error) {
	return Marshal(m)
}

// MarshalTo serializes a Signed into the passed byte slice
func (m *OrderLimit) MarshalTo(b []byte) (n int, err error) {
	return MarshalTo(m, b)
}

// Unmarshal deserializes a Signed
func (m *OrderLimit) Unmarshal(b []byte) error {
	return Unmarshal(m, b)
}

// Size returns the length of a Signed (implements gogo's custom type interface)
func (m *OrderLimit) Size() int {
	return Size(m)
}

// MarshalJSON serializes a Signed to a json string as bytes
func (m *OrderLimit) MarshalJSON() ([]byte, error) {
	return MarshalJSON(m)
}

// UnmarshalJSON deserializes a json string (as bytes) to a Signed
func (m *OrderLimit) UnmarshalJSON(b []byte) error {
	return UnmarshalJSON(m, b)
}

//Sign adds the crypto-related aspects of signed message
func (m *OrderLimit) Sign(id identity.FullIdentity) (err error) {
	return Sign(m, id)
}

//Verify checks the crypto-related aspects of signed message
func (m *OrderLimit) Verify(signer storj.NodeID) error {
	return Verify(m, signer)
}
