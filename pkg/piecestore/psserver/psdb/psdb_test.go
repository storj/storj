// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package psdb_test

import (
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"github.com/stretchr/testify/require"
	"go.uber.org/zap"

	"storj.io/storj/internal/testcontext"
	"storj.io/storj/internal/teststorj"
	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/piecestore/psserver/psdb"
	"storj.io/storj/pkg/storj"
)

const concurrency = 10

func newDB(t testing.TB, id string) (*psdb.DB, func()) {
	tmpdir, err := ioutil.TempDir("", "storj-psdb-"+id)
	require.NoError(t, err)

	dbpath := filepath.Join(tmpdir, "psdb.db")

	db, err := psdb.Open(dbpath)
	require.NoError(t, err)

	err = psdb.Migration().Run(zap.NewNop(), db)
	require.NoError(t, err)

	return db, func() {
		err := db.Close()
		require.NoError(t, err)
		err = os.RemoveAll(tmpdir)
		require.NoError(t, err)
	}
}

func TestNewInmemory(t *testing.T) {
	db, err := psdb.OpenInMemory()
	if err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
}

func TestHappyPath(t *testing.T) {
	db, cleanup := newDB(t, "1")
	defer cleanup()

	type TTL struct {
		ID         string
		Expiration int64
	}

	tests := []TTL{
		{ID: "", Expiration: 0},
		{ID: "\x00", Expiration: ^int64(0)},
		{ID: "test", Expiration: 666},
	}

	t.Run("Create", func(t *testing.T) {
		for P := 0; P < concurrency; P++ {
			t.Run("#"+strconv.Itoa(P), func(t *testing.T) {
				t.Parallel()
				for _, ttl := range tests {
					err := db.AddTTL(ttl.ID, ttl.Expiration, 0)
					if err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	})

	t.Run("Get", func(t *testing.T) {
		for P := 0; P < concurrency; P++ {
			t.Run("#"+strconv.Itoa(P), func(t *testing.T) {
				t.Parallel()
				for _, ttl := range tests {
					expiration, err := db.GetTTLByID(ttl.ID)
					if err != nil {
						t.Fatal(err)
					}

					if ttl.Expiration != expiration {
						t.Fatalf("expected %d got %d", ttl.Expiration, expiration)
					}
				}
			})
		}
	})

	t.Run("Delete", func(t *testing.T) {
		for P := 0; P < concurrency; P++ {
			t.Run("Delete", func(t *testing.T) {
				t.Parallel()
				for _, ttl := range tests {
					err := db.DeleteTTLByID(ttl.ID)
					if err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	})

	t.Run("Get Deleted", func(t *testing.T) {
		for P := 0; P < concurrency; P++ {
			t.Run("#"+strconv.Itoa(P), func(t *testing.T) {
				t.Parallel()
				for _, ttl := range tests {
					expiration, err := db.GetTTLByID(ttl.ID)
					if err == nil {
						t.Fatal(err)
					}
					if expiration != 0 {
						t.Fatalf("expected expiration 0 got %d", expiration)
					}
				}
			})
		}
	})

	bandwidthAllocation := func(signature string, satelliteID storj.NodeID, total int64) *pb.RenterBandwidthAllocation {
		return &pb.RenterBandwidthAllocation{
			PayerAllocation: pb.PayerBandwidthAllocation{SatelliteId: satelliteID},
			Total:           total,
			Signature:       []byte(signature),
		}
	}

	//TODO: use better data
	nodeIDAB := teststorj.NodeIDFromString("AB")
	allocationTests := []*pb.RenterBandwidthAllocation{
		bandwidthAllocation("signed by test", nodeIDAB, 0),
		bandwidthAllocation("signed by sigma", nodeIDAB, 10),
		bandwidthAllocation("signed by sigma", nodeIDAB, 98),
		bandwidthAllocation("signed by test", nodeIDAB, 3),
	}

	t.Run("Bandwidth Allocation", func(t *testing.T) {
		for P := 0; P < concurrency; P++ {
			t.Run("#"+strconv.Itoa(P), func(t *testing.T) {
				t.Parallel()
				for _, test := range allocationTests {
					err := db.WriteBandwidthAllocToDB(test)
					if err != nil {
						t.Fatal(err)
					}

					agreements, err := db.GetBandwidthAllocationBySignature(test.Signature)
					if err != nil {
						t.Fatal(err)
					}

					found := false
					for _, agreement := range agreements {
						if pb.Equal(agreement, test) {
							found = true
							break
						}
					}

					if !found {
						t.Fatal("did not find added bandwidth allocation")
					}
				}
			})
		}
	})

	t.Run("Get all Bandwidth Allocations", func(t *testing.T) {
		for P := 0; P < concurrency; P++ {
			t.Run("#"+strconv.Itoa(P), func(t *testing.T) {
				t.Parallel()

				agreementGroups, err := db.GetBandwidthAllocations()
				if err != nil {
					t.Fatal(err)
				}

				found := false
				for _, agreements := range agreementGroups {
					for _, agreement := range agreements {
						for _, test := range allocationTests {
							if pb.Equal(&agreement.Agreement, test) {
								found = true
								break
							}
						}
					}
				}

				if !found {
					t.Fatal("did not find added bandwidth allocation")
				}
			})
		}
	})
}

func TestBandwidthUsage(t *testing.T) {
	db, cleanup := newDB(t, "2")
	defer cleanup()

	type BWUSAGE struct {
		size    int64
		timenow time.Time
	}

	bwtests := []BWUSAGE{
		{size: 1000, timenow: time.Now()},
	}

	var bwTotal int64
	t.Run("AddBandwidthUsed", func(t *testing.T) {
		for P := 0; P < concurrency; P++ {
			bwTotal = bwTotal + bwtests[0].size
			t.Run("#"+strconv.Itoa(P), func(t *testing.T) {
				t.Parallel()
				for _, bw := range bwtests {
					err := db.AddBandwidthUsed(bw.size)
					if err != nil {
						t.Fatal(err)
					}
				}
			})
		}
	})

	t.Run("GetTotalBandwidthBetween", func(t *testing.T) {
		for P := 0; P < concurrency; P++ {
			t.Run("#"+strconv.Itoa(P), func(t *testing.T) {
				t.Parallel()
				for _, bw := range bwtests {
					size, err := db.GetTotalBandwidthBetween(bw.timenow, bw.timenow)
					if err != nil {
						t.Fatal(err)
					}
					if bwTotal != size {
						t.Fatalf("expected %d got %d", bw.size, size)
					}
				}
			})
		}
	})

	t.Run("GetBandwidthUsedByDay", func(t *testing.T) {
		for P := 0; P < concurrency; P++ {
			t.Run("#"+strconv.Itoa(P), func(t *testing.T) {
				t.Parallel()
				for _, bw := range bwtests {
					size, err := db.GetBandwidthUsedByDay(bw.timenow)
					if err != nil {
						t.Fatal(err)
					}
					if bwTotal != size {
						t.Fatalf("expected %d got %d", bw.size, size)
					}
				}
			})
		}
	})
}

func TestMigration(t *testing.T) {
	_, err := exec.LookPath("sqlite3")
	if err != nil {
		t.Skip("unable to find sqlite3 executable")
	}

	// find latest version
	migration := psdb.Migration()
	migration.Now = "now"

	latestVersion := 0
	// TODO support missing intermediate versions
	for _, step := range migration.Steps {
		if latestVersion < step.Version {
			latestVersion = step.Version
		}
	}

	for version := 0; version <= latestVersion; version++ {
		t.Run("migration-v"+strconv.Itoa(version), func(t *testing.T) {
			ctx := testcontext.New(t)
			defer ctx.Cleanup()

			dbpath := ctx.File("psdb.db")
			db, err := psdb.Open(dbpath)
			require.NoError(t, err)
			defer ctx.Check(db.Close)

			testedMigration := migration.TargetVersion(version)
			err = testedMigration.Run(zap.NewNop(), db)
			require.NoError(t, err)

			out, err := exec.Command("sqlite3", dbpath, ".dump").Output()
			require.NoError(t, err)

			testfile := "testdata/db.v" + strconv.Itoa(version) + ".sql"
			if _, err := os.Stat(testfile); err != nil {
				t.Fatal("no test data ", testfile)
			}
			expected, err := ioutil.ReadFile(testfile)

			assert.Equal(t, string(expected), string(out))
		})
	}
}

func BenchmarkWriteBandwidthAllocation(b *testing.B) {
	db, cleanup := newDB(b, "3")
	defer cleanup()
	const WritesPerLoop = 10
	b.RunParallel(func(b *testing.PB) {
		for b.Next() {
			for i := 0; i < WritesPerLoop; i++ {
				_ = db.WriteBandwidthAllocToDB(&pb.RenterBandwidthAllocation{
					PayerAllocation: pb.PayerBandwidthAllocation{},
					Total:           156,
					Signature:       []byte("signed by test"),
				})
			}
		}
	})
}
