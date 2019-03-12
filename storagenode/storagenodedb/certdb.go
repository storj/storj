// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package storagenodedb

import (
	"context"
	"strings"

	"storj.io/storj/pkg/storj"
)

type certdb struct {
	*infodb
}

// CertDB returns certificate database.
func (db *infodb) CertDB() certdb { return certdb{db} }

// Include includes the certificate in the table and returns an unique id.
func (db *certdb) Include(ctx context.Context, nodeid storj.NodeID, pkix []byte) (certid int64, err error) {
	defer db.locked()()

	result, err := db.db.Exec(`INSERT INTO certificate(node_id, pkix) VALUES(?, ?)`, nodeid.Bytes(), pkix)
	if err != nil && strings.Contains(err.Error(), "UNIQUE constraint") {
		err = db.db.QueryRow(`SELECT cert_id FROM certificate WHERE pkix = ?`, pkix).Scan(&certid)
		return certid, ErrInfo.Wrap(err)
	} else if err != nil {
		return -1, ErrInfo.Wrap(err)
	}

	certid, err = result.LastInsertId()
	return certid, ErrInfo.Wrap(err)
}

// LookupByCertID finds certificate by the certid returned by Include.
func (db *certdb) LookupByCertID(ctx context.Context, id int64) (pkix []byte, err error) {
	defer db.locked()()

	var ppkix *[]byte
	err = db.db.QueryRow(`SELECT pkix FROM certificate WHERE cert_id = ?`, id).Scan(&ppkix)
	if err != nil {
		return nil, ErrInfo.Wrap(err)
	}
	if ppkix == nil {
		return nil, ErrInfo.New("did not find certificate")
	}
	return *ppkix, nil
}
