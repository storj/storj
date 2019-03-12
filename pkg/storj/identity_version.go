// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storj

import (
	"crypto"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"strconv"
	"strings"

	"storj.io/storj/pkg/peertls"
	"storj.io/storj/pkg/pkcrypto"
)

const (
	V1 = IDVersionNumber(iota + 1)
	V2
)

var (
	IDVersions = map[IDVersionNumber]IDVersion{
		/* V1 breaking change:
		+ removed support for difficulties < 9
		*/
		V1: {
			Number:        V1,
			NewPrivateKey: pkcrypto.GeneratePrivateKey,
		},
		/* V2 changes:
		+ add version support
		+ change elliptic curve to non-NIST
		*/
		V2: {
			Number:        V2,
			NewPrivateKey: pkcrypto.GeneratePrivateKey,
			// TODO@thepaul: update this
		},
	}

	IDVersionExtensionOID = peertls.IdentityVersionExtID
)

type IDVersionNumber uint8

type IDVersion struct {
	Number        IDVersionNumber
	NewPrivateKey func() (crypto.PrivateKey, error)
}

type IDVersionExtensionHandler struct {
	oid *asn1.ObjectIdentifier
}

func init() {
	peertls.AvailableExtensionHandlers.Register(NewIDVersionExtensionHandler())
}

func NewIDVersionExtensionHandler() peertls.ExtensionHandler {
	return &IDVersionExtensionHandler{
		oid: &IDVersionExtensionOID,
	}
}

func GetIDVersion(number IDVersionNumber) (IDVersion, error) {
	if number == 0 {
		return LatestIDVersion(), nil
	}

	version, ok := IDVersions[number]
	if !ok {
		return IDVersion{}, ErrVersion.New("unknown version")
	}

	return version, nil
}

func LatestIDVersion() IDVersion {
	return IDVersions[IDVersionNumber(len(IDVersions))]
}

func IDVersionFromCert(cert *x509.Certificate) (IDVersion, error) {
	// TODO: switch to cert.Extensions; ExtraExtensions is not populated when parsing!
	for _, ext := range cert.ExtraExtensions {
		if ext.Id.Equal(peertls.IdentityVersionExtID) {
			return GetIDVersion(IDVersionNumber(ext.Value[0]))
		}
	}
	return IDVersion{}, ErrVersion.New("unknown version")
}

func IDVersionInVersions(versionNumber IDVersionNumber, versionsStr string) error {
	switch versionsStr {
	case "latest":
		if versionNumber == LatestIDVersion().Number {
			return nil
		}
	default:
		versionRanges := strings.Split(versionsStr, ",")
		for _, versionRange := range versionRanges {
			if strings.Contains(versionRange, "-") {
				versionLimits := strings.Split(versionRange, "-")
				if len(versionLimits) != 2 {
					return ErrVersion.New("malformed PeerIDVersions string: %s", versionsStr)
				}

				begin, err := strconv.Atoi(versionLimits[0])
				if err != nil {
					return ErrVersion.Wrap(err)
				}

				end, err := strconv.Atoi(versionLimits[1])
				if err != nil {
					return ErrVersion.Wrap(err)
				}

				for i := begin; i <= end; i ++ {
					if versionNumber == IDVersionNumber(i) {
						return nil
					}
				}
			} else {
				versionInt, err := strconv.Atoi(versionRange)
				if err != nil {
					return ErrVersion.Wrap(err)
				}
				if versionNumber == IDVersionNumber(versionInt) {
					return nil
				}
			}
		}
	}
	return ErrVersion.New("version %d not in versions %s", versionNumber, versionsStr)
}

// TODO: should this include signature?
func AddVersionExt(version IDVersionNumber, cert *x509.Certificate) error {
	return peertls.AddExtension(cert, pkix.Extension{
		Id:    peertls.IdentityVersionExtID,
		Value: []byte{byte(version)},
	})
}

func (idVersionExtHandler *IDVersionExtensionHandler) OID() *asn1.ObjectIdentifier {
	return idVersionExtHandler.oid
}

func (idVersionExtHandler *IDVersionExtensionHandler) NewVerifier(opts peertls.ExtensionOptions) peertls.ExtensionVerificationFunc {
	return func(ext pkix.Extension, chain [][]*x509.Certificate) error {
		return IDVersionInVersions(IDVersionNumber(ext.Value[0]), opts.PeerIDVersions)
	}
}
