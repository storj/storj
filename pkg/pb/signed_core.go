package pb

import (
	"encoding/json"

	"github.com/gogo/protobuf/proto"

	"storj.io/storj/pkg/auth"
	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/storj"
)

//Signed allows simple signing and custom protobuf serialization
type Signed interface {
	Message() proto.Message
	Signed() *SignedMessage
}

// Marshal serializes a Signed
func Marshal(m Signed) (b []byte, err error) {
	return proto.Marshal(m.Message())
}

// MarshalTo serializes a Signed into the passed byte slice
func MarshalTo(m Signed, b []byte) (n int, err error) {
	out, err := proto.Marshal(m.Message())
	n = copy(b, out)
	return n, err
}

// Unmarshal deserializes a Signed
func Unmarshal(m Signed, b []byte) error {
	err := proto.Unmarshal(b, m.Message())
	if err != nil {
		return err
	}
	return proto.Unmarshal(m.Signed().Data, m.Message())
}

// Size returns the length of a Signed (implements gogo's custom type interface)
func Size(m Signed) int {
	signed := m.Signed()
	return signed.XXX_Size()
}

// MarshalJSON serializes a Signed to a json string as bytes
func MarshalJSON(m Signed) ([]byte, error) {
	return json.Marshal(m.Message())
}

// UnmarshalJSON deserializes a json string (as bytes) to a Signed
func UnmarshalJSON(m Signed, b []byte) error {
	err := json.Unmarshal(b, m.Message())
	if err != nil {
		return err
	}
	return proto.Unmarshal(m.Signed().Data, m.Message())
}

//Sign adds the crypto-related aspects of signed message
func Sign(m Signed, id identity.FullIdentity) (err error) {
	signed := m.Signed()
	signed.Data, err = proto.Marshal(m.Message())
	if err != nil {
		return auth.ErrMarshal.Wrap(err)
	}
	signed.Certs = id.ChainRaw()
	signed.Signature, err = pkcrypto.HashAndSign(id.Key, signed.Data)
	if err != nil {
		return auth.ErrSign.Wrap(err)
	}
	return nil
}

//Verify checks the crypto-related aspects of signed message
func Verify(m Signed, signer storj.NodeID) error {
	//check certs
	if len(m.Signed().Certs) < 2 {
		return auth.ErrVerify.New("Expected at least leaf and CA public keys")
	}
	err := peertls.VerifyPeerFunc(peertls.VerifyPeerCertChains)(m.Signed().Certs, nil)
	if err != nil {
		return auth.ErrVerify.Wrap(err)
	}
	leaf, err := pkcrypto.CertFromDER(m.Signed().Certs[0])
	if err != nil {
		return err
	}
	ca, err := pkcrypto.CertFromDER(m.Signed().Certs[1])
	if err != nil {
		return err
	}
	// verify signature
	if id, err := identity.NodeIDFromKey(ca.PublicKey); err != nil || id != signer {
		return auth.ErrSigner.New("%+v vs %+v", id, signer)
	}
	if err := pkcrypto.HashAndVerifySignature(leaf.PublicKey, m.Signed().Data, m.Signed().Signature); err != nil {
		return auth.ErrVerify.New("%+v", err)
	}
	//cleanup
	if err = proto.Unmarshal(m.Signed().Data, m.Message()); err != nil {
		return auth.ErrMarshal.Wrap(err)
	}
	return nil
}
