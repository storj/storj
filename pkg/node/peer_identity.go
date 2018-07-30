// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package node

// import (
// 	"bytes"
// 	"crypto/x509"
// 	"encoding/base64"
// 	"io"
//
// 	"github.com/zeebo/errs"
// )
//
// // PeerIdentity implements dht.nodeID and is used for the public portion of an identity (i.e. tls public key)
// type PeerIdentity struct {
// 	hash    []byte
// 	pubKey  []byte
// }
//
// // CertToPeerIdentity returns an `PeerIdentity` given an x509 cert and a hash length
// func CertToPeerIdentity(cert *x509.Certificate, hashLen uint16) (*PeerIdentity, error) {
// 	pubKey, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
// 	if err != nil {
// 		return nil, errs.Wrap(err)
// 	}
//
// 	peerID, err := pubKeyToPeerIdentity(pubKey)
// 	if err != nil {
// 		return nil, err
// 	}
//
// 	return peerID, nil
// }
//
// // ParsePeerIdentity parses a `PeerIdentity` from its `String()` representation (i.e.
// // base64-url-encoded concatenation of hash, public key, and hash
// // length).
// func ParsePeerIdentity(id string) (*PeerIdentity, error) {
// 	idBytes, err := base64.URLEncoding.DecodeString(id)
// 	if err != nil {
// 		return nil, ErrInvalidNodeID.Wrap(err)
// 	}
//
// 	keyBytes := []byte{}
// 	io.ReadFull(bytes.NewReader(idBytes), keyBytes)
// 	if !(len(keyBytes) > 0) {
// 		return nil, ErrInvalidNodeID.New("deserialized key length >= 0")
// 	}
//
// 	peerID, err := pubKeyToPeerIdentity(keyBytes)
// 	if err != nil {
// 		return nil, ErrInvalidNodeID.Wrap(err)
// 	}
//
// 	return peerID, nil
// }
//
// // String serializes the hash, public key, and hash length into a PEM-encoded string
// func (k *PeerIdentity) String() string {
// 	return string(k.Bytes())
// }
//
// // Bytes serializes the hash, public key, and hash length into a PEM-encoded byte-slice
// func (k *PeerIdentity) Bytes() []byte {
// 	return idBytes(k.pubKey)
// }
//
// // Hash returns the hash the public key to a langth of `k.hashLen`
// func (k *PeerIdentity) Hash() []byte {
// 	return k.hash
// }
//
// // Difficulty returns the number of trailing zero-value bits in the hash
// func (k *PeerIdentity) Difficulty() uint16 {
// 	return idDifficulty(k.Hash())
// }
