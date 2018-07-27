// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package paths

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/base64"
	"path"
	"strings"
)

// Path is a unique identifier for an object stored in the Storj network
type Path []string

// New creates new Path from the given path segments
func New(segs ...string) Path {
	s := path.Join(segs...)
	s = strings.Trim(s, "/")
	p := strings.Split(s, "/")
	if len(p) == 1 && p[0] == "" {
		// Avoid building a path with a single empty segment
		return []string{}
	}
	return p
}

// String returns the string representation of the path
func (p Path) String() string {
	return path.Join([]string(p)...)
}

// Bytes serializes the current path to []byte
func (p Path) Bytes() []byte {
	return []byte(p.String())
}

// Prepend creates new Path from the current path with the given segments prepended
func (p Path) Prepend(segs ...string) Path {
	return New(append(segs, []string(p)...)...)
}

// Append creates new Path from the current path with the given segments appended
func (p Path) Append(segs ...string) Path {
	return New(append(p, segs...)...)
}

// Encrypt creates new Path by encrypting the current path with the given key
func (p Path) Encrypt(key []byte) (encrypted Path, err error) {
	encrypted = make([]string, len(p))
	for i, seg := range p {
		encrypted[i], err = encrypt(seg, key)
		if err != nil {
			return nil, err
		}
		key, err = deriveSecret(key, seg)
		if err != nil {
			return nil, err
		}
	}
	return encrypted, nil
}

// Decrypt creates new Path by decrypting the current path with the given key
func (p Path) Decrypt(key []byte) (decrypted Path, err error) {
	decrypted = make([]string, len(p))
	for i, seg := range p {
		decrypted[i], err = decrypt(seg, key)
		if err != nil {
			return nil, err
		}
		key, err = deriveSecret(key, decrypted[i])
		if err != nil {
			return nil, err
		}
	}
	return decrypted, nil
}

// DeriveKey derives the key for the given depth from the given root key
//
// This method must be called on an unencrypted path.
func (p Path) DeriveKey(key []byte, depth int) (derivedKey []byte, err error) {
	if depth < 0 {
		return nil, Error.New("negative depth")
	}
	if depth > len(p) {
		return nil, Error.New("depth greater than path length")
	}

	derivedKey = key
	for i := 0; i < depth; i++ {
		derivedKey, err = deriveSecret(derivedKey, p[i])
		if err != nil {
			return nil, err
		}
	}
	return derivedKey, nil
}

func encrypt(text string, secret []byte) (cipherText string, err error) {
	key, nonce, err := getAESGCMKeyAndNonce(secret)
	if err != nil {
		return "", Error.Wrap(err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", Error.Wrap(err)
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", Error.Wrap(err)
	}
	data := aesgcm.Seal(nil, nonce, []byte(text), nil)
	cipherText = base64.RawURLEncoding.EncodeToString(data)
	// prepend version number to the cipher text
	return "1" + cipherText, nil
}

func decrypt(cipherText string, secret []byte) (text string, err error) {
	if cipherText == "" {
		return "", Error.New("empty cipher text")
	}
	// check the version number, only "1" is supported for now
	if cipherText[0] != '1' {
		return "", Error.New("invalid version number")
	}
	data, err := base64.RawURLEncoding.DecodeString(cipherText[1:])
	if err != nil {
		return "", Error.Wrap(err)
	}
	key, nonce, err := getAESGCMKeyAndNonce(secret)
	if err != nil {
		return "", Error.Wrap(err)
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", Error.Wrap(err)
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", Error.Wrap(err)
	}
	decrypted, err := aesgcm.Open(nil, nonce, data, nil)
	if err != nil {
		return "", Error.Wrap(err)
	}
	return string(decrypted), nil
}

func getAESGCMKeyAndNonce(secret []byte) (key, nonce []byte, err error) {
	mac := hmac.New(sha512.New, secret)
	_, err = mac.Write([]byte("enc"))
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}
	key = mac.Sum(nil)[:32]
	mac.Reset()
	_, err = mac.Write([]byte("nonce"))
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}
	nonce = mac.Sum(nil)[:12]
	return key, nonce, nil
}

func deriveSecret(secret []byte, child string) (derived []byte, err error) {
	mac := hmac.New(sha512.New, secret)
	_, err = mac.Write([]byte(child))
	if err != nil {
		return nil, Error.Wrap(err)
	}
	return mac.Sum(nil), nil
}
