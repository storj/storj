// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package peertls

import (
	"crypto/tls"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
)

func (t *TLSHelper) NewTLSConfig(c *tls.Config) *tls.Config {
	config := cloneTLSConfig(c)

	config.Certificates = []tls.Certificate{*t.cert}
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

// func (t *TLSHelper) loadTLS() (_ error) {
// 	leafC, err := LoadCert(t.LeafCertAbsPath, t.LeafKeyAbsPath)
// 	if err != nil {
// 		return err
// 	}
//
// 	t.cert = leafC
// 	return nil
// }

// func (t *TLSHelper) missingFiles() ([]string, error) {
// 	missingFiles := []string{}
//
// 	paths := map[fileRole]string{
// 		rootCert: t.RootCertAbsPath,
// 		rootKey:  t.RootKeyAbsPath,
// 		leafCert: t.LeafCertAbsPath,
// 		leafKey:  t.LeafKeyAbsPath,
// 	}
//
// 	requiredFiles := t.requiredFiles()
//
// 	for _, requiredRole := range requiredFiles {
// 		for role, path := range paths {
// 			if role == requiredRole {
// 				if _, err := os.Stat(path); err != nil {
// 					if !IsNotExist(err) {
// 						return nil, errs.Wrap(err)
// 					}
//
// 					missingFiles = append(missingFiles, fileLabels[role])
// 				}
// 			}
// 		}
// 	}
//
// 	return missingFiles, nil
// }
//
// func (t *TLSHelper) requiredFiles() []fileRole {
// 	var roles = []fileRole{}
//
// 	// rootCert is always required
// 	roles = append(roles, rootCert, leafCert, leafKey)
//
// 	if t.Create {
// 		// required for writing rootKey when create is true
// 		roles = append(roles, rootKey)
// 	}
// 	return roles
// }
//
// func (t *TLSHelper) hasRequiredFiles() (bool, error) {
// 	missingFiles, err := t.missingFiles()
// 	if err != nil {
// 		return false, err
// 	}
//
// 	return len(missingFiles) == 0, nil
// }
//
// func (t *TLSHelper) pathMap() map[*string]string {
// 	return map[*string]string{
// 		&t.RootCertAbsPath: t.RootCertRelPath,
// 		&t.RootKeyAbsPath:  t.RootKeyRelPath,
// 		&t.LeafCertAbsPath: t.LeafCertRelPath,
// 		&t.LeafKeyAbsPath:  t.LeafKeyRelPath,
// 	}
// }
//
// func (t *TLSHelper) pathRoleMap() map[*string]fileRole {
// 	return map[*string]fileRole{
// 		&t.RootCertAbsPath: rootCert,
// 		&t.RootKeyAbsPath:  rootKey,
// 		&t.LeafCertAbsPath: leafCert,
// 		&t.LeafKeyAbsPath:  leafKey,
// 	}
// }
