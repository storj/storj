package peertls

import (
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"

	"github.com/zeebo/errs"
)

func newKeyBlock(b []byte) (_ *pem.Block) {
	return &pem.Block{Type: "EC PRIVATE KEY", Bytes: b}
}

func newCertBlock(b []byte) (_ *pem.Block) {
	return &pem.Block{Type: "CERTIFICATE", Bytes: b}
}

func keyToDERBytes(key *ecdsa.PrivateKey) (_ []byte, _ error) {
	b, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		return nil, errs.New("unable to marshal ECDSA private key", err)
	}

	return b, nil
}

func keyToBlock(key *ecdsa.PrivateKey) (_ *pem.Block, _ error) {
	b, err := keyToDERBytes(key)
	if err != nil {
		return nil, err
	}

	return newKeyBlock(b), nil
}

func certFromPEMs(certPEMBytes, keyPEMBytes []byte) (*tls.Certificate, error) {
	var (
		certDERs = [][]byte{}
	)

	for {
		var certDERBlock *pem.Block
		certDERBlock, certPEMBytes = pem.Decode(certPEMBytes)
		if certDERBlock == nil {
			break
		}

		certDERs = append(certDERs, certDERBlock.Bytes)
	}

	keyPEMBlock, _ := pem.Decode(keyPEMBytes)

	return certFromDERs(certDERs, keyPEMBlock.Bytes)
}

func certFromDERs(certDERBytes [][]byte, keyDERBytes []byte) (*tls.Certificate, error) {
	fail := func(err error) (*tls.Certificate, error) { return &tls.Certificate{}, err }

	var cert = new(tls.Certificate)
	cert.Certificate = certDERBytes

	var err error
	cert.PrivateKey, err = x509.ParseECPrivateKey(keyDERBytes)
	if err != nil {
		return fail(err)
	}

	return cert, nil
}
