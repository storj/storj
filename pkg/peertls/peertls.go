// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package peertls

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/asn1"
	"math/big"
	"os"

	"github.com/zeebo/errs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
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

// TLSHelper stores information about a tls certificate and key, and options for use with tls helper functions/methods
type TLSHelper struct {
	cert    tls.Certificate
	rootKey *ecdsa.PrivateKey
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

// NewTLSHelper initializes a new `TLSHelper` struct with a new certificate
func NewTLSHelper(cert *tls.Certificate) (*TLSHelper, error) {
	var (
		c       tls.Certificate
		err     error
		rootKey *ecdsa.PrivateKey = nil
	)

	if cert == nil {
		c, rootKey, err = generateTLS()
		if err != nil {
			return nil, err
		}
	} else {
		c = *cert
	}

	t := &TLSHelper{
		cert:    c,
		rootKey: rootKey,
	}

	return t, nil
}

func (t *TLSHelper) NewTLSConfig(c *tls.Config) *tls.Config {
	config := cloneTLSConfig(c)

	config.Certificates = []tls.Certificate{t.cert}
	// Skip normal verification
	config.InsecureSkipVerify = true
	// Required client certificate
	config.ClientAuth = tls.RequireAnyClientCert
	// Custom verification logic for *both* client and server
	config.VerifyPeerCertificate = VerifyPeerCertificate

	return config
}

func (t *TLSHelper) NewPeerTLS(config *tls.Config) credentials.TransportCredentials {
	return credentials.NewTLS(t.NewTLSConfig(config))
}

func (t *TLSHelper) DialOption() grpc.DialOption {
	return grpc.WithTransportCredentials(t.NewPeerTLS(nil))
}

func (t *TLSHelper) ServerOption() grpc.ServerOption {
	return grpc.Creds(t.NewPeerTLS(nil))
}

func (t *TLSHelper) PubKey() ecdsa.PublicKey {
	return t.cert.PrivateKey.(*ecdsa.PrivateKey).PublicKey
}

func (t *TLSHelper) Certificate() tls.Certificate {
	return t.cert
}

func (t *TLSHelper) RootKey() ecdsa.PrivateKey {
	if t.rootKey == nil {
		return ecdsa.PrivateKey{}
	}

	return *t.rootKey
}

func verifyCertSignature(parentCert, childCert *x509.Certificate) (bool, error) {
	pubKey := parentCert.PublicKey.(*ecdsa.PublicKey)
	signature := new(ecdsaSignature)

	if _, err := asn1.Unmarshal(childCert.Signature, signature); err != nil {
		return false, ErrVerifySignature.New("unable to unmarshal ecdsa signature", err)
	}

	h := crypto.SHA256.New()
	h.Write(childCert.RawTBSCertificate)
	digest := h.Sum(nil)

	isValid := ecdsa.Verify(pubKey, digest, signature.R, signature.S)

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
