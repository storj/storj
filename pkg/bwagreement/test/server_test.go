// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package test

import (
	"context"
	"crypto/ecdsa"
	"flag"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	"storj.io/storj/internal/identity"
	"storj.io/storj/internal/testcontext"
	"storj.io/storj/pkg/bwagreement"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/satellite/satellitedb"
)

const (
	// postgres connstring that works with docker-compose
	defaultPostgresConn = "postgres://storj:storj-pass@test-postgres/teststorj?sslmode=disable"
)

var (
	testPostgres = flag.String("postgres-test-db", os.Getenv("STORJ_POSTGRES_TEST"), "PostgreSQL test database connection string")
)

func TestBandwidthAgreements(t *testing.T) {
	testBandwidthAgreements := func(ctx context.Context, t *testing.T, service *bwagreement.Server, satelliteKey *ecdsa.PrivateKey, uplinkKey *ecdsa.PrivateKey) {
		pba, err := GeneratePayerBandwidthAllocation(pb.PayerBandwidthAllocation_GET, satelliteKey)
		assert.NoError(t, err)

		rba, err := GenerateRenterBandwidthAllocation(pba, uplinkKey)
		assert.NoError(t, err)

		/* emulate sending the bwagreement stream from piecestore node */
		replay, err := service.BandwidthAgreements(ctx, rba)
		assert.NoError(t, err)
		assert.Equal(t, pb.AgreementsSummary_OK, replay.Status)
	}

	t.Run("Sqlite", func(t *testing.T) {
		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		// creating in-memory db and opening connection
		db, err := satellitedb.NewDB("sqlite3://file::memory:?mode=memory&cache=shared")
		if err != nil {
			t.Fatal(err)
		}
		defer ctx.Check(db.Close)

		err = db.CreateTables()
		if err != nil {
			t.Fatal(err)
		}

		satellitePubKey, satellitePrivKey, uplinkPrivKey := generateKeys(ctx, t)
		server := bwagreement.NewServer(db.BandwidthAgreement(), zap.NewNop(), satellitePubKey)

		testBandwidthAgreements(ctx, t, server, satellitePrivKey, uplinkPrivKey)
	})

	t.Run("Postgres", func(t *testing.T) {
		if *testPostgres == "" {
			t.Skipf("postgres flag missing, example:\n-postgres-test-db=%s", defaultPostgresConn)
		}

		ctx := testcontext.New(t)
		defer ctx.Cleanup()

		db, err := satellitedb.NewDB(*testPostgres)
		if err != nil {
			t.Fatal(err)
		}
		defer ctx.Check(db.Close)

		err = db.CreateTables()
		if err != nil {
			t.Fatal(err)
		}

		satellitePubKey, satellitePrivKey, uplinkPrivKey := generateKeys(ctx, t)
		server := bwagreement.NewServer(db.BandwidthAgreement(), zap.NewNop(), satellitePubKey)

		testBandwidthAgreements(ctx, t, server, satellitePrivKey, uplinkPrivKey)
	})
}

func generateKeys(ctx context.Context, t *testing.T) (satellitePubKey *ecdsa.PublicKey, satellitePrivKey *ecdsa.PrivateKey, uplinkPrivKey *ecdsa.PrivateKey) {
	fiS, err := testidentity.NewTestIdentity()
	assert.NoError(t, err)

	satellitePubKey, ok := fiS.Leaf.PublicKey.(*ecdsa.PublicKey)
	assert.True(t, ok)

	satellitePrivKey, ok = fiS.Key.(*ecdsa.PrivateKey)
	assert.True(t, ok)

	fiU, err := testidentity.NewTestIdentity()
	assert.NoError(t, err)

	uplinkPrivKey, ok = fiU.Key.(*ecdsa.PrivateKey)
	assert.True(t, ok)
	return
}
