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
		key, err = derivePathSecret(key, comp)
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
		key, err = derivePathSecret(key, comps[i])
		if err != nil {
			return "", err
		}
	}
	return path.Join(comps...), nil
}

// DerivePathKey derives the key for the given depth from the given root key
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
		derivedKey, err = derivePathSecret(derivedKey, comps[i])
		if err != nil {
			return nil, err
		}
	}
	return derivedKey, nil
}

// DerivePathContentKey derives the key for the encrypted object data using the root key
// This method must be called on an unencrypted path.
func DerivePathContentKey(path storj.Path, key *storj.Key) (derivedKey *storj.Key, err error) {
	comps := storj.PathComponents(path)
	if len(comps) == 0 {
		return nil, Error.New("path is empty")
	}
	derivedKey, err = DerivePathKey(path, key, len(comps)-1)
	if err != nil {
		return nil, err
	}
	derivedKey, err = derivePathSecret(derivedKey, "content")
	if err != nil {
		return nil, err
	}
	return derivedKey, nil
}

func encryptPathComponent(comp storj.Path, secret *storj.Key) (encryptedComp storj.Path, err error) {
	key, nonce, err := getAESGCMKeyAndNonce(secret)
	if err != nil {
		return "", Error.Wrap(err)
	}
	data, err := EncryptAESGCM([]byte(comp), key, nonce)
	if err != nil {
		return "", Error.Wrap(err)
	}
	// prepend version number to the cipher text
	return "1" + base64.RawURLEncoding.EncodeToString(data), nil
}

func decryptPathComponent(encryptedComp storj.Path, secret *storj.Key) (comp storj.Path, err error) {
	if encryptedComp == "" {
		return "", Error.New("empty cipher text")
	}
	// check the version number, only "1" is supported for now
	if encryptedComp[0] != '1' {
		return "", Error.New("invalid version number")
	}
	data, err := base64.RawURLEncoding.DecodeString(encryptedComp[1:])
	if err != nil {
		return "", Error.Wrap(err)
	}
	key, nonce, err := getAESGCMKeyAndNonce(secret)
	if err != nil {
		return "", Error.Wrap(err)
	}
	decrypted, err := DecryptAESGCM(data, key, nonce)
	if err != nil {
		return "", Error.Wrap(err)
	}
	return string(decrypted), nil
}

func getAESGCMKeyAndNonce(secret *storj.Key) (key *storj.Key, nonce *AESGCMNonce, err error) {
	mac := hmac.New(sha512.New, secret[:])
	_, err = mac.Write([]byte("enc"))
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	key = new(storj.Key)
	copy(key[:], mac.Sum(nil))

	mac.Reset()
	_, err = mac.Write([]byte("nonce"))
	if err != nil {
		return nil, nil, Error.Wrap(err)
	}

	nonce = new(AESGCMNonce)
	copy(nonce[:], mac.Sum(nil))

	return key, nonce, nil
}

func derivePathSecret(secret *storj.Key, child string) (derived *storj.Key, err error) {
	mac := hmac.New(sha512.New, secret[:])
	_, err = mac.Write([]byte(child))
	if err != nil {
		return nil, Error.Wrap(err)
	}

	derived = new(storj.Key)
	copy(derived[:], mac.Sum(nil))

	return derived, nil
}
