// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package psdb

import (
	"bytes"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/gogo/protobuf/proto"
	_ "github.com/mattn/go-sqlite3"
	"storj.io/storj/pkg/pb"

	"golang.org/x/net/context"
)

var ctx = context.Background()

const concurrency = 10

func openTest(t testing.TB) (*DB, func()) {
	tmpdir, err := ioutil.TempDir("", "storj-psdb")
	if err != nil {
		t.Fatal(err)
	}
	dbpath := filepath.Join(tmpdir, "psdb.db")

	db, err := Open(ctx, "", dbpath)
	if err != nil {
		t.Fatal(err)
	}

	return db, func() {
		err := db.Close()
		if err != nil {
			t.Fatal(err)
		}

		err = os.RemoveAll(tmpdir)
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestHappyPath(t *testing.T) {
	db, cleanup := openTest(t)
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

	t.Run("Add", func(t *testing.T) {
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

	bandwidthAllocation := func(total int64) []byte {
		return serialize(t, &pb.RenterBandwidthAllocation_Data{
			PayerAllocation: &pb.PayerBandwidthAllocation{},
			Total:           total,
		})
	}

	//TODO: use better data
	allocationTests := []*pb.RenterBandwidthAllocation{
		{
			Signature: []byte("signed by test"),
			Data:      bandwidthAllocation(0),
		},
		{
			Signature: []byte("signed by sigma"),
			Data:      bandwidthAllocation(10),
		},
		{
			Signature: []byte("signed by sigma"),
			Data:      bandwidthAllocation(98),
		},
		{
			Signature: []byte("signed by test"),
			Data:      bandwidthAllocation(3),
		},
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
						if bytes.Equal(agreement, test.Data) {
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
}

func TestBandwidthUsage(t *testing.T) {
	db, cleanup := openTest(t)
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

func BenchmarkWriteBandwidthAllocation(b *testing.B) {
	db, cleanup := openTest(b)
	defer cleanup()

	const WritesPerLoop = 10

	data := serialize(b, &pb.RenterBandwidthAllocation_Data{
		PayerAllocation: &pb.PayerBandwidthAllocation{},
		Total:           156,
	})

	b.RunParallel(func(b *testing.PB) {
		for b.Next() {
			for i := 0; i < WritesPerLoop; i++ {
				_ = db.WriteBandwidthAllocToDB(&pb.RenterBandwidthAllocation{
					Signature: []byte("signed by test"),
					Data:      data,
				})
			}
		}
	})
}

func serialize(t testing.TB, v proto.Message) []byte {
	data, err := proto.Marshal(v)
	if err != nil {
		t.Fatal(err)
	}
	return data
}
