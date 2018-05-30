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
	secret := key
	for i, seg := range path {
		encryptedPath[i], err = encrypt(seg, secret)
		if err != nil {
			return nil, err
		}
		secret = deriveSecret(secret, seg)
	}
	return encryptedPath, nil
}

// Decrypt the given encrypted path with the given key
func Decrypt(encryptedPath []string, key []byte) (path []string, err error) {
	path = make([]string, len(encryptedPath))
	secret := key
	for i, seg := range encryptedPath {
		path[i], err = decrypt(seg, secret)
		if err != nil {
			return nil, err
		}
		secret = deriveSecret(secret, path[i])
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
		return "", err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	data := aesgcm.Seal(nil, nonce, []byte(text), nil)
	cipherText = base64.RawURLEncoding.EncodeToString(data)
	return cipherText, nil
}

func decrypt(cipherText string, secret []byte) (text string, err error) {
	key, nonce := getAESGCMKeyAndNonce(secret)
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	data, err := base64.RawURLEncoding.DecodeString(cipherText)
	if err != nil {
		return "", err
	}
	decrypted, err := aesgcm.Open(nil, nonce, data, nil)
	if err != nil {
		return "", err
	}
	text = string(decrypted)
	return text, nil
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
