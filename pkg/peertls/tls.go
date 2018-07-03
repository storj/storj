// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package peertls

import (
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"path/filepath"
	"strings"

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
	RootCertRelPath   string
	RootCertAbsPath   string
	LeafCertRelPath   string
	LeafCertAbsPath   string
	ClientCertRelPath string
	ClientCertAbsPath string
	// NB: Populate absolute paths from relative paths,
	// 			with respect to pwd via `.EnsureAbsPaths`
	RootKeyRelPath    string
	RootKeyAbsPath    string
	LeafKeyRelPath    string
	LeafKeyAbsPath    string
	ClientKeyRelPath  string
	ClientKeyAbsPath  string
	LeafCertificate   *tls.Certificate
	ClientCertificate *tls.Certificate
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

	// TODO(bryanchriswhite): see "S/Kademlia extensions - Secure nodeId generation"
	// (https://www.pivotaltracker.com/story/show/158238535)
	fmt.Println("len(rawCerts):", len(rawCerts))

	return nil
}

// NewTLSFileOptions initializes a new `TLSFileOption` struct given the arguments
func NewTLSFileOptions(baseCertPath, baseKeyPath, hosts string, client, create, overwrite bool) (_ *TLSFileOptions, _ error) {
	t := &TLSFileOptions{
		RootCertRelPath:   fmt.Sprintf("%s.root.cert", baseCertPath),
		RootKeyRelPath:    fmt.Sprintf("%s.root.key", baseKeyPath),
		LeafCertRelPath:   fmt.Sprintf("%s.leaf.cert", baseCertPath),
		LeafKeyRelPath:    fmt.Sprintf("%s.leaf.key", baseKeyPath),
		ClientCertRelPath: fmt.Sprintf("%s.client.cert", baseCertPath),
		ClientKeyRelPath:  fmt.Sprintf("%s.client.key", baseKeyPath),
		Client:            client,
		Overwrite:         overwrite,
		Create:            create,
		Hosts:             hosts,
	}

	if err := t.EnsureExists(); err != nil {
		return nil, err
	}

	return t, nil
}

func (t *TLSFileOptions) pathMap() (_ map[*string]string) {
	return map[*string]string{
		&t.RootCertAbsPath:   t.RootCertRelPath,
		&t.RootKeyAbsPath:    t.RootKeyRelPath,
		&t.LeafCertAbsPath:   t.LeafCertRelPath,
		&t.LeafKeyAbsPath:    t.LeafKeyRelPath,
		&t.ClientCertAbsPath: t.ClientCertRelPath,
		&t.ClientKeyAbsPath:  t.ClientKeyRelPath,
	}
}

func (t *TLSFileOptions) pathRoleMap() (_ map[*string]fileRole) {
	return map[*string]fileRole{
		&t.RootCertAbsPath:   rootCert,
		&t.RootKeyAbsPath:    rootKey,
		&t.LeafCertAbsPath:   leafCert,
		&t.LeafKeyAbsPath:    leafKey,
		&t.ClientCertAbsPath: clientCert,
		&t.ClientKeyAbsPath:  clientKey,
	}
}

// EnsureAbsPath ensures that the absolute path fields are not empty, deriving them from the relative paths if not
func (t *TLSFileOptions) EnsureAbsPaths() (_ error) {
	for _, role := range t.requiredFiles() {
		for absPtr, relPath := range t.pathMap() {
			if t.pathRoleMap()[absPtr] == role {
				if *absPtr == "" {
					if relPath == "" {
						return ErrTLSOptions.New("No relative %s path provided", fileLabels[t.pathRoleMap()[absPtr]])
					}

					absPath, err := filepath.Abs(relPath)
					if err != nil {
						return ErrTLSOptions.Wrap(err)
					}

					*absPtr = absPath
				}
			}
		}
	}

	return nil
}

// EnsureExists checks whether the cert/key exists and whether to create/overwrite them given `t`s field values.
func (t *TLSFileOptions) EnsureExists() (_ error) {
	if err := t.EnsureAbsPaths(); err != nil {
		return err
	}

	hasRequiredFiles, err := t.hasRequiredFiles()
	if err != nil {
		return err
	}

	if t.Create && !t.Overwrite && hasRequiredFiles {
		return ErrNoOverwrite.New("certificates and keys exist; refusing to create without overwrite")
	}

	// NB: even when `overwrite` is false, this WILL overwrite
	//      a key if the cert is missing (vice versa)
	if t.Create && (t.Overwrite || !hasRequiredFiles) {
		if err := t.generateTLS(); err != nil {
			return err
		}

		return nil
	}

	if !hasRequiredFiles {
		missing, _ := t.missingFiles()

		return ErrNotExist.New(fmt.Sprintf(strings.Join(missing, ", ")))
	}

	// NB: ensure `t.Certificate` is not nil when create is false
	if !t.Create {
		t.loadTLS()
	}

	return nil
}

func (t *TLSFileOptions) loadTLS() (_ error) {
	if t.Client {
		clientC, err := LoadCert(t.ClientCertAbsPath, t.ClientKeyAbsPath)
		if err != nil {
			return err
		}

		t.ClientCertificate = clientC
	} else {
		leafC, err := LoadCert(t.LeafCertAbsPath, t.LeafKeyAbsPath)
		if err != nil {
			return err
		}

		t.LeafCertificate = leafC
	}

	return nil
}

func (t *TLSFileOptions) NewTLSConfig(c *tls.Config) *tls.Config {
	config := cloneTLSConfig(c)

	// TODO(bryanchriswhite): more
	if t.Client {
		config.Certificates = []tls.Certificate{*t.ClientCertificate}
	} else {
		config.Certificates = []tls.Certificate{*t.LeafCertificate}
	}

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

func (t *TLSFileOptions) missingFiles() (_ []string, _ error) {
	missingFiles := []string{}

	paths := map[fileRole]string{
		rootCert:   t.RootCertAbsPath,
		rootKey:    t.RootKeyAbsPath,
		leafCert:   t.LeafCertAbsPath,
		leafKey:    t.LeafKeyAbsPath,
		clientCert: t.ClientCertAbsPath,
		clientKey:  t.ClientKeyAbsPath,
	}

	requiredFiles := t.requiredFiles()

	for _, requiredRole := range requiredFiles {
		for role, path := range paths {
			if role == requiredRole {
				if _, err := os.Stat(path); err != nil {
					if !IsNotExist(err) {
						return nil, errs.Wrap(err)
					}

					missingFiles = append(missingFiles, fileLabels[role])
				}
			}
		}
	}

	return missingFiles, nil
}

func (t *TLSFileOptions) requiredFiles() (_ []fileRole) {
	var roles = []fileRole{}

	// rootCert is always required
	roles = append(roles, rootCert)

	if t.Create {
		// required for writing rootKey when create is true
		roles = append(roles, rootKey)
	}

	if t.Client {
		roles = append(roles, clientCert, clientKey)
	} else {
		roles = append(roles, leafCert, leafKey)
	}

	return roles
}

func (t *TLSFileOptions) hasRequiredFiles() (_ bool, _ error) {
	missingFiles, err := t.missingFiles()
	if err != nil {
		return false, err
	}

	return len(missingFiles) == 0, nil
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
