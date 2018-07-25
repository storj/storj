package provider

import (
	"crypto/tls"
	"encoding/pem"
	"storj.io/storj/pkg/peertls"
	"crypto/x509"
	"github.com/zeebo/errs"
)

func parseIDBytes(PEMBytes []byte) (*tls.Certificate, error) {
	certDERs := [][]byte{}
	keyDER := []byte{}

	for {
		var DERBlock *pem.Block

		DERBlock, PEMBytes = pem.Decode(PEMBytes)
		if DERBlock == nil {
			break
		}

		switch DERBlock.Type {
		case peertls.BlockTypeCertificate:
			certDERs = append(certDERs, DERBlock.Bytes)
			continue

		case peertls.BlockTypeEcPrivateKey:
			keyDER = DERBlock.Bytes
			continue
		}
	}

	if len(certDERs) == 0 || len(certDERs[0]) == 0 {
		return nil, errs.New("no certificates found in identity file")
	}

	if len(keyDER) == 0 {
		return nil, errs.New("no private key found in identity file")
	}

	cert, err := certFromDERs(certDERs, keyDER)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return cert, nil
}

func certFromDERs(certDERBytes [][]byte, keyDERBytes []byte) (*tls.Certificate, error) {
	var (
		err  error
		cert = new(tls.Certificate)
	)

	cert.Certificate = certDERBytes
	cert.PrivateKey, err = x509.ParseECPrivateKey(keyDERBytes)
	if err != nil {
		return nil, errs.New("unable to parse EC private key", err)
	}

	parsedLeaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, errs.Wrap(err)
	}

	cert.Leaf = parsedLeaf

	return cert, nil
}
