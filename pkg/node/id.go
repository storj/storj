// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package node

import (
	"bytes"
	"crypto/x509"
	"encoding/base64"
	"encoding/binary"
	"io"

	"github.com/zeebo/errs"
)

// ID implements dht.nodeID and is used for the public portion of an identity (i.e. tls public key)
type ID struct {
	hash    []byte
	pubKey  []byte
	hashLen uint16
}

// CertToID returns an `ID` given an x509 cert and a hash length
func CertToID(cert *x509.Certificate, hashLen uint16) (*ID, error) {
	pubKey, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	hashBytes, err := hash(pubKey, hashLen)
	if err != nil {
		return nil, err
	}

	kadID := &ID{
		pubKey:  pubKey,
		hash:    hashBytes,
		hashLen: hashLen,
	}

	return kadID, nil
}

// ParseID parses a `ID` from its `String()` representation (i.e.
// base64-url-encoded concatenation of hash, public key, and hash
// length).
func ParseID(id string) (*ID, error) {
	kadID := &ID{}

	idBytes, err := base64.URLEncoding.DecodeString(id)
	if err != nil {
		return nil, ErrInvalidNodeID.Wrap(err)
	}

	idBytesReader := bytes.NewReader(idBytes)

	hashLenSectionReader := io.NewSectionReader(
		idBytesReader,
		idBytesReader.Size()-lenSize,
		lenSize,
	)

	var (
		hashLen uint16
	)
	if err := binary.Read(hashLenSectionReader, binary.BigEndian, &hashLen); err != nil {
		return nil, ErrInvalidNodeID.Wrap(err)
	}

	kadID.hashLen = hashLen
	keyLen := idBytesReader.Size() - int64(hashLen) - int64(lenSize)

	if keyLen <= 0 {
		return nil, ErrInvalidNodeID.New("unreasonable length(s); hash: %d key: %d", hashLen, keyLen)
	}

	hashBytes := make([]byte, hashLen)
	keyBytes := make([]byte, keyLen)

	hashSectionReader := io.NewSectionReader(idBytesReader, 0, int64(hashLen))
	if _, err := hashSectionReader.Read(hashBytes); err != nil {
		if err != io.EOF {
			return nil, ErrInvalidNodeID.Wrap(err)
		}
	}

	keySectionReader := io.NewSectionReader(idBytesReader, int64(hashLen), int64(keyLen))
	if _, err := keySectionReader.Read(keyBytes); err != nil {
		if err != io.EOF {
			return nil, ErrInvalidNodeID.Wrap(err)
		}
	}

	kadID.hash = hashBytes
	kadID.pubKey = keyBytes

	return kadID, nil
}

// String serializes the hash, public key, and hash length into a PEM-encoded string
func (k *ID) String() string {
	return string(k.Bytes())
}

// Bytes serializes the hash, public key, and hash length into a PEM-encoded byte-slice
func (k *ID) Bytes() []byte {
	return idBytes(k.hash, k.pubKey, k.hashLen)
}

// Hash returns the hash the public key to a langth of `k.hashLen`
func (k *ID) Hash() []byte {
	return k.hash
}

// Difficulty returns the number of trailing zero-value bits in the hash
func (k *ID) Difficulty() uint16 {
	return idDifficulty(k.Hash())
}
