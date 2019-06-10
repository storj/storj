// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package encryption

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/base64"
	"strings"

	"github.com/zeebo/errs"
	"storj.io/storj/pkg/storj"
)

// EncryptBucketPath encrypts the bucketPath using the provided cipher and looking up
// keys from the provided store.
func EncryptBucketPath(bucketPath storj.UnencryptedBucketPath, cipher storj.Cipher, store *Store) (
	storj.EncryptedBucketPath, error) {

	bucket := bucketPath.Bucket()
	if cipher == storj.Unencrypted {
		return storj.NewEncryptedPath(bucketPath.Path().Raw()).WithBucket(bucket), nil
	}

	_, consumed, base := store.LookupUnencrypted(bucketPath)
	if base == nil {
		return storj.EncryptedBucketPath{}, errs.New(
			"unable to find encryption base for: %q", bucketPath)
	}

	path, ok := bucketPath.Path().Consume(consumed)
	if !ok {
		return storj.EncryptedBucketPath{}, errs.New(
			"unable to encrypt bucket path: %q", bucketPath)
	}

	var builder strings.Builder
	builder.WriteString(base.Encrypted.Raw())

	key := &base.Key
	for iter, i := path.Iterator(), 0; !iter.Done(); i++ {
		component := iter.Next()

		encComponent, err := encryptPathComponent(component, cipher, key)
		if err != nil {
			return storj.EncryptedBucketPath{}, errs.Wrap(err)
		}
		key, err = DeriveKey(key, "path:"+component)
		if err != nil {
			return storj.EncryptedBucketPath{}, errs.Wrap(err)
		}

		if builder.Len() > 0 || i > 0 {
			builder.WriteByte('/')
		}
		builder.WriteString(encComponent)
	}

	return storj.NewEncryptedPath(builder.String()).WithBucket(bucket), nil
}

// DecryptBucketPath decrypts the bucketPath using the provided cipher and looking up
// keys from the provided store.
func DecryptBucketPath(bucketPath storj.EncryptedBucketPath, cipher storj.Cipher, store *Store) (
	storj.UnencryptedBucketPath, error) {

	bucket := bucketPath.Bucket()
	if cipher == storj.Unencrypted {
		return storj.NewUnencryptedPath(bucketPath.Path().Raw()).WithBucket(bucket), nil
	}

	_, consumed, base := store.LookupEncrypted(bucketPath)
	if base == nil {
		return storj.UnencryptedBucketPath{}, errs.New(
			"unable to find encryption base for: %q", bucketPath)
	}

	path, ok := bucketPath.Path().Consume(consumed)
	if !ok {
		return storj.UnencryptedBucketPath{}, errs.New(
			"unable to encrpt bucket path: %q", bucketPath)
	}

	var builder strings.Builder
	builder.WriteString(base.Unencrypted.Raw())

	key := &base.Key
	for iter, i := path.Iterator(), 0; !iter.Done(); i++ {
		component := iter.Next()

		unencComponent, err := decryptPathComponent(component, cipher, key)
		if err != nil {
			return storj.UnencryptedBucketPath{}, errs.Wrap(err)
		}
		key, err = DeriveKey(key, "path:"+unencComponent)
		if err != nil {
			return storj.UnencryptedBucketPath{}, errs.Wrap(err)
		}

		if builder.Len() > 0 || i > 0 {
			builder.WriteByte('/')
		}
		builder.WriteString(unencComponent)
	}

	return storj.NewUnencryptedPath(builder.String()).WithBucket(bucket), nil
}

// DeriveContentKey returns the content key for the passed in bucketPath by looking up
// the appropriate base key from the store and deriving the rest.
func DeriveContentKey(bucketPath storj.UnencryptedBucketPath, store *Store) (
	key *storj.Key, err error) {

	key, err = DerivePathKey(bucketPath, store)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	key, err = DeriveKey(key, "content")
	return key, errs.Wrap(err)
}

// DerivePathKey returns the path key for the passed in bucketPath by looking up the
// appropriate base key from the store and deriving the rest.
func DerivePathKey(bucketPath storj.UnencryptedBucketPath, store *Store) (
	key *storj.Key, err error) {

	_, consumed, base := store.LookupUnencrypted(bucketPath)
	if base == nil {
		return nil, errs.New("unable to find encryption base for: %q", bucketPath)
	}

	path, ok := bucketPath.Path().Consume(consumed)
	if !ok {
		return nil, errs.New("unable to derive path key for: %q", bucketPath)
	}

	key = &base.Key
	for iter := path.Iterator(); !iter.Done(); {
		key, err = DeriveKey(key, "path:"+iter.Next())
		if err != nil {
			return nil, errs.Wrap(err)
		}
	}
	return key, nil
}

// encryptPathComponent encrypts a single path component with the provided cipher and key.
func encryptPathComponent(comp string, cipher storj.Cipher, key *storj.Key) (string, error) {
	// derive the key for the next path component. this is so that
	// every encrypted component has a unique nonce.
	//
	// TODO(jeff): could we have just written the path component into an
	// hmac keyed by the current path key? that seems like it would be
	// just as good for nonce generation, and wouldn't require deriving
	// the path key multiple times.
	derivedKey, err := DeriveKey(key, "path:"+comp)
	if err != nil {
		return "", err
	}

	// use the derived key to derive the nonce
	mac := hmac.New(sha512.New, derivedKey[:])
	_, err = mac.Write([]byte("nonce"))
	if err != nil {
		return "", Error.Wrap(err)
	}

	nonce := new(storj.Nonce)
	copy(nonce[:], mac.Sum(nil))

	// encrypt the path components with the parent's key and the derived nonce
	cipherText, err := Encrypt([]byte(comp), cipher, key, nonce)
	if err != nil {
		return "", Error.Wrap(err)
	}

	nonceSize := storj.NonceSize
	if cipher == storj.AESGCM {
		nonceSize = AESGCMNonceSize
	}

	// keep the nonce together with the cipher text
	return base64.RawURLEncoding.EncodeToString(append(nonce[:nonceSize], cipherText...)), nil
}

// decryptPathComponent decrypts a single path component with the provided cipher and key.
func decryptPathComponent(comp string, cipher storj.Cipher, key *storj.Key) (string, error) {
	data, err := base64.RawURLEncoding.DecodeString(comp)
	if err != nil {
		return "", Error.Wrap(err)
	}

	nonceSize := storj.NonceSize
	if cipher == storj.AESGCM {
		nonceSize = AESGCMNonceSize
	}

	// extract the nonce from the cipher text
	nonce := new(storj.Nonce)
	copy(nonce[:], data[:nonceSize])

	decrypted, err := Decrypt(data[nonceSize:], cipher, key, nonce)
	if err != nil {
		return "", Error.Wrap(err)
	}

	return string(decrypted), nil
}
