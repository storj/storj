// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package spannerutil

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/zeebo/errs"

	"storj.io/common/context2"
)

// EphemeralDB manages lifecycle of a temporary database.
type EphemeralDB struct {
	Params ConnParams
	admin  *EmulatorAdmin

	ephemeralInstance bool
	ephemeralDatabase bool
}

// CreateEphemeralDB automatically creates instance and database as necessary.
func CreateEphemeralDB(ctx context.Context, connstr string, databasePrefix string, ddls ...string) (*EphemeralDB, error) {
	params, err := ParseConnStr(connstr)
	if err != nil {
		return nil, errs.New("failed to parse connection string %q: %w", connstr, err)
	}

	if !params.Emulator && params.Project == "" {
		return nil, errs.New("when not using an emulator project is required")
	}

	if params.Project == "" {
		params.Project = randomIdentifier("temp", 8)
	}

	var ephemeralInstance bool
	if params.Instance == "" {
		params.Instance = randomIdentifier("temp", 8)
		ephemeralInstance = true
	}

	var ephemeralDatabase bool
	if params.Database == "" || params.Emulator {
		params.Database = randomIdentifier("temp-"+databasePrefix, 8)
		ephemeralDatabase = true
	}

	if !params.Emulator {
		if !strings.Contains(params.Database, "test") && !strings.Contains(params.Database, "temp") {
			return nil, errs.New("the database name must contain test or temp to run on Spanner")
		}
	}

	admin := OpenEmulatorAdmin(params)

	if ephemeralInstance {
		err := admin.CreateInstance(ctx, params)
		if err != nil {
			return nil, errors.Join(fmt.Errorf("failed to create instance: %w", err), admin.Close())
		}
	}
	if ephemeralDatabase {
		err := admin.CreateDatabase(ctx, params, ddls...)
		if err != nil {
			var errDeleteInstance error
			if ephemeralInstance {
				errDeleteInstance = admin.DeleteInstance(ctx, params)
			}
			return nil, errors.Join(fmt.Errorf("failed to create instance: %w", err), errDeleteInstance, admin.Close())
		}
	}

	return &EphemeralDB{
		Params: params,
		admin:  admin,

		ephemeralInstance: ephemeralInstance,
		ephemeralDatabase: ephemeralDatabase,
	}, nil
}

// Close deletes the created the instance and database.
func (db *EphemeralDB) Close(ctx context.Context) error {
	ctx, cancel := context2.WithRetimeout(ctx, time.Minute)
	defer cancel()

	var errdrop error
	switch {
	case db.ephemeralInstance:
		// dropping instance should get rid of any associated databases as well
		errdrop = db.admin.DeleteInstance(ctx, db.Params)
		if errdrop != nil {
			errdrop = Error.New("deleting instance failed: %w", errdrop)
		}
	case db.ephemeralDatabase:
		errdrop = db.admin.DropDatabase(ctx, db.Params)
		if errdrop != nil {
			errdrop = Error.New("dropping database failed: %w", errdrop)
		}
	}

	return errors.Join(db.admin.Close(), errdrop)
}

const maxConnectionStringTokenLength = 30

func randomIdentifier(prefix string, randomBytes int) string {
	if prefix != "" {
		prefix = strings.ToLower(prefix)
		prefix = strings.TrimPrefix(prefix, "test")
		prefix = strings.TrimPrefix(prefix, "benchmark")
		prefix = strings.ReplaceAll(prefix, `\`, "_")
		prefix = strings.ReplaceAll(prefix, `/`, "_")

		if len(prefix)+randomBytes > maxConnectionStringTokenLength {
			prefix = prefix[:maxConnectionStringTokenLength-randomBytes-1]
		}
		prefix += "-"
	}

	const alphabet = "abcdefghijklmnopqrstuvwxyz"
	n := len(prefix) + randomBytes
	b := make([]byte, n)
	offset := copy(b, prefix)

	rn, err := rand.Read(b[offset:])
	if err != nil || rn != n-offset {
		panic("reading random failed: ")
	}

	for i, v := range b[offset:] {
		b[offset+i] = alphabet[int(v)%len(alphabet)]
	}

	return string(b)
}
