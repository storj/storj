package kademlia

import (
	"encoding/base64"
	"fmt"
	"bytes"
	"io"
	"crypto"
	"crypto/x509"
	"github.com/zeebo/errs"
	"golang.org/x/crypto/sha3"
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

func keyToNodeID(pubKey *crypto.PublicKey, hashLen uint16) (_ *KadID, _ error) {
	keyBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		// TODO(bryanchriswhite) better error handling
		return nil, errs.Wrap(err)
	}

	shake := sha3.NewShake256()
	shake.Write(keyBytes)
	hashBytes := make([]byte, hashLen)

	_, err = shake.Read(hashBytes)
	if err != nil {
		// TODO(bryanchriswhite) better error handling
		return nil, errs.Wrap(err)
	}

	nodeID := &KadID{
		hash:    hashBytes,
		pubKey:  keyBytes,
		keyLen:  uint16(len(keyBytes)),
		hashLen: hashLen,
	}

	return nodeID, nil
}

func (k *KadID) String() (_ string) {
	b := bytes.NewBuffer([]byte{})
	enc := base64.NewEncoder(base64.URLEncoding, b)
	enc.Write(k.hash)
	enc.Write(k.pubKey)
	enc.Write(k.keyLen)
	enc.Write(k.hashLen)

	strID := base64.URLEncoding.EncodeToString(b.Bytes())


	return
}

func KadIDFromString(id string) (_ *KadID, _ error) {
	idBytes, err := base64.URLEncoding.DecodeString(id)
	if err != nil {
		// TODO(bryanchriswhite) better error handling
		return nil, errs.Wrap(err)
	}

	idBytesReader := bytes.NewReader(idBytes)
	lenSize := int64(2)
	offset := idBytesReader.Size() - lenSize

	// readTail(idBytesReader)
	reader := io.NewSectionReader(idBytesReader, offset, lenSize)
}
