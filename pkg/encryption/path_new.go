// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package encryption

import (
	"crypto/hmac"
	"crypto/sha512"
	"encoding/base64"
	"strings"

	"github.com/zeebo/errs"
	"storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/storj"
)

// StoreEncryptPath encrypts the path using the provided cipher and looking up
// keys from the provided store and bucket.
func StoreEncryptPath(bucket string, path paths.Unencrypted, cipher storj.Cipher, store *Store) (
	encPath paths.Encrypted, err error) {

	// Invalid paths map to invalid paths
	if !path.Valid() {
		return paths.Encrypted{}, nil
	}

	if cipher == storj.Unencrypted {
		return paths.NewEncrypted(path.Raw()), nil
	}

	_, consumed, base := store.LookupUnencrypted(bucket, path)
	if base == nil {
		return paths.Encrypted{}, errs.New("unable to find encryption base for: %s/%q", bucket, path)
	}

	remaining, ok := path.Consume(consumed)
	if !ok {
		return paths.Encrypted{}, errs.New("unable to encrypt bucket path: %s/%q", bucket, path)
	}

	// if we didn't consume any path, we're at the root of the bucket, and so we have
	// to fold the bucket name into the key.
	key := &base.Key
	if !consumed.Valid() {
		key, err = DeriveKey(key, "path:"+bucket)
		if err != nil {
			return paths.Encrypted{}, errs.Wrap(err)
		}
	}

	encrypted, err := EncryptPathRaw(remaining.Raw(), cipher, key)
	if err != nil {
		return paths.Encrypted{}, errs.Wrap(err)
	}

	var builder strings.Builder
	builder.WriteString(base.Encrypted.Raw())

	if len(encrypted) > 0 {
		if builder.Len() > 0 {
			builder.WriteByte('/')
		}
		builder.WriteString(encrypted)
	}

	return paths.NewEncrypted(builder.String()), nil
}

// EncryptPathRaw encrypts the path using the provided key directly. EncryptPath should be
// preferred if possible.
func EncryptPathRaw(raw string, cipher storj.Cipher, key *storj.Key) (string, error) {
	if cipher == storj.Unencrypted {
		return raw, nil
	}

	var builder strings.Builder
	for iter, i := paths.NewIterator(raw), 0; !iter.Done(); i++ {
		component := iter.Next()
		encComponent, err := storeEncryptPathComponent(component, cipher, key)
		if err != nil {
			return "", errs.Wrap(err)
		}
		key, err = DeriveKey(key, "path:"+component)
		if err != nil {
			return "", errs.Wrap(err)
		}
		if i > 0 {
			builder.WriteByte('/')
		}
		builder.WriteString(encComponent)
	}
	return builder.String(), nil
}

// StoreDecryptPath decrypts the path using the provided cipher and looking up
// keys from the provided store and bucket.
func StoreDecryptPath(bucket string, path paths.Encrypted, cipher storj.Cipher, store *Store) (
	unencPath paths.Unencrypted, err error) {

	// Invalid paths map to invalid paths
	if !path.Valid() {
		return paths.Unencrypted{}, nil
	}

	if cipher == storj.Unencrypted {
		return paths.NewUnencrypted(path.Raw()), nil
	}

	_, consumed, base := store.LookupEncrypted(bucket, path)
	if base == nil {
		return paths.Unencrypted{}, errs.New("unable to find encryption base for: %q", path)
	}

	remaining, ok := path.Consume(consumed)
	if !ok {
		return paths.Unencrypted{}, errs.New("unable to encrpt bucket path: %q", path)
	}

	// if we didn't consume any path, we're at the root of the bucket, and so we have
	// to fold the bucket name into the key.
	key := &base.Key
	if !consumed.Valid() {
		key, err = DeriveKey(key, "path:"+bucket)
		if err != nil {
			return paths.Unencrypted{}, errs.Wrap(err)
		}
	}

	decrypted, err := DecryptPathRaw(remaining.Raw(), cipher, key)
	if err != nil {
		return paths.Unencrypted{}, errs.Wrap(err)
	}

	var builder strings.Builder
	builder.WriteString(base.Unencrypted.Raw())

	if len(decrypted) > 0 {
		if builder.Len() > 0 {
			builder.WriteByte('/')
		}
		builder.WriteString(decrypted)
	}

	return paths.NewUnencrypted(builder.String()), nil
}

// DecryptPathRaw decrypts the path using the provided key directly. DecryptPath should be
// preferred if possible.
func DecryptPathRaw(raw string, cipher storj.Cipher, key *storj.Key) (string, error) {
	if cipher == storj.Unencrypted {
		return raw, nil
	}

	var builder strings.Builder
	for iter, i := paths.NewIterator(raw), 0; !iter.Done(); i++ {
		component := iter.Next()
		unencComponent, err := storeDecryptPathComponent(component, cipher, key)
		if err != nil {
			return "", errs.Wrap(err)
		}
		key, err = DeriveKey(key, "path:"+unencComponent)
		if err != nil {
			return "", errs.Wrap(err)
		}
		if i > 0 {
			builder.WriteByte('/')
		}
		builder.WriteString(unencComponent)
	}
	return builder.String(), nil
}

// StoreDeriveContentKey returns the content key for the passed in path by looking up
// the appropriate base key from the store and bucket and deriving the rest.
func StoreDeriveContentKey(bucket string, path paths.Unencrypted, store *Store) (key *storj.Key, err error) {
	key, err = StoreDerivePathKey(bucket, path, store)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	key, err = DeriveKey(key, "content")
	return key, errs.Wrap(err)
}

// StoreDerivePathKey returns the path key for the passed in path by looking up the
// appropriate base key from the store and bucket and deriving the rest.
func StoreDerivePathKey(bucket string, path paths.Unencrypted, store *Store) (key *storj.Key, err error) {
	_, consumed, base := store.LookupUnencrypted(bucket, path)
	if base == nil {
		return nil, errs.New("unable to find encryption base for: %s/%q", bucket, path)
	}

	// If asking for the key at the bucket, do that and return.
	if !path.Valid() {
		key, err = DeriveKey(&base.Key, "path:"+bucket)
		if err != nil {
			return nil, errs.Wrap(err)
		}
		return key, nil
	}

	remaining, ok := path.Consume(consumed)
	if !ok {
		return nil, errs.New("unable to derive path key for: %s/%q", bucket, path)
	}

	// if we didn't consume any path, we're at the root of the bucket, and so we have
	// to fold the bucket name into the key.
	key = &base.Key
	if !consumed.Valid() {
		key, err = DeriveKey(key, "path:"+bucket)
		if err != nil {
			return nil, errs.Wrap(err)
		}
	}

	for iter := remaining.Iterator(); !iter.Done(); {
		key, err = DeriveKey(key, "path:"+iter.Next())
		if err != nil {
			return nil, errs.Wrap(err)
		}
	}
	return key, nil
}

// storeEncryptPathComponent encrypts a single path component with the provided cipher and key.
func storeEncryptPathComponent(comp string, cipher storj.Cipher, key *storj.Key) (string, error) {
	if comp == "" {
		return "", nil
	}

	// derive the key for the next path component. this is so that
	// every encrypted component has a unique nonce.
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

// storeDecryptPathComponent decrypts a single path component with the provided cipher and key.
func storeDecryptPathComponent(comp string, cipher storj.Cipher, key *storj.Key) (string, error) {
	if comp == "" {
		return "", nil
	}

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
