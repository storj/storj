// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package node

import (
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"io"

	"bytes"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"math/bits"
	"os"
	"path/filepath"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
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

func idBytes(hash, pubKey []byte, hashLen uint16) []byte {
	b := bytes.NewBuffer([]byte{})
	encoder := base64.NewEncoder(base64.URLEncoding, b)
	if _, err := encoder.Write(hash); err != nil {
		zap.S().Error(errs.Wrap(err))
	}

	if _, err := encoder.Write(pubKey); err != nil {
		zap.S().Error(errs.Wrap(err))
	}

	if err := binary.Write(encoder, binary.BigEndian, hashLen); err != nil {
		zap.S().Error(errs.Wrap(err))
	}

	if err := encoder.Close(); err != nil {
		zap.S().Error(errs.Wrap(err))
	}

	return b.Bytes()
}

func idDifficulty(hash []byte) uint16 {
	for i := 1; i < len(hash); i++ {
		b := hash[len(hash)-i]

		if b != 0 {
			zeroBits := bits.TrailingZeros16(uint16(b))
			if zeroBits == 16 {
				zeroBits = 0
			}

			return uint16((i-1)*8 + zeroBits)
		}
	}

	// NB: this should never happen
	reason := fmt.Sprintf("difficulty matches hash length! hash: %s", hash)
	zap.S().Error(reason)
	panic(reason)
}

func (c *Creds) writeRootKey(dir string) error {
	path := filepath.Join(filepath.Dir(dir), "root.pem")
	rootKey := c.tlsH.RootKey()

	if rootKey != (ecdsa.PrivateKey{}) {
		file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
		if err != nil {
			return errs.New("unable to open identity file for writing \"%s\"", path, err)
		}

		defer func() {
			if err := file.Close(); err != nil {
				zap.S().Error(errs.Wrap(err))
			}
		}()

		keyBytes, err := peertls.KeyToDERBytes(&rootKey)
		if err != nil {
			return err
		}

		if err := pem.Encode(file, peertls.NewKeyBlock(keyBytes)); err != nil {
			return errs.Wrap(err)
		}

		c.tlsH.DeleteRootKey()
	}

	return nil
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

	return nil
}

func read(PEMBytes []byte) (*tls.Certificate, error) {
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
