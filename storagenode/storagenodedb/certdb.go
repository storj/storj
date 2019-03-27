// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"crypto/x509"
	"encoding/asn1"
	"strings"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/storagenode/trust"
)

type certdb struct {
	*infodb
}

// CertDB returns certificate database.
func (db *DB) CertDB() trust.CertDB { return db.info.CertDB() }

// CertDB returns certificate database.
func (db *infodb) CertDB() trust.CertDB { return &certdb{db} }

// Include includes the certificate in the table and returns an unique id.
func (db *certdb) Include(ctx context.Context, pi *identity.PeerIdentity) (certid int64, err error) {
	chain := encodePeerIdentity(pi)

	defer db.locked()()

	result, err := db.db.Exec(`INSERT INTO certificate(node_id, peer_identity) VALUES(?, ?)`, pi.ID, chain)
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint") {
		err = db.db.QueryRow(`SELECT cert_id FROM certificate WHERE peer_identity = ?`, chain).Scan(&certid)
		return certid, ErrInfo.Wrap(err)
	} else if err != nil {
		return -1, ErrInfo.Wrap(err)
	}

	certid, err = result.LastInsertId()
	return certid, ErrInfo.Wrap(err)
}

// LookupByCertID finds certificate by the certid returned by Include.
func (db *certdb) LookupByCertID(ctx context.Context, id int64) (*identity.PeerIdentity, error) {
	var pem *[]byte

	db.mu.Lock()
	err := db.db.QueryRow(`SELECT peer_identity FROM certificate WHERE cert_id = ?`, id).Scan(&pem)
	db.mu.Unlock()

	if err != nil {
		return nil, ErrInfo.Wrap(err)
	}
	if pem == nil {
		return nil, ErrInfo.New("did not find certificate")
	}

	peer, err := decodePeerIdentity(*pem)
	return peer, ErrInfo.Wrap(err)
}

// TODO: move into pkcrypto
func encodePeerIdentity(pi *identity.PeerIdentity) []byte {
	var chain []byte
	chain = append(chain, pi.Leaf.Raw...)
	chain = append(chain, pi.CA.Raw...)
	for _, cert := range pi.RestChain {
		chain = append(chain, cert.Raw...)
	}
	return chain
}

func decodePeerIdentity(chain []byte) (*identity.PeerIdentity, error) {
	var certs []*x509.Certificate
	for len(chain) > 0 {
		var raw asn1.RawValue
		var err error

		chain, err = asn1.Unmarshal(chain, &raw)
		if err != nil {
			return nil, ErrInfo.Wrap(err)
		}

		cert, err := pkcrypto.CertFromDER(raw.FullBytes)
		if err != nil {
			return nil, ErrInfo.Wrap(err)
		}

		certs = append(certs, cert)
	}
	if len(certs) < 2 {
		return nil, ErrInfo.New("not enough certificates")
	}
	return identity.PeerIdentityFromCerts(certs[0], certs[1], certs[2:])
}
