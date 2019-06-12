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

// EncryptPath encrypts the path using the provided cipher and looking up
// keys from the provided store and bucket.
func EncryptPath(bucket string, path paths.Unencrypted, cipher storj.Cipher, store *Store) (
	paths.Encrypted, error) {

	if cipher == storj.Unencrypted {
		return paths.NewEncrypted(path.Raw()), nil
	}

	_, consumed, base := store.LookupUnencrypted(bucket, path)
	if base == nil {
		return paths.Encrypted{}, errs.New("unable to find encryption base for: %q", path)
	}

	remaining, ok := path.Consume(consumed)
	if !ok {
		return paths.Encrypted{}, errs.New("unable to encrypt bucket path: %q", path)
	}

	var builder strings.Builder
	builder.WriteString(base.Encrypted.Raw())

	key := &base.Key
	for iter, i := remaining.Iterator(), 0; !iter.Done(); i++ {
		component := iter.Next()

		encComponent, err := encryptPathComponent(component, cipher, key)
		if err != nil {
			return paths.Encrypted{}, errs.Wrap(err)
		}
		key, err = DeriveKey(key, "path:"+component)
		if err != nil {
			return paths.Encrypted{}, errs.Wrap(err)
		}

		if builder.Len() > 0 || i > 0 {
			builder.WriteByte('/')
		}
		builder.WriteString(encComponent)
	}

	return paths.NewEncrypted(builder.String()), nil
}

// DecryptPath decrypts the path using the provided cipher and looking up
// keys from the provided store and bucket.
func DecryptPath(bucket string, path paths.Encrypted, cipher storj.Cipher, store *Store) (
	paths.Unencrypted, error) {

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

	var builder strings.Builder
	builder.WriteString(base.Unencrypted.Raw())

	key := &base.Key
	for iter, i := remaining.Iterator(), 0; !iter.Done(); i++ {
		component := iter.Next()

		unencComponent, err := decryptPathComponent(component, cipher, key)
		if err != nil {
			return paths.Unencrypted{}, errs.Wrap(err)
		}
		key, err = DeriveKey(key, "path:"+unencComponent)
		if err != nil {
			return paths.Unencrypted{}, errs.Wrap(err)
		}

		if builder.Len() > 0 || i > 0 {
			builder.WriteByte('/')
		}
		builder.WriteString(unencComponent)
	}

	return paths.NewUnencrypted(builder.String()), nil
}

// DeriveContentKey returns the content key for the passed in path by looking up
// the appropriate base key from the store and bucket and deriving the rest.
func DeriveContentKey(bucket string, path paths.Unencrypted, store *Store) (key *storj.Key, err error) {
	key, err = DerivePathKey(bucket, path, store)
	if err != nil {
		return nil, errs.Wrap(err)
	}
	key, err = DeriveKey(key, "content")
	return key, errs.Wrap(err)
}

// DerivePathKey returns the path key for the passed in path by looking up the
// appropriate base key from the store and bucket and deriving the rest.
func DerivePathKey(bucket string, path paths.Unencrypted, store *Store) (key *storj.Key, err error) {
	_, consumed, base := store.LookupUnencrypted(bucket, path)
	if base == nil {
		return nil, errs.New("unable to find encryption base for: %q", path)
	}

	remaining, ok := path.Consume(consumed)
	if !ok {
		return nil, errs.New("unable to derive path key for: %q", path)
	}

	key = &base.Key
	for iter := remaining.Iterator(); !iter.Done(); {
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
