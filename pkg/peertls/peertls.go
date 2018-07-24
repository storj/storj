// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package peertls

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/asn1"
	"fmt"
	"math/big"
	"os"

	"github.com/zeebo/errs"
)

var (
	// ErrNotExist is used when a file or directory doesn't exist
	ErrNotExist = errs.Class("file or directory not found error")
	// ErrNoOverwrite is used when `create == true && overwrite == false`
	// 	and tls certs/keys already exist at the specified paths
	ErrNoOverwrite = errs.Class("tls overwrite disabled error")
	// ErrGenerate is used when an error occured during cert/key generation
	ErrGenerate = errs.Class("tls generation error")
	// ErrTLSOptions is used inconsistently and should probably just be removed
	ErrTLSOptions = errs.Class("tls options error")
	// ErrTLSTemplate is used when an error occurs during tls template generation
	ErrTLSTemplate = errs.Class("tls template error")
	// ErrVerifyPeerCert is used when an error occurs during `VerifyPeerCertificate`
	ErrVerifyPeerCert = errs.Class("tls peer certificate verification error")
	// ErrVerifySignature is used when a cert-chain signature verificaion error occurs
	ErrVerifySignature = errs.Class("tls certificate signature verification error")
)

// IsNotExist checks that a file or directory does not exist
func IsNotExist(err error) bool {
	return os.IsNotExist(err) || ErrNotExist.Has(err)
}

// TLSFileOptions stores information about a tls certificate and key, and options for use with tls helper functions/methods
type TLSFileOptions struct {
	RootCertRelPath string
	RootCertAbsPath string
	LeafCertRelPath string
	LeafCertAbsPath string
	// NB: Populate absolute paths from relative paths,
	// 			with respect to pwd via `.EnsureAbsPaths`
	RootKeyRelPath  string
	RootKeyAbsPath  string
	LeafKeyRelPath  string
	LeafKeyAbsPath  string
	LeafCertificate *tls.Certificate
	// Create if cert or key nonexistent
	Create bool
	// Overwrite if `create` is true and cert and/or key exist
	Overwrite bool
}

type ecdsaSignature struct {
	R, S *big.Int
}

// VerifyPeerCertificate verifies that the provided raw certificates are valid
func VerifyPeerCertificate(rawCerts [][]byte, _ [][]*x509.Certificate) error {
	// Verify parent ID/sig
	// Verify leaf  ID/sig
	// Verify leaf signed by parent

	// TODO(bryanchriswhite): see "S/Kademlia extensions - Secure nodeId generation"
	// (https://www.pivotaltracker.com/story/show/158238535)

	for i, cert := range rawCerts {
		isValid := false

		if i < len(rawCerts)-1 {
			parentCert, err := x509.ParseCertificate(rawCerts[i+1])
			if err != nil {
				return ErrVerifyPeerCert.New("unable to parse certificate", err)
			}

			childCert, err := x509.ParseCertificate(cert)
			if err != nil {
				return ErrVerifyPeerCert.New("unable to parse certificate", err)
			}

			isValid, err = verifyCertSignature(parentCert, childCert)
			if err != nil {
				return ErrVerifyPeerCert.Wrap(err)
			}
		} else {
			rootCert, err := x509.ParseCertificate(cert)
			if err != nil {
				return ErrVerifyPeerCert.New("unable to parse certificate", err)
			}

			isValid, err = verifyCertSignature(rootCert, rootCert)
			if err != nil {
				return ErrVerifyPeerCert.Wrap(err)
			}
		}

		if !isValid {
			return ErrVerifyPeerCert.New("certificate chain signature verification failed")
		}
	}

	return nil
}

// NewTLSFileOptions initializes a new `TLSFileOption` struct given the arguments
func NewTLSFileOptions(baseCertPath, baseKeyPath string, create, overwrite bool) (_ *TLSFileOptions, _ error) {
	t := &TLSFileOptions{
		RootCertRelPath: fmt.Sprintf("%s.root.cert", baseCertPath),
		RootKeyRelPath:  fmt.Sprintf("%s.root.key", baseKeyPath),
		LeafCertRelPath: fmt.Sprintf("%s.leaf.cert", baseCertPath),
		LeafKeyRelPath:  fmt.Sprintf("%s.leaf.key", baseKeyPath),
		Overwrite:       overwrite,
		Create:          create,
	}

	if err := t.EnsureExists(); err != nil {
		return nil, err
	}

	return t, nil
}

func verifyCertSignature(parentCert, childCert *x509.Certificate) (bool, error) {
	pubkey := parentCert.PublicKey.(*ecdsa.PublicKey)
	signature := new(ecdsaSignature)

	if _, err := asn1.Unmarshal(childCert.Signature, signature); err != nil {
		return false, ErrVerifySignature.New("unable to unmarshal ecdsa signature", err)
	}

	h := crypto.SHA256.New()
	_, err := h.Write(childCert.RawTBSCertificate)
	if err != nil {
		return false, err
	}
	digest := h.Sum(nil)

	isValid := ecdsa.Verify(pubkey, digest, signature.R, signature.S)

	return isValid, nil
}

// * Copyright 2017 gRPC authors.
// * Licensed under the Apache License, Version 2.0 (the "License");
// * (see https://github.com/grpc/grpc-go/blob/v1.13.0/credentials/credentials_util_go18.go)
// cloneTLSConfig returns a shallow clone of the exported
// fields of cfg, ignoring the unexported sync.Once, which
// contains a mutex and must not be copied.
//
// If cfg is nil, a new zero tls.Config is returned.
func cloneTLSConfig(cfg *tls.Config) *tls.Config {
	if cfg == nil {
		return &tls.Config{}
	}

	return cfg.Clone()
}
