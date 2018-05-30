// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package paths

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha512"
	"encoding/base64"
)

// Encrypt the given path with the given key
func Encrypt(path []string, key []byte) (encryptedPath []string, err error) {
	encryptedPath = make([]string, len(path))
	for i, seg := range path {
		encryptedPath[i], err = encrypt(seg, key)
		if err != nil {
			return nil, err
		}
		key = deriveSecret(key, seg)
	}
	return encryptedPath, nil
}

// Decrypt the given encrypted path with the given key
func Decrypt(encryptedPath []string, key []byte) (path []string, err error) {
	path = make([]string, len(encryptedPath))
	for i, seg := range encryptedPath {
		path[i], err = decrypt(seg, key)
		if err != nil {
			return nil, err
		}
		key = deriveSecret(key, path[i])
	}
	return path, nil
}

// DeriveKey derives the key for the given path from the given root key
func DeriveKey(key []byte, path []string) (derivedKey []byte) {
	derivedKey = key
	for _, seg := range path {
		derivedKey = deriveSecret(derivedKey, seg)
	}
	return derivedKey
}

func encrypt(text string, secret []byte) (cipherText string, err error) {
	key, nonce := getAESGCMKeyAndNonce(secret)
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
	key, nonce := getAESGCMKeyAndNonce(secret)
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

func getAESGCMKeyAndNonce(secret []byte) (key, nonce []byte) {
	mac := hmac.New(sha512.New, secret)
	mac.Write([]byte("enc"))
	key = mac.Sum(nil)[:32]
	mac.Reset()
	mac.Write([]byte("nonce"))
	nonce = mac.Sum(nil)[:12]
	return key, nonce
}

func deriveSecret(secret []byte, child string) []byte {
	mac := hmac.New(sha512.New, secret)
	mac.Write([]byte(child))
	return mac.Sum(nil)
}
