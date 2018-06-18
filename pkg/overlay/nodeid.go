package overlay

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

type NodeID struct {
	hash    []byte
	pubKey  []byte
	keyLen  uint16
	hashLen uint16
}

func keyToNodeID(pubKey *crypto.PublicKey, hashLen uint16) (_ *NodeID, _ error) {
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

	nodeID := &NodeID{
		hash:    hashBytes,
		pubKey:  keyBytes,
		keyLen:  uint16(len(keyBytes)),
		hashLen: hashLen,
	}

	return nodeID, nil
}

func (n *NodeID) String() (_ string) {
	hashString := base64.URLEncoding.EncodeToString(n.hash)
	keyString := base64.URLEncoding.EncodeToString(n.pubKey)

	nodeIDString := fmt.Sprintf("%s%s%s%s", hashString, keyString, n.keyLen, n.hashLen)
	return nodeIDString
}

func NodeIDFromBase64(id string) (_ *NodeID, _ error) {
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
