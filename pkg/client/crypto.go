// Copyright (C) 2018 Storj Labs, Inc.
//
// This file is part of the Storj client library.
//
// The Storj client library is free software: you can redistribute it and/or
// modify it under the terms of the GNU Lesser General Public License as
// published by the Free Software Foundation, either version 3 of the
// License, or (at your option) any later version.
//
// The Storj client library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with The Storj client library.  If not, see
// <http://www.gnu.org/licenses/>.

package storj

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/sha256"
	"crypto/sha512"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"

	bip39 "github.com/tyler-smith/go-bip39"
)

const (
	bucketNameMagic = "398734aab3c4c30c9f22590e83a95f7e43556a45fc2b3060e0c39fde31f50272"
	gcmDigestSize   = 16
	ivSize          = 32
)

var bucketMetaMagic = []byte{66, 150, 71, 16, 50, 114, 88, 160, 163, 35, 154, 65, 162, 213, 226, 215, 70, 138, 57, 61, 52, 19, 210, 170, 38, 164, 162, 200, 86, 201, 2, 81}

func sha256Hash(str string) string {
	h := sha256.New()
	h.Write([]byte(str))
	return fmt.Sprintf("%x", h.Sum(nil))
}

func mnemonicToSeed(mnemonic string) (string, error) {
	seedHex, err := bip39.NewSeedWithErrorChecking(mnemonic, "")
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(seedHex), nil
}

func getDeterministicKey(key, id string) (string, error) {
	sha512Input := key + id
	sha512InputHex, err := hex.DecodeString(sha512Input)
	if err != nil {
		return "", err
	}
	hasher := sha512.New()
	hasher.Write(sha512InputHex)
	if err != nil {
		return "", err
	}
	sha512Str := hex.EncodeToString(hasher.Sum(nil))
	return sha512Str[0 : len(sha512Str)/2], nil
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
		return []byte{}, errors.New("Invalid encrypted name")
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
