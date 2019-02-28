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

//GetSigned returns the signing data for this signed type
func (m *OrderLimit) GetSigned() SignedMessage {
	return m.SignedMessage
}

//SetSigned sets the signing data for this signed type
func (m *OrderLimit) SetSigned(sm SignedMessage) {
	m.Data = sm.Data
	m.Certs = sm.Certs
	m.Signature = sm.Signature
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

//Sign adds the crypto-related aspects of signed message
func (m *OrderLimit) Sign(id *identity.FullIdentity) (err error) {
	return Sign(m, id)
}

//Verify checks the crypto-related aspects of signed message
func (m *OrderLimit) Verify(signer storj.NodeID) error {
	return Verify(m, signer)
}
