// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"encoding/base64"
	"bytes"
	"crypto"
	"crypto/x509"

	"github.com/zeebo/errs"
	"golang.org/x/crypto/sha3"
	"encoding/binary"
	"io"
	"storj.io/storj/pkg/dht"
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
	keyLen  uint16
	hashLen uint16
}

func NewNodeID(pubKey *crypto.PublicKey, hashLen uint16) (_ dht.NodeID, _ error) {
	keyBytes, err := x509.MarshalPKIXPublicKey(pubKey)
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

func ParseNodeID(id string) (_ dht.NodeID, _ error) {
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

func (k *KadID) Bytes() ([]byte) {
	b := bytes.NewBuffer([]byte{})
	enc := base64.NewEncoder(base64.URLEncoding, b)
	enc.Write(k.hash)
	enc.Write(k.pubKey)
	binary.Write(enc, binary.LittleEndian, k.keyLen)
	binary.Write(enc, binary.LittleEndian, k.hashLen)

	return b.Bytes()
}
