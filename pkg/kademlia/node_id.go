// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"encoding/pem"
	"io"
	"io/ioutil"
	"os"
	"path/filepath"

	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/crypto/sha3"
	"storj.io/storj/pkg/dht"
	"storj.io/storj/pkg/peertls"
)

const (
	lenSize = int64(2) // NB: number of bytes required to represent `keyLen` and `hashLen`
)

var (
	ErrInvalidNodeID = errs.Class("InvalidNodeIDError")
)

type KadID struct {
	hash    []byte
	pubKey  []byte
	hashLen uint16
}

type KadCreds struct {
	hash    []byte
	hashLen uint16
	tlsH    *peertls.TLSHelper
}

// LoadID reads and parses an "identity" file containing a tls certificate
// chain and a private key for the leaf certificate.
//
// The files must contain PEM encoded data. The certificate portion
// may contain intermediate certificates following the leaf certificate to
// form a certificate chain.
func LoadID(path string) (dht.NodeID, error) {
	baseDir := filepath.Dir(path)

	if _, err := os.Stat(baseDir); err != nil {
		if err == os.ErrNotExist {
			if err := os.MkdirAll(baseDir, 600); err != nil {
				return nil, errs.Wrap(err)
			}
		} else {
			return nil, errs.Wrap(err)
		}
	}

	// Attempt to load
	IDBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	cert, hashLen, err := decodeIDBytes(IDBytes)
	nodeID, err := certToKadCreds(cert, hashLen)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return nodeID, nil
}

func (k *KadCreds) Save(path string) error {
	baseDir := filepath.Dir(path)

	if _, err := os.Stat(baseDir); err != nil {
		if err == os.ErrNotExist {
			if err := os.MkdirAll(baseDir, 600); err != nil {
				return errs.Wrap(err)
			}
		} else {
			return errs.Wrap(err)
		}
	}

	file, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return errs.New("unable to open identity file for writing \"%s\"", path, err)
	}

	defer file.Close()

	if err = writeIDBytes(file, k.tlsH.Certificate(), k.hashLen); err != nil {
		return err
	}

	return nil
}

func writeIDBytes(writer io.Writer, cert tls.Certificate, hashLen uint16) error {
	for _, c := range cert.Certificate {
		certBlock := peertls.NewCertBlock(c)

		if err := pem.Encode(writer, certBlock); err != nil {
			return errs.Wrap(err)
		}
	}

	keyDERBytes, err := peertls.KeyToDERBytes(cert.PrivateKey.(*ecdsa.PrivateKey))
	if err != nil {
		return err
	}

	if err := pem.Encode(writer, peertls.NewKeyBlock(keyDERBytes)); err != nil {
		return errs.Wrap(err)
	}

	// Write `hashLen` after private key
	binary.Write(writer, binary.LittleEndian, hashLen)
	return nil
}

func decodeIDBytes(PEMBytes []byte) (*tls.Certificate, uint16, error) {
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

	return cert, nil
}

// func LoadOrCreateID(basePath string, minDifficulty uint16) (dht.NodeID, error) {
// 	t, err := peertls.NewTLSHelper(
// 		basePath,
// 		basePath,
// 		false,
// 		false,
// 	)
//
// 	if err != nil {
// 		if peertls.ErrNotExist.Has(err) {
// 		}
//
// 		return nil, errs.Wrap(err)
// 	}
//
// 	baseDir := filepath.Dir(basePath)
//
// 	if _, err := os.Stat(baseDir); err != nil {
// 		if err == os.ErrNotExist {
// 			if err := os.MkdirAll(baseDir, 600); err != nil {
// 				return nil, errs.Wrap(err)
// 			}
//
// 			t, err := peertls.NewTLSHelper(
// 				basePath,
// 				basePath,
// 				true,
// 				false,
// 			)
//
// 			pubKey := t.cert.Leaf.PublicKey.(*crypto.PublicKey)
// 			nodeID, err := certToKadCreds(pubKey, 256)
// 			if err != nil {
// 				return nil, errs.Wrap(err)
// 			}
//
// 			nodeID.t = t
//
// 			return nodeID, nil
// 		} else {
// 			return nil, errs.Wrap(err)
// 		}
// 	}
// }

// func generateID(minDifficulty uint16) (*KadID, error) {
//
// }

