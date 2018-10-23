// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package encryption

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/base64"
	"path"

	"storj.io/storj/pkg/storj"
)

// EncryptPath encrypts the p path with the given key
func EncryptPath(p storj.Path, key *storj.Key) (encrypted storj.Path, err error) {
	comps := storj.PathComponents(p)
	for i, comp := range comps {
		comps[i], err = encryptPathComponent(comp, key)
		if err != nil {
			return "", err
		}
		key, err = DeriveKey(key, comp)
		if err != nil {
			return "", err
		}
	}
	return path.Join(comps...), nil
}

// DecryptPath decrypts the p path with the given key
func DecryptPath(p storj.Path, key *storj.Key) (decrypted storj.Path, err error) {
	comps := storj.PathComponents(p)
	for i, comp := range comps {
		comps[i], err = decryptPathComponent(comp, key)
		if err != nil {
			return "", err
		}
		key, err = DeriveKey(key, comps[i])
		if err != nil {
			return "", err
		}
	}
	return path.Join(comps...), nil
}

// DerivePathKey derives the key for the given depth from the given root key.
// This method must be called on an unencrypted path.
func DerivePathKey(path storj.Path, key *storj.Key, depth int) (derivedKey *storj.Key, err error) {
	if depth < 0 {
		return nil, Error.New("negative depth")
	}

	comps := storj.PathComponents(path)
	if depth > len(comps) {
		return nil, Error.New("depth greater than path length")
	}

	derivedKey = key
	for i := 0; i < depth; i++ {
		derivedKey, err = DeriveKey(derivedKey, comps[i])
		if err != nil {
			return nil, err
		}
	}
	return derivedKey, nil
}

// DeriveContentKey derives the key for the encrypted object data using the root key.
// This method must be called on an unencrypted path.
func DeriveContentKey(path storj.Path, key *storj.Key) (derivedKey *storj.Key, err error) {
	comps := storj.PathComponents(path)
	if len(comps) == 0 {
		return nil, Error.New("path is empty")
	}
	derivedKey, err = DerivePathKey(path, key, len(comps))
	if err != nil {
		return nil, err
	}
	derivedKey, err = DeriveKey(derivedKey, "content")
	if err != nil {
		return nil, err
	}
	return derivedKey, nil
}

func encryptPathComponent(comp string, key *storj.Key) (string, error) {
	// derive the key for the current path component
	derivedKey, err := DeriveKey(key, comp)
	if err != nil {
		return "", err
	}

	// use the derived key to derive the nonce
	mac := hmac.New(sha512.New, derivedKey[:])
	_, err = mac.Write([]byte("nonce"))
	if err != nil {
		return "", Error.Wrap(err)
	}

	nonce := new(AESGCMNonce)
	copy(nonce[:], mac.Sum(nil))

	// encrypt the path components with the parent's key and the derived nonce
	cipherText, err := EncryptAESGCM([]byte(comp), key, nonce)
	if err != nil {
		return "", Error.Wrap(err)
	}

	// keep the nonce together with the cipher text
	return base64.RawURLEncoding.EncodeToString(append(nonce[:], cipherText...)), nil
}

func decryptPathComponent(comp string, key *storj.Key) (string, error) {
	if comp == "" {
		return "", Error.New("empty cipher text")
	}

	data, err := base64.RawURLEncoding.DecodeString(comp)
	if err != nil {
		return "", Error.Wrap(err)
	}

	// extract the nonce from the cipher text
	nonce := new(AESGCMNonce)
	copy(nonce[:], data[:AESGCMNonceSize])

	decrypted, err := DecryptAESGCM(data[AESGCMNonceSize:], key, nonce)
	if err != nil {
		return "", Error.Wrap(err)
	}

	return string(decrypted), nil
}
