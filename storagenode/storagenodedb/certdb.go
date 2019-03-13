// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"bytes"
	"context"
	"crypto/x509"
	"strings"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/peertls"
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
	var pem bytes.Buffer
	err = peertls.WriteChain(&pem, append([]*x509.Certificate{pi.Leaf, pi.CA}, pi.RestChain...)...)
	if err != nil {
		return -1, ErrInfo.Wrap(err)
	}

	defer db.locked()()

	result, err := db.db.Exec(`INSERT INTO certificate(node_id, peer_identity) VALUES(?, ?)`, pi.ID, pem.Bytes())
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint") {
		err = db.db.QueryRow(`SELECT cert_id FROM certificate WHERE peer_identity = ?`, pem.Bytes()).Scan(&certid)
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

	peer, err := identity.PeerIdentityFromPEM(*pem)
	return peer, ErrInfo.Wrap(err)
}
