// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package uplink

import (
	"storj.io/storj/pkg/storj"
)

const (
	defaultCipher = storj.EncAESGCM
)

// EncryptionAccess specifies the encryption details needed to encrypt or
// decrypt objects.
type EncryptionAccess struct {
	// Key is the base encryption key to be used for decrypting objects.
	Key storj.Key

	// EncryptedPathPrefix is the (possibly empty) encrypted version of the
	// path from the top of the storage Bucket to this point.
	EncryptedPathPrefix storj.Path

	// UnencryptedPathPrefix is the (possibly empty) unencrypted version of
	// the path from the top of the storage Bucket to this point.
	UnencryptedPathPrefix storj.Path
}
