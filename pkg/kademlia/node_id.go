// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package kademlia

import (
	"bytes"
	"crypto/ecdsa"
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
	// ErrInvalidNodeID is used when a node id can't be parsed
	ErrInvalidNodeID = errs.Class("InvalidNodeIDError")
	ErrDifficulty = errs.Class("difficulty error")
)

// KadID implements dht.nodeID and is used for the public portion of an identity (i.e. tls public key)
type KadID struct {
	hash    []byte
	pubKey  []byte
	hashLen uint16
}

// KadCreds implements dht.nodeID and is used for the private portion of an identity (i.e. tls cert/private key)
type KadCreds struct {
	hash    []byte
	hashLen uint16
	tlsH    *peertls.TLSHelper
}

func baseConfig(difficulty, hashLen uint16) (*tls.Config) {
	verify := func(_ [][]byte, certChains [][]*x509.Certificate) error {
		for _, certs := range certChains {
			for _, c := range certs {
				kadID, err := CertToKadID(c, hashLen)
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

// LoadID reads and parses an "identity" file containing a tls certificate
// chain (leaf-first), private key, and hash length for the "identity file"
// at `path`.
//
// The "identity file" must contain PEM encoded data. The certificate portion
// may contain intermediate certificates following the leaf certificate to
// form a certificate chain.
func LoadID(path string) (*KadCreds, error) {
	baseDir := filepath.Dir(path)

	if _, err := os.Stat(baseDir); err != nil {
		if err == os.ErrNotExist {
			return nil, peertls.ErrNotExist.Wrap(err)
		}

		return nil, errs.Wrap(err)
		// if err := os.MkdirAll(baseDir, 600); err != nil {
		// 	return nil, errs.Wrap(err)
		// }
	}

	IDBytes, err := ioutil.ReadFile(path)
	if err != nil {
		return nil, peertls.ErrNotExist.Wrap(err)
	}

	cert, hashLen, err := read(IDBytes)
	kadCreds, err := CertToKadCreds(cert, hashLen)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	return kadCreds, nil
}

// Save saves the certificate chain (leaf-first), private key, and
// hash length (ordered respectively) from `KadCreds` to a single
// PEM-encoded "identity file".
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

	defer func() {
		if err := file.Close(); err != nil {
			zap.S().Error(errs.Wrap(err))
		}
	}()

	if err = k.write(file); err != nil {
		return err
	}

	return nil
}

// func LoadOrCreateID(basePath string, minDifficulty uint16) (dht.NodeID, error) {
// 	nodeID, err := LoadID(basePath)
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
// 			nodeID, err := CertToKadCreds(pubKey, 256)
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

// CertToKadCreds takes a tls certificate pointer and a hash length to build a `KadCreds` struct
func CertToKadCreds(cert *tls.Certificate, hashLen uint16) (*KadCreds, error) {
	pubKey, ok := cert.Leaf.PublicKey.(*ecdsa.PublicKey)
	if pubKey == nil || !ok {
		return nil, errs.New("unsupported public key type (type assertion to `*ecdsa.PublicKey` failed)")
	}

	keyBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return nil, errs.New("unable to marshal pubKey", err)
	}

	hashBytes, err := hash(keyBytes, hashLen)
	if err != nil {
		return nil, err
	}

	tlsH, err := peertls.NewTLSHelper(cert)
	if err != nil {
		return nil, err
	}

	kadCreds := &KadCreds{
		tlsH:    tlsH,
		hash:    hashBytes,
		hashLen: hashLen,
	}

	kadCreds.tlsH.BaseConfig = baseConfig(kadCreds.Difficulty(), hashLen)

	return kadCreds, nil
}

func CertToKadID(cert *x509.Certificate, hashLen uint16) (*KadID, error) {
	pubKey, err := x509.MarshalPKIXPublicKey(cert.PublicKey)
	if err != nil {
		return nil, errs.Wrap(err)
	}

	hashBytes, err := hash(pubKey, hashLen)
	if err != nil {
		return nil, err
	}

	kadID := &KadID{
		pubKey:  pubKey,
		hash:    hashBytes,
		hashLen: hashLen,
	}

	return kadID, nil
}

// ParseID parses a `KadID` from its `String()` representation (i.e.
// base64-url-encoded concatenation of hash, public key, and hash
// length).
func ParseID(id string) (*KadID, error) {
	kadID := &KadID{}

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
	if err := binary.Read(lengthsSectionReader, binary.LittleEndian, &hashLen); err != nil {
		// TODO(bryanchriswhite): error handling
		// ensure hashLen exist
		return nil, ErrInvalidNodeID.Wrap(err)
	}

	kadID.hashLen = hashLen
	keyLen := idBytesReader.Size() - int64(hashLen) - int64(lenSize)

	// TODO(bryanchriswhite): ensure `keyLen` is reasonable (e.g. > 0, etc.)

	hashBytes := make([]byte, hashLen)
	keyBytes := make([]byte, keyLen)

	hashSectionReader := io.NewSectionReader(idBytesReader, 0, int64(hashLen))
	if _, err := hashSectionReader.Read(hashBytes); err != nil {
		// TODO(bryanchriswhite): error handling
		if err != io.EOF {
			return nil, ErrInvalidNodeID.Wrap(err)
		}
	}

	keySectionReader := io.NewSectionReader(idBytesReader, int64(hashLen), int64(keyLen))
	if _, err := keySectionReader.Read(keyBytes); err != nil {
		// TODO(bryanchriswhite): error handling
		if err != io.EOF {
			return nil, ErrInvalidNodeID.Wrap(err)
		}
	}

	kadID.hash = hashBytes
	kadID.pubKey = keyBytes

	return kadID, nil
}

// String serializes the hash, public key, and hash length into a PEM-encoded string
func (k *KadCreds) String() string {
	return string(k.Bytes())
}

// Bytes serializes the hash, public key, and hash length into a PEM-encoded byte-slice
func (k *KadCreds) Bytes() []byte {
	p := k.tlsH.PubKey()
	pubKey, err := x509.MarshalPKIXPublicKey(&p)
	if err != nil {
		zap.S().Error(errs.New("unable to marshal public key"))
	}

	b := bytes.NewBuffer([]byte{})
	encoder := base64.NewEncoder(base64.URLEncoding, b)
	if _, err := encoder.Write(k.hash); err != nil {
		zap.S().Error(errs.Wrap(err))
	}

	if _, err := encoder.Write(pubKey); err != nil {
		zap.S().Error(errs.Wrap(err))
	}

	if err := binary.Write(encoder, binary.LittleEndian, k.hashLen); err != nil {
		zap.S().Error(errs.Wrap(err))
	}

	if err := encoder.Close(); err != nil {
		zap.S().Error(errs.Wrap(err))
	}

	return b.Bytes()
}

// Hash returns the hash the public key to a langth of `k.hashLen`
func (k *KadCreds) Hash() []byte {
	return k.hash
}

// Difficulty returns the number of trailing zero-value bytes in the hash
func (k *KadCreds) Difficulty() uint16 {
	hash := k.Hash()
	for i := 1; i < len(hash); i++ {
		b := hash[len(hash)-i]

		if b != 0 {
			return uint16(i - 1)
		}
	}

	// NB: this should never happen
	return 0
}

// String serializes the hash, public key, and hash length into a PEM-encoded string
func (k *KadID) String() string {
	return string(k.Bytes())
}

// Bytes serializes the hash, public key, and hash length into a PEM-encoded byte-slice
func (k *KadID) Bytes() []byte {
	b := bytes.NewBuffer([]byte{})
	encoder := base64.NewEncoder(base64.URLEncoding, b)
	if _, err := encoder.Write(k.hash); err != nil {
		zap.S().Error(errs.Wrap(err))
	}

	if _, err := encoder.Write(k.pubKey); err != nil {
		zap.S().Error(errs.Wrap(err))
	}

	if err := binary.Write(encoder, binary.LittleEndian, k.hashLen); err != nil {
		zap.S().Error(errs.Wrap(err))
	}

	if err := encoder.Close(); err != nil {
		zap.S().Error(errs.Wrap(err))
	}

	return b.Bytes()
}

// Hash returns the hash the public key to a langth of `k.hashLen`
func (k *KadID) Hash() []byte {
	return k.hash
}

// Difficulty returns the number of trailing zero-value bytes in the hash
func (k *KadID) Difficulty() uint16 {
	hash := k.Hash()
	for i := 1; i < len(hash); i++ {
		b := hash[len(hash)-i]

		if b != 0 {
			return uint16(i - 1)
		}
	}

	// NB: this should never happen
	return 0
}

// NewID returns a pointer to a newly intialized, NodeID with at least the
// given difficulty
func NewID(difficulty uint16, hashLen uint16, concurrency uint, rootKeyPath string) (dht.NodeID, error) {
	done := make(chan bool, 0)
	c := make(chan KadCreds, 1)
	for i := 0; i < int(concurrency); i++ {
		go generateCreds(difficulty, hashLen, c, done)
	}

	kadCreds, _ := <-c
	close(done)

	// TODO(bryanchriswhite): write `tlsH.RootKey()` to `rootKeyPath`

	return &kadCreds, nil
}

func generateCreds(difficulty, hashLen uint16, c chan KadCreds, done chan bool) {
	for {
		select {
		case <-done:

			return
		default:
			tlsH, _ := peertls.NewTLSHelper(nil)

			cert := tlsH.Certificate()
			kadCreds, _ := CertToKadCreds(&cert, hashLen)
			kadCreds.tlsH.BaseConfig = baseConfig(kadCreds.Difficulty(), hashLen)

			if kadCreds.Difficulty() >= difficulty {
				c <- *kadCreds
			}
		}
	}
}

func (k *KadCreds) write(writer io.Writer) error {
	for _, c := range k.tlsH.Certificate().Certificate {
		certBlock := peertls.NewCertBlock(c)

		if err := pem.Encode(writer, certBlock); err != nil {
			return errs.Wrap(err)
		}
	}

	keyDERBytes, err := peertls.KeyToDERBytes(
		k.tlsH.Certificate().PrivateKey.(*ecdsa.PrivateKey),
	)
	if err != nil {
		return err
	}

	if err := pem.Encode(writer, peertls.NewKeyBlock(keyDERBytes)); err != nil {
		return errs.Wrap(err)
	}

	// Write `hashLen` after private key
	return binary.Write(writer, binary.LittleEndian, k.hashLen)
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
