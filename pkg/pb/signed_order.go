package pb

import (
	proto "github.com/gogo/protobuf/proto"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/storj"
)

//Order hides the protocol buffer nature of RenterBandwidthAllocation
type Order RenterBandwidthAllocation

//Message returns the base message of this signed type
func (m *Order) Message() proto.Message {
	return (*RenterBandwidthAllocation)(m)
}

//GetSigned returns the signing data for this signed type
func (m *Order) GetSigned() SignedMessage {
	return m.SignedMessage
}

//SetSigned sets the signing data for this signed type
func (m *Order) SetSigned(sm SignedMessage) {
	m.Data = sm.Data
	m.Certs = sm.Certs
	m.Signature = sm.Signature
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

//Sign adds the crypto-related aspects of signed message
func (m *Order) Sign(id *identity.FullIdentity) (err error) {
	return Sign(m, id)
}

//Verify checks the crypto-related aspects of signed message
func (m *Order) Verify(signer storj.NodeID) error {
	return Verify(m, signer)
}
