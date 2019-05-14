// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pkcrypto

import "crypto/rand"

// GenerateSalt generates a secure cryptographic random salt value of size
// length.
//
// An error is returned if size is less than 8 or if rand reader returns an
// error.
func GenerateSalt(size uint32) ([]byte, error) {
	if size < 8 {
		return nil, ErrSalt.New("size must be greater or equal than 8, got %d", size)
	}

	salt := make([]byte, size)
	_, err := rand.Read(salt)
	if err != nil {
		return nil, ErrSalt.Wrap(err)
	}

	return salt, nil
}
