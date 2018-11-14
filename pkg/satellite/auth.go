package satellite

import (
	"encoding/base64"

	"storj.io/storj/pkg/satellite/satelliteauth"
)

//TODO: change to JWT or Macaroon based auth

// Signer creates signature for provided data
type Signer interface {
	Sign(data []byte) ([]byte, error)
}

// signToken signs token with given signer
func signToken(token *satelliteauth.Token, signer Signer) error {
	encoded := base64.URLEncoding.EncodeToString(token.Payload)

	signature, err := signer.Sign([]byte(encoded))
	if err != nil {
		return err
	}

	token.Signature = signature
	return nil
}
