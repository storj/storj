// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package node

import (
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/binary"
	"encoding/pem"
	"io"

	"github.com/zeebo/errs"
	"golang.org/x/crypto/sha3"
	"storj.io/storj/pkg/peertls"
)

func baseConfig(difficulty, hashLen uint16) *tls.Config {
	verify := func(_ [][]byte, certChains [][]*x509.Certificate) error {
		for _, certs := range certChains {
			for _, c := range certs {
				kadID, err := CertToID(c, hashLen)
				if err != nil {
					return err
				}

				if kadID.Difficulty() < difficulty {
					return ErrDifficulty.New("expected: %d; got: %d", difficulty, kadID.Difficulty())
				}
			}
		}

		return nil
	}

	return &tls.Config{
		VerifyPeerCertificate: verify,
	}
}

func generateCreds(difficulty, hashLen uint16, c chan Creds, done chan bool) {
	for {
		select {
		case <-done:

			return
		default:
			tlsH, _ := peertls.NewTLSHelper(nil)

			cert := tlsH.Certificate()
			kadCreds, _ := CertToCreds(&cert, hashLen)
			kadCreds.tlsH.BaseConfig = baseConfig(kadCreds.Difficulty(), hashLen)

			if kadCreds.Difficulty() >= difficulty {
				c <- *kadCreds
			}
		}
	}
}

func (c *Creds) write(writer io.Writer) error {
	for _, c := range c.tlsH.Certificate().Certificate {
		certBlock := peertls.NewCertBlock(c)

		if err := pem.Encode(writer, certBlock); err != nil {
			return errs.Wrap(err)
		}
	}

	keyDERBytes, err := peertls.KeyToDERBytes(
		c.tlsH.Certificate().PrivateKey.(*ecdsa.PrivateKey),
	)
	if err != nil {
		return err
	}

	if err := pem.Encode(writer, peertls.NewKeyBlock(keyDERBytes)); err != nil {
		return errs.Wrap(err)
	}

	// Write `hashLen` after private key
	return binary.Write(writer, binary.LittleEndian, c.hashLen)
}

func read(PEMBytes []byte) (*tls.Certificate, uint16, error) {
	var hashLen uint16
	certDERs := [][]byte{}
	keyDER := []byte{}

	for {
		var DERBlock *pem.Block

		DERBlock, PEMBytes = pem.Decode(PEMBytes)
		if DERBlock == nil {
			break
		}

		if DERBlock.Type == peertls.BlockTypeCertificate {
			certDERs = append(certDERs, DERBlock.Bytes)
			continue
		}

		if DERBlock.Type == peertls.BlockTypeEcPrivateKey {
			keyDER = DERBlock.Bytes

			// NB: `hashLen` is stored after the private key block
			if PEMBytes == nil || len(PEMBytes) == 0 {
				return nil, 0, errs.New("hash length expected following private key; none found")
			}

			hashLen = binary.LittleEndian.Uint16(PEMBytes)
			continue
		}
	}

	if len(certDERs) == 0 || len(certDERs[0]) == 0 {
		return nil, 0, errs.New("no certificates found in identity file")
	}

	if len(keyDER) == 0 {
		return nil, 0, errs.New("no private key found in identity file")
	}

	cert, err := certFromDERs(certDERs, keyDER)
	if err != nil {
		return nil, 0, errs.Wrap(err)
	}

	return cert, hashLen, nil
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

func hash(input []byte, hashLen uint16) ([]byte, error) {
	shake := sha3.NewShake256()
	if _, err := shake.Write(input); err != nil {
		return nil, errs.Wrap(err)
	}

	hashBytes := make([]byte, hashLen)

	bytesRead, err := shake.Read(hashBytes)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	if uint16(bytesRead) != hashLen {
		return nil, errs.New("hash length error")
	}

	return hashBytes, nil
}
