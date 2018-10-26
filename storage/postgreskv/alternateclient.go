// Copyright (C) 2018 Storj Labs, Inc.

// See LICENSE for copying information.

package postgreskv

import (
	"database/sql"

	"storj.io/storj/pkg/utils"
	"storj.io/storj/storage"
)

const (
	alternateSQLSetup = `
CREATE OR REPLACE FUNCTION local_path (fullpath BYTEA, prefix BYTEA, delimiter BYTEA)
	RETURNS BYTEA AS $$
DECLARE
	relative BYTEA;
	pos INTEGER;
BEGIN
	relative := substring(fullpath FROM (octet_length(prefix)+1));
	pos := position(delimiter IN relative);
	IF pos = 0 THEN
		RETURN relative;
	END IF;
	RETURN substring(relative FOR pos);
END;
$$ LANGUAGE 'plpgsql'
	IMMUTABLE STRICT;
`

	alternateSQLTeardown = `
DROP FUNCTION local_path(BYTEA, BYTEA, BYTEA);
`

	alternateForwardQuery = `
SELECT DISTINCT
	$2::BYTEA || x.localpath AS p,
	first_value(x.metadata) OVER (PARTITION BY x.localpath ORDER BY x.fullpath) AS m
FROM (
	SELECT
		pd.fullpath,
		local_path(pd.fullpath, $2::BYTEA, set_byte(' '::BYTEA, 0, b.delim)) AS localpath,
		pd.metadata
	FROM
		pathdata pd,
		buckets b
	WHERE
		b.bucketname = $1::BYTEA
		AND pd.bucket = b.bucketname
		AND pd.fullpath >= $2::BYTEA
		AND ($2::BYTEA = ''::BYTEA OR pd.fullpath < bytea_increment($2::BYTEA))
		AND pd.fullpath >= $3::BYTEA
) x
ORDER BY p
LIMIT $4
`

	alternateReverseQuery = `
SELECT DISTINCT
	$2::BYTEA || x.localpath AS p,
	first_value(x.metadata) OVER (PARTITION BY x.localpath ORDER BY x.fullpath) AS m
FROM (
	SELECT
		pd.fullpath,
		local_path(pd.fullpath, $2::BYTEA, set_byte(' '::BYTEA, 0, b.delim)) AS localpath,
		pd.metadata
	FROM
		pathdata pd,
		buckets b
	WHERE
		b.bucketname = $1::BYTEA
		AND pd.bucket = b.bucketname
		AND pd.fullpath >= $2::BYTEA
		AND ($2::BYTEA = ''::BYTEA OR pd.fullpath < bytea_increment($2::BYTEA))
		AND ($3::BYTEA = ''::BYTEA OR pd.fullpath <= $3::BYTEA)
) x
ORDER BY p DESC
LIMIT $4
`
)

// AlternateClient is the entrypoint into an alternate postgreskv data store
type AlternateClient struct {
	*Client
}

// AltNew instantiates a new postgreskv AlternateClient given db URL
func AltNew(dbURL string) (*AlternateClient, error) {
	client, err := New(dbURL)
	if err != nil {
		return nil, err
	}
	_, err = client.pgConn.Exec(alternateSQLSetup)
	if err != nil {
		return nil, utils.CombineErrors(err, client.Close())
	}
	return &AlternateClient{Client: client}, nil
}

// Close closes an AlternateClient and frees its resources.
func (altClient *AlternateClient) Close() error {
	_, err := altClient.pgConn.Exec(alternateSQLTeardown)
	return utils.CombineErrors(err, altClient.Client.Close())
}

type alternateOrderedPostgresIterator struct {
	*orderedPostgresIterator
}

func (opi *alternateOrderedPostgresIterator) doNextQuery() (*sql.Rows, error) {
	if opi.opts.Recurse {
		return opi.orderedPostgresIterator.doNextQuery()
	}
	start := opi.lastKeySeen
	if start == nil {
		start = opi.opts.First
	}
	var query string
	if opi.opts.Reverse {
		query = alternateReverseQuery
	} else {
		query = alternateForwardQuery
	}
	return opi.client.pgConn.Query(query, []byte(opi.bucket), []byte(opi.opts.Prefix), []byte(start), opi.batchSize+1)
}

func newAlternateOrderedPostgresIterator(altClient *AlternateClient, opts storage.IterateOptions, batchSize int) (*alternateOrderedPostgresIterator, error) {
	if opts.Prefix == nil {
		opts.Prefix = storage.Key("")
	}
	if opts.First == nil {
		opts.First = storage.Key("")
	}
	opi1 := &orderedPostgresIterator{
		client:    altClient.Client,
		opts:      &opts,
		bucket:    storage.Key(defaultBucket),
		delimiter: byte('/'),
		batchSize: batchSize,
		curIndex:  0,
	}
	opi := &alternateOrderedPostgresIterator{orderedPostgresIterator: opi1}
	opi.nextQuery = opi.doNextQuery
	newRows, err := opi.nextQuery()
	if err != nil {
		return nil, err
	}
	opi.curRows = newRows
	return opi, nil
}

// Iterate iterates over items based on opts
func (altClient *AlternateClient) Iterate(opts storage.IterateOptions, fn func(storage.Iterator) error) (err error) {
	opi, err := newAlternateOrderedPostgresIterator(altClient, opts, defaultBatchSize)
	if err != nil {
		return err
	}
	defer func() {
		err = utils.CombineErrors(err, opi.Close())
	}()

	return fn(opi)
}
