// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package peertls

import (
	"crypto/tls"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/zeebo/errs"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

type fileRole int

const (
	rootCert fileRole = iota
	rootKey
	leafCert
	leafKey
	clientCert
	clientKey
)

var (
	fileLabels = map[fileRole]string{
		rootCert:   "root certificate",
		rootKey:    "root key",
		leafCert:   "leaf certificate",
		leafKey:    "leaf key",
		clientCert: "client certificate",
		clientKey:  "client key",
	}
)

// EnsureAbsPath ensures that the absolute path fields are not empty, deriving them from the relative paths if not
func (t *TLSFileOptions) EnsureAbsPaths() (error) {
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
func (t *TLSFileOptions) EnsureExists() (error) {
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

func (t *TLSFileOptions) NewPeerTLS(config *tls.Config) (credentials.TransportCredentials) {
	return credentials.NewTLS(t.NewTLSConfig(config))
}

func (t *TLSFileOptions) DialOption() (grpc.DialOption) {
	return grpc.WithTransportCredentials(t.NewPeerTLS(nil))
}

func (t *TLSFileOptions) ServerOption() (grpc.ServerOption) {
	return grpc.Creds(t.NewPeerTLS(nil))
}

func (t *TLSFileOptions) loadTLS() (error) {
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

func (t *TLSFileOptions) missingFiles() ([]string, error) {
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

func (t *TLSFileOptions) requiredFiles() ([]fileRole) {
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

func (t *TLSFileOptions) hasRequiredFiles() (bool, error) {
	missingFiles, err := t.missingFiles()
	if err != nil {
		return false, err
	}

	return len(missingFiles) == 0, nil
}

func (t *TLSFileOptions) pathMap() (map[*string]string) {
	return map[*string]string{
		&t.RootCertAbsPath:   t.RootCertRelPath,
		&t.RootKeyAbsPath:    t.RootKeyRelPath,
		&t.LeafCertAbsPath:   t.LeafCertRelPath,
		&t.LeafKeyAbsPath:    t.LeafKeyRelPath,
		&t.ClientCertAbsPath: t.ClientCertRelPath,
		&t.ClientKeyAbsPath:  t.ClientKeyRelPath,
	}
}

func (t *TLSFileOptions) pathRoleMap() (map[*string]fileRole) {
	return map[*string]fileRole{
		&t.RootCertAbsPath:   rootCert,
		&t.RootKeyAbsPath:    rootKey,
		&t.LeafCertAbsPath:   leafCert,
		&t.LeafKeyAbsPath:    leafKey,
		&t.ClientCertAbsPath: clientCert,
		&t.ClientKeyAbsPath:  clientKey,
	}
}
