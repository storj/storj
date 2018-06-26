// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package peertls

import (
	"crypto/tls"
	"crypto/x509"
	"os"
	"path/filepath"

	"github.com/zeebo/errs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

var (
	ErrNotExist    = errs.Class("file or directory not found error")
	ErrNoCreate    = errs.Class("tls creation disabled error")
	ErrNoOverwrite = errs.Class("tls overwrite disabled error")
	ErrBadHost     = errs.Class("bad host error")
	ErrGenerate    = errs.Class("tls generation error")
	ErrCredentials = errs.Class("grpc credentials error")
	ErrTLSOptions  = errs.Class("tls options error")
	ErrTLSTemplate = errs.Class("tls template error")
)

func IsNotExist(err error) bool {
	return os.IsNotExist(err) || ErrNotExist.Has(err)
}

// TLSFileOptions stores information about a tls certificate and key, and options for use with tls helper functions/methods
type TLSFileOptions struct {
	CertRelPath string
	CertAbsPath string
	Certificate *tls.Certificate
	// NB: Populate absolute paths from relative paths,
	// 			with respect to pwd via `.EnsureAbsPaths`
	KeyRelPath string
	KeyAbsPath string
	// Comma-separated list of hostname(s) (IP or FQDN)
	Hosts string
	// If true, key is not required or checked
	Client bool
	// Create if cert or key nonexistent
	Create bool
	// Overwrite if `create` is true and cert and/or key exist
	Overwrite bool
}

func VerifyPeerCertificate(rawCerts [][]byte, _ [][]*x509.Certificate) error {
	// Verify parent ID/sig
	// Verify leaf  ID/sig
	// Verify leaf signed by parent

	// b64 := base64.URLEncoding.EncodeToString(rawCerts[0])
	// fmt.Printf("rawCerts:%s\n", b64)
	fmt.Println("rawCerts:", rawCerts)
	// parentCert, err := x509.ParseCertificate(rawCerts[0])
	// leafCert, err := x509.ParseCertificate(rawCerts[1])
	// parentCert.Signature
	// parentCert.PublicKey
	return nil
}

// NewTLSFileOptions initializes a new `TLSFileOption` struct given the arguments
func NewTLSFileOptions(certPath, keyPath, hosts string, client, create, overwrite bool) (_ *TLSFileOptions, _ error) {
	t := &TLSFileOptions{
		CertRelPath: certPath,
		KeyRelPath:  keyPath,
		Client:      client,
		Overwrite:   overwrite,
		Create:      create,
		Hosts:       hosts,
	}

	if err := t.EnsureAbsPaths(); err != nil {
		return nil, err
	}

	if err := t.EnsureExists(); err != nil {
		return nil, err
	}

	return t, nil
}

// EnsureAbsPath ensures that the absolute path fields are not empty, deriving them from the relative paths if not
func (t *TLSFileOptions) EnsureAbsPaths() (_ error) {
	if t.CertAbsPath == "" {
		if t.CertRelPath == "" {
			return ErrTLSOptions.New("No relative certificate path provided")
		}

		certAbsPath, err := filepath.Abs(t.CertRelPath)
		if err != nil {
			return ErrTLSOptions.Wrap(err)
		}

		t.CertAbsPath = certAbsPath
	}

	if t.KeyAbsPath == "" {
		if t.KeyRelPath == "" {
			return ErrTLSOptions.New("No relative key path provided")
		}

		keyAbsPath, err := filepath.Abs(t.KeyRelPath)
		if err != nil {
			return errs.New(err.Error())
		}

		t.KeyAbsPath = keyAbsPath
	}

	return nil
}

// EnsureExists checks whether the cert/key exists and whether to create/overwrite them given `t`s fields
func (t *TLSFileOptions) EnsureExists() (_ error) {
	// Assume cert and key exist
	certMissing, keyMissing := false, false
	var lastErr error

	if err := t.EnsureAbsPaths(); err != nil {
		return err
	}

	if _, err := os.Stat(t.CertAbsPath); err != nil {
		lastErr = ErrNoCreate.Wrap(err)
		certMissing = true
	}

	if _, err := os.Stat(t.KeyAbsPath); err != nil {
		if !IsNotExist(err) {
			return errs.New(err.Error())
		}

		lastErr = ErrNoCreate.Wrap(err)
		keyMissing = true
	}

	if t.Create && !t.Overwrite && (!certMissing || !keyMissing) {
		return ErrNoOverwrite.New("certificate and key exist; refusing to create without overwrite")
	}

	// NB: even when `overwrite` is false, this WILL overwrite
	//      a key if the cert is missing (vice versa)
	if t.Create && (t.Overwrite || certMissing || keyMissing) {
		_, err := t.generateTLS()
		if err != nil {
			return err
		}

		return nil
	}

	if certMissing || keyMissing {
		return ErrNotExist.Wrap(lastErr)
	}

	// NB: ensure `t.Certificate` is not nil when create is false
	if !t.Create {
		cert, err := tls.LoadX509KeyPair(t.CertAbsPath, t.KeyAbsPath)
		if err != nil {
			return err
		}

		t.Certificate = &cert
	}

	return nil
}

func (t *TLSFileOptions) NewTLSConfig(c *tls.Config) *tls.Config {
	config := cloneTLSConfig(c)
	config.Certificates = []tls.Certificate{*t.Certificate}
	config.InsecureSkipVerify = true
	config.VerifyPeerCertificate = VerifyPeerCertificate

	return config
}

func (t *TLSFileOptions) NewPeerTLS(config *tls.Config) (_ credentials.TransportCredentials) {
	creds := credentials.NewTLS(t.NewTLSConfig(config))

	return creds
}

func (t *TLSFileOptions) DialOption() (_ grpc.DialOption) {
	creds := t.NewPeerTLS(nil)

	return grpc.WithTransportCredentials(creds)
}

func (t *TLSFileOptions) ServerOption() (_ grpc.ServerOption) {
	creds := t.NewPeerTLS(nil)

	return grpc.Creds(creds)
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
