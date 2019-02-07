// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package pkcrypto

import (
	"github.com/zeebo/errs"
)

const (
	// BlockTypeEcPrivateKey is the value to define a block type of private key
	BlockTypeEcPrivateKey = "EC PRIVATE KEY"
	// BlockTypeCertificate is the value to define a block type of certificates
	BlockTypeCertificate = "CERTIFICATE"
	// BlockTypeExtension is the value to define a block type of certificate extensions
	BlockTypeExtension = "EXTENSION"
)

var (
	// ErrUnsupportedKey is used when key type is not supported.
	ErrUnsupportedKey = errs.Class("unsupported key type")
	// ErrParseCerts is used when an error occurs while parsing a certificate or cert chain.
	ErrParseCerts = errs.Class("unable to parse certificate")
	// ErrSign is used when something goes wrong while generating a signature.
	ErrSign = errs.Class("unable to generate signature")
	// ErrVerifySignature is used when a cert-chain signature verificaion error occurs.
	ErrVerifySignature = errs.Class("tls certificate signature verification error")
)
