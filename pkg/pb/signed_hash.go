package pb

import (
	"storj.io/storj/pkg/identity"

	proto "github.com/gogo/protobuf/proto"

	"storj.io/storj/pkg/storj"
)

//SignedHash implements signing and custom protobuf serialization
type SignedHash Hash

//Message returns the base message of this signed type
func (m *SignedHash) Message() proto.Message {
	return (*Hash)(m)
}

//Signed returns the signing data for this signed type
func (m *SignedHash) Signed() *SignedMessage {
	return &m.SignedMessage
}

// Marshal serializes a Signed
func (m *SignedHash) Marshal() (b []byte, err error) {
	return Marshal(m)
}

// MarshalTo serializes a Signed into the passed byte slice
func (m *SignedHash) MarshalTo(b []byte) (n int, err error) {
	return MarshalTo(m, b)
}

// Unmarshal deserializes a Signed
func (m *SignedHash) Unmarshal(b []byte) error {
	return Unmarshal(m, b)
}

// Size returns the length of a Signed (implements gogo's custom type interface)
func (m *SignedHash) Size() int {
	return Size(m)
}

// MarshalJSON serializes a Signed to a json string as bytes
func (m *SignedHash) MarshalJSON() ([]byte, error) {
	return MarshalJSON(m)
}

// UnmarshalJSON deserializes a json string (as bytes) to a Signed
func (m *SignedHash) UnmarshalJSON(b []byte) error {
	return UnmarshalJSON(m, b)
}

//Sign adds the crypto-related aspects of signed message
func (m *SignedHash) Sign(id identity.FullIdentity) (err error) {
	return Sign(m, id)
}

//Verify checks the crypto-related aspects of signed message
func (m *SignedHash) Verify(signer storj.NodeID) error {
	return Verify(m, signer)
}
