// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package client

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"

	bip39 "github.com/tyler-smith/go-bip39"
)

const (
	bucketNameMagic = "398734aab3c4c30c9f22590e83a95f7e43556a45fc2b3060e0c39fde31f50272"
	gcmDigestSize   = 16
	ivSize          = 32
)

var bucketMetaMagic = []byte{
	66, 150, 71, 16, 50, 114, 88, 160, 163, 35, 154, 65, 162, 213, 226, 215,
	70, 138, 57, 61, 52, 19, 210, 170, 38, 164, 162, 200, 86, 201, 2, 81}

func sha256Sum(str string) string {
	checksum := sha256.Sum256([]byte(str))
	return hex.EncodeToString(checksum[:])
}

func mnemonicToSeed(mnemonic string) (string, error) {
	seed, err := bip39.NewSeedWithErrorChecking(mnemonic, "")
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(seed), nil
}

func getDeterministicKey(key, id string) (string, error) {
	input, err := hex.DecodeString(key + id)
	if err != nil {
		return "", err
	}
	checksum := sha512.Sum512(input)
	str := hex.EncodeToString(checksum[:])
	return str[0 : len(str)/2], nil
}

func generateBucketKey(mnemonic, bucketID string) (string, error) {
	seed, err := mnemonicToSeed(mnemonic)
	if err != nil {
		return "", err
	}
	return getDeterministicKey(seed, bucketNameMagic)
}

func decryptMeta(encryptedName string, key []byte) ([]byte, error) {
	nameBase64, err := base64.StdEncoding.DecodeString(encryptedName)
	if err != nil {
		return []byte{}, err
	}
	if len(nameBase64) <= gcmDigestSize+ivSize {
		return []byte{}, CryptoError.New("Invalid encrypted name")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return []byte{}, err
	}
	aesgcm, err := cipher.NewGCMWithNonceSize(block, ivSize)
	if err != nil {
		return []byte{}, err
	}
	digest := nameBase64[:gcmDigestSize]
	iv := nameBase64[gcmDigestSize : gcmDigestSize+ivSize]
	cipherText := nameBase64[gcmDigestSize+ivSize:]
	return aesgcm.Open(nil, iv, append(cipherText, digest...), nil)
}

func decryptBucketName(name, mnemonic string) (string, error) {
	bucketKey, err := generateBucketKey(mnemonic, bucketNameMagic)
	if err != nil {
		return "", err
	}
	bucketKeyHex, err := hex.DecodeString(bucketKey)
	if err != nil {
		return "", err
	}
	sig := hmac.New(sha512.New, bucketKeyHex)
	_, err = sig.Write(bucketMetaMagic)
	if err != nil {
		return "", err
	}
	hmacSHA512 := sig.Sum(nil)
	key := hmacSHA512[0 : len(hmacSHA512)/2]
	decryptedName, err := decryptMeta(name, key)
	if err != nil {
		return "", err
	}
	return string(decryptedName), nil
}
