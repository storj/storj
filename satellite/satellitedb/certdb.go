// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package satellitedb

import (
	"context"
	"crypto/x509"
	"database/sql"
	"encoding/asn1"
	"time"

	"github.com/zeebo/errs"

	"storj.io/storj/pkg/identity"
	"storj.io/storj/pkg/pkcrypto"
	"storj.io/storj/pkg/storj"
	dbx "storj.io/storj/satellite/satellitedb/dbx"
)

type certDB struct {
	db *dbx.DB
}

func (certs *certDB) Set(ctx context.Context, nodeID storj.NodeID, pi *identity.PeerIdentity) (err error) {
	defer mon.Task()(&ctx)(&err)

	tx, err := certs.db.Begin()
	if err != nil {
		return Error.Wrap(err)
	}

	defer func() {
		if err == nil {
			err = tx.Commit()
		} else {
			err = errs.Combine(err, tx.Rollback())
		}
	}()

	if pi == nil {
		return Error.New("Peer Identity cannot be nil")
	}
	chain := encodePeerIdentity(pi)

	var id int64
	query := `SELECT peer_identity FROM certRecords WHERE serial_num = ?;`
	err = tx.QueryRow(certs.db.Rebind(query), pi.Leaf.SerialNumber.Bytes()).Scan(&id)
	if err != nil {
		if err == sql.ErrNoRows {
			// create a new entry
			_, err = tx.Exec(certs.db.Rebind(`INSERT INTO certRecords ( peer_identity, node_id, update_at ) VALUES ( ?, ?, ? );`), chain, nodeID.Bytes(), time.Now())
			if err != nil {
				return Error.Wrap(err)
			}
			return nil
		}
		return Error.Wrap(err)
	}

	// already public key exists, just return
	return nil
}

// Get gets the public key of uplink corresponding to uplink id
func (certs *certDB) Get(ctx context.Context, nodeID storj.NodeID) (_ *identity.PeerIdentity, err error) {
	defer mon.Task()(&ctx)(&err)
	dbxInfo, err := certs.db.All_CertRecord_By_NodeId_OrderBy_Desc_UpdateAt(ctx, dbx.CertRecord_NodeId(nodeID.Bytes()))
	if err != nil {
		return nil, err
	}

	if len(dbxInfo) == 0 {
		return nil, Error.New("Invalid nodeID : %+v: %+v ", nodeID.String(), err)
	}

	// the first indext always holds the lastest of the keys

	peer, err := decodePeerIdentity(ctx, dbxInfo[0].PeerIdentity)
	return peer, Error.Wrap(err)
}

func encodePeerIdentity(pi *identity.PeerIdentity) []byte {
	var chain []byte
	chain = append(chain, pi.Leaf.Raw...)
	chain = append(chain, pi.CA.Raw...)
	for _, cert := range pi.RestChain {
		chain = append(chain, cert.Raw...)
	}
	return chain
}

func decodePeerIdentity(ctx context.Context, chain []byte) (_ *identity.PeerIdentity, err error) {
	defer mon.Task()(&ctx)(&err)

	var certs []*x509.Certificate
	for len(chain) > 0 {
		var raw asn1.RawValue
		var err error

		chain, err = asn1.Unmarshal(chain, &raw)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		cert, err := pkcrypto.CertFromDER(raw.FullBytes)
		if err != nil {
			return nil, Error.Wrap(err)
		}

		certs = append(certs, cert)
	}
	if len(certs) < 2 {
		return nil, Error.New("not enough certificates")
	}
	return identity.PeerIdentityFromChain(certs)
}
