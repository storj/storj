// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package encryption

import (
	"github.com/zeebo/errs"
)

// Error is the default encryption errs class
var Error = errs.Class("encryption error")

// DecryptionFailedError is the errs class when the decryption fails
var DecryptionFailedError = errs.Class("decryption failed, check encryption key")

// InvalidEncryptionTypeError is the errs class for invalid encryption type
var InvalidEncryptionTypeError = errs.Class("invalid encryption type")

// EncryptedBlockSizeTooSmallError is the errs class for too small size of the encrypted block
var EncryptedBlockSizeTooSmallError = errs.Class("encrypted block size too small")
