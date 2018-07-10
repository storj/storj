// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"io"

	"github.com/zeebo/errs"
	"golang.org/x/crypto/sha3"
	"storj.io/storj/pkg/dht"
	"crypto/rand"
	"storj.io/storj/pkg/peertls"
	"path/filepath"
	"os"
	"crypto/tls"
	"io/ioutil"
	"encoding/pem"
	"crypto/ecdsa"
)

const (
	lenSize = int64(2) // NB: number of bytes required to represent `keyLen` and `hashLen`
)

var (
	ErrInvalidNodeID = errs.Class("InvalidNodeIDError")
)

type KadID struct {
	tlsCert *peertls.TLSHelper
	hash    []byte
	pubKey  []byte
	keyLen  uint16
	hashLen uint16
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
	PEMBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	cert, err := decodePEMs(PEMBytes)
	nodeID, err := certToNodeID(cert, 256)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return nodeID, nil
}

func decodePEMs(PEMBytes []byte) (*tls.Certificate, error) {
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
			continue
		}
	}

	return certFromDERs(certDERs, keyDER)
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

// func LoadOrCreateID(basePath string, minDifficulty uint16) (_ dht.NodeID, _ error) {
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
// 			pubkey := t.cert.Leaf.PublicKey.(*crypto.PublicKey)
// 			nodeID, err := certToNodeID(pubkey, 256)
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

func certToNodeID(cert *tls.Certificate, hashLen uint16) (_ *KadID, _ error) {
	pubkey := cert.Leaf.PublicKey.(*ecdsa.PublicKey)
	keyBytes, err := x509.MarshalPKIXPublicKey(pubkey)
	if err != nil {
		return nil, errs.New("unable to marshal pubkey", err)
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

	nodeID := &KadID{
		hash:    hashBytes,
		pubKey:  keyBytes,
		keyLen:  uint16(len(keyBytes)),
		hashLen: hashLen,
	}

	return nodeID, nil
}

func ParseNodeID(id string) (_ *KadID, _ error) {
	nodeID := &KadID{}

	idBytes, err := base64.URLEncoding.DecodeString(id)
	if err != nil {
		// TODO(bryanchriswhite) better error handling
		return nil, ErrInvalidNodeID.Wrap(err)
	}

	idBytesReader := bytes.NewReader(idBytes)

	lengthsByteLen := 2 * lenSize
	lengthsOffset := idBytesReader.Size() - lengthsByteLen
	lengthsSectionReader := io.NewSectionReader(idBytesReader, lengthsOffset, lengthsByteLen)

	lengths := []uint16{}
	if err := binary.Read(lengthsSectionReader, binary.LittleEndian, lengths); err != nil {
		// TODO(bryanchriswhite): error handling
		// ensure lengths exist
		return nil, ErrInvalidNodeID.Wrap(err)
	}

	nodeID.keyLen = lengths[1]
	nodeID.hashLen = lengths[0]

	keySectionReader := io.NewSectionReader(idBytesReader, 0, int64(nodeID.keyLen))
	if _, err := keySectionReader.Read(nodeID.pubKey); err != nil {
		// TODO(bryanchriswhite): error handling
		return nil, ErrInvalidNodeID.Wrap(err)
	}

	hashSectionReader := io.NewSectionReader(idBytesReader, 0, int64(nodeID.keyLen))
	if _, err := hashSectionReader.Read(nodeID.hash); err != nil {
		// TODO(bryanchriswhite): error handling
		return nil, ErrInvalidNodeID.Wrap(err)
	}

	return nodeID, nil
}

func (k *KadID) String() (_ string) {
	return base64.URLEncoding.EncodeToString(k.Bytes())
}

func (k *KadID) Bytes() []byte {
	b := bytes.NewBuffer([]byte{})
	enc := base64.NewEncoder(base64.URLEncoding, b)
	enc.Write(k.hash)
	enc.Write(k.pubKey)
	binary.Write(enc, binary.LittleEndian, k.keyLen)
	binary.Write(enc, binary.LittleEndian, k.hashLen)

	return b.Bytes()
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