func certToKadCreds(cert *tls.Certificate, hashLen uint16) (*KadCreds, error) {
	pubKey := cert.Leaf.PublicKey.(*ecdsa.PublicKey)
	keyBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return nil, errs.New("unable to marshal pubKey", err)
	}

	shake := sha3.NewShake256()
	shake.Write(keyBytes)
	hashBytes := make([]byte, hashLen)

	bytesRead, err := shake.Read(hashBytes)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	if uint16(bytesRead) != hashLen {
		return nil, errs.New("hash length error")
	}

	tlsH, _ := peertls.NewTLSHelper(cert)
	nodeID := &KadCreds{
		tlsH: tlsH,
		hash: hashBytes,
	}

	return nodeID, nil
}

func ParseNodeID(id string) (*KadID, error) {
	nodeID := &KadID{}

	idBytes, err := base64.URLEncoding.DecodeString(id)
	if err != nil {
		// TODO(bryanchriswhite) better error handling
		return nil, ErrInvalidNodeID.Wrap(err)
	}

	idBytesReader := bytes.NewReader(idBytes)

	lengthsSectionReader := io.NewSectionReader(
		idBytesReader,
		idBytesReader.Size()-lenSize,
		lenSize,
	)

	var (
		hashLen uint16
	)
	if err := binary.Read(lengthsSectionReader, binary.LittleEndian, hashLen); err != nil {
		// TODO(bryanchriswhite): error handling
		// ensure hashLen exist
		return nil, ErrInvalidNodeID.Wrap(err)
	}

	nodeID.hashLen = hashLen
	keyLen := idBytesReader.Size() - int64(hashLen) - int64(lenSize)
	hashBytes := make([]byte, hashLen)
	keyBytes := make([]byte, keyLen)

	hashSectionReader := io.NewSectionReader(idBytesReader, 0, int64(hashLen))
	if _, err := hashSectionReader.Read(hashBytes); err != nil {
		// TODO(bryanchriswhite): error handling
		return nil, ErrInvalidNodeID.Wrap(err)
	}

	keySectionReader := io.NewSectionReader(idBytesReader, int64(hashLen), int64(keyLen))
	if _, err := keySectionReader.Read(keyBytes); err != nil {
		// TODO(bryanchriswhite): error handling
		return nil, ErrInvalidNodeID.Wrap(err)
	}

	return nodeID, nil
}

func (k *KadCreds) String() string {
	return base64.URLEncoding.EncodeToString(k.Bytes())
}

func (k *KadCreds) Bytes() []byte {
	pubKey, err := x509.MarshalPKIXPublicKey(k.tlsH.PubKey())
	if err != nil {
		zap.S().Error(errs.New("unable to marshal public key"))
	}

	b := bytes.NewBuffer([]byte{})
	enc := base64.NewEncoder(base64.URLEncoding, b)
	enc.Write(k.hash)
	enc.Write(pubKey)
	binary.Write(enc, binary.LittleEndian, k.hashLen)

	return b.Bytes()
}

func (k *KadCreds) Hash() []byte {
	return k.hash
}

func (k *KadID) String() string {
	return base64.URLEncoding.EncodeToString(k.Bytes())
}

func (k *KadID) Bytes() []byte {
	b := bytes.NewBuffer([]byte{})
	enc := base64.NewEncoder(base64.URLEncoding, b)
	enc.Write(k.hash)
	enc.Write(k.pubKey)
	binary.Write(enc, binary.LittleEndian, k.hashLen)

	return b.Bytes()
}

func (k *KadID) Hash() []byte {
	return k.hash
}

type mockID struct {
	bytes []byte
}

func (m *mockID) String() string {
	return string(m.bytes)
}

func (m *mockID) Bytes() []byte {
	return m.bytes
}

// NewID returns a pointer to a newly intialized NodeID
func NewID() (dht.NodeID, error) {
	idBytes, err := newID(2)
	if err != nil {
		return nil, err
	}

	nodeID := &mockID{
		idBytes,
	}

	return nodeID, nil
}

// newID generates a new random ID.
// This purely to get things working. We shouldn't use this as the ID in the actual network
func newID(difficulty uint16) ([]byte, error) {
	id := make([]byte, 20)
	if _, err := rand.Read(id); err != nil {
		return nil, errs.Wrap(err)
	}

	result := []byte(base64.URLEncoding.EncodeToString(id))
	return result, nil
}
