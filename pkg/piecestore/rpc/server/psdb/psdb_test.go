// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package psdb

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"testing"

	"github.com/gogo/protobuf/proto"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	pb "storj.io/storj/protos/piecestore"

	"golang.org/x/net/context"
)

var ctx = context.Background()
var concurrency = 10

func TestOpenPSDB(t *testing.T) {
	tests := []struct {
		it  string
		err string
	}{
		{
			it:  "should successfully create database",
			err: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.it, func(t *testing.T) {
			assert := assert.New(t)

			tmp, err := ioutil.TempDir("", "example")
			if err != nil {
				log.Fatal(err)
			}
			defer os.RemoveAll(tmp)

			dbpath := filepath.Join(tmp, "test.db")

			DB, err := OpenPSDB(ctx, "", dbpath)
			if tt.err != "" {
				assert.NotNil(err)
				assert.Equal(tt.err, err.Error())
				return
			}
			assert.NoError(err)
			assert.NotNil(DB)
			assert.NotNil(DB.DB)
		})
	}
}

func TestAddTTLToDB(t *testing.T) {
	tests := []struct {
		it         string
		id         string
		expiration int64
		err        string
	}{
		{
			it:         "should successfully Put TTL",
			id:         "Butts",
			expiration: 666,
			err:        "",
		},
	}

	tmp, err := ioutil.TempDir("", "example")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	dbpath := filepath.Join(tmp, "test.db")
	db, err := OpenPSDB(ctx, "", dbpath)
	if err != nil {
		t.Errorf("Failed to create database")
		return
	}
	defer os.Remove(dbpath)

	for _, tt := range tests {
		for i := 0; i < concurrency; i++ {
			t.Run(tt.it, func(t *testing.T) {
				assert := assert.New(t)

				err := db.AddTTLToDB(tt.id, tt.expiration)
				if tt.err != "" {
					assert.NotNil(err)
					assert.Equal(tt.err, err.Error())
					return
				}
				assert.NoError(err)

				db.mtx.Lock()
				rows, err := db.DB.Query(fmt.Sprintf(`SELECT * FROM ttl WHERE id="%s"`, tt.id))
				assert.NoError(err)

				rows.Next()
				var expiration int64
				var id string
				var time int64
				err = rows.Scan(&id, &time, &expiration)
				assert.NoError(err)
				rows.Close()

				db.mtx.Unlock()

				assert.Equal(tt.id, id)
				assert.True(time > 0)
				assert.Equal(tt.expiration, expiration)
			})
		}
	}
}

// This test depends on AddTTLToDB to pass
func TestDeleteTTLByID(t *testing.T) {
	tests := []struct {
		it  string
		id  string
		err string
	}{
		{
			it:  "should successfully Delete TTL by ID",
			id:  "butts",
			err: "",
		},
	}

	tmp, err := ioutil.TempDir("", "example")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	dbpath := filepath.Join(tmp, "test.db")
	db, err := OpenPSDB(ctx, "", dbpath)
	if err != nil {
		t.Errorf("Failed to create database")
		return
	}
	defer os.Remove(dbpath)

	for _, tt := range tests {
		for i := 0; i < concurrency; i++ {
			t.Run(tt.it, func(t *testing.T) {
				assert := assert.New(t)
				err := db.AddTTLToDB(tt.id, 0)
				assert.NoError(err)

				err = db.DeleteTTLByID(tt.id)
				if tt.err != "" {
					assert.NotNil(err)
					assert.Equal(tt.err, err.Error())
					return
				}
				assert.NoError(err)

			})
		}
	}
}

// This test depends on AddTTLToDB to pass
func TestGetTTLByID(t *testing.T) {
	tests := []struct {
		it         string
		id         string
		expiration int64
		err        string
	}{
		{
			it:         "should successfully Get TTL by ID",
			id:         "butts",
			expiration: 666,
			err:        "",
		},
	}

	tmp, err := ioutil.TempDir("", "example")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	dbpath := filepath.Join(tmp, "test.db")
	db, err := OpenPSDB(ctx, "", dbpath)
	if err != nil {
		t.Errorf("Failed to create database")
		return
	}
	defer os.Remove(dbpath)

	for _, tt := range tests {
		for i := 0; i < concurrency; i++ {
			t.Run(tt.it, func(t *testing.T) {
				assert := assert.New(t)
				err := db.AddTTLToDB(tt.id, tt.expiration)
				assert.NoError(err)

				expiration, err := db.GetTTLByID(tt.id)
				if tt.err != "" {
					assert.NotNil(err)
					assert.Equal(tt.err, err.Error())
					return
				}
				assert.NoError(err)
				assert.Equal(tt.expiration, expiration)
			})
		}
	}

	t.Run("should return 0 if ttl doesn't exist", func(t *testing.T) {
		assert := assert.New(t)
		expiration, err := db.GetTTLByID("fake-id")
		assert.NotNil(err)
		assert.Equal(int64(0), expiration)
	})

}

func TestWriteBandwidthAllocToDB(t *testing.T) {
	tests := []struct {
		it              string
		payerAllocation *pb.PayerBandwidthAllocation
		total           int64
		err             string
	}{
		{
			it:              "should successfully Put Bandwidth Allocation",
			payerAllocation: &pb.PayerBandwidthAllocation{},
			total:           5,
			err:             "",
		},
	}

	tmp, err := ioutil.TempDir("", "example")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	dbpath := filepath.Join(tmp, "test.db")
	db, err := OpenPSDB(ctx, "", dbpath)
	if err != nil {
		t.Errorf("Failed to create database")
		return
	}
	defer os.Remove(dbpath)

	for _, tt := range tests {
		for i := 0; i < concurrency; i++ {
			t.Run(tt.it, func(t *testing.T) {
				assert := assert.New(t)
				ba := &pb.RenterBandwidthAllocation{
					Signature: []byte{'A', 'B'},
					Data: serializeData(&pb.RenterBandwidthAllocation_Data{
						PayerAllocation: tt.payerAllocation,
						Total:           tt.total,
					}),
				}
				err = db.WriteBandwidthAllocToDB(ba)
				if tt.err != "" {
					assert.NotNil(err)
					assert.Equal(tt.err, err.Error())
					return
				}
				assert.NoError(err)
				// check db to make sure agreement and signature were stored correctly
				db.mtx.Lock()
				rows, err := db.DB.Query(`SELECT * FROM bandwidth_agreements Limit 1`)
				assert.NoError(err)

				for rows.Next() {
					var (
						agreement []byte
						signature []byte
					)

					err = rows.Scan(&agreement, &signature)
					assert.NoError(err)

					decodedRow := &pb.RenterBandwidthAllocation_Data{}
					err = proto.Unmarshal(agreement, decodedRow)
					assert.NoError(err)

					assert.Equal(ba.GetSignature(), signature)
					assert.Equal(tt.payerAllocation, decodedRow.GetPayerAllocation())
					assert.Equal(tt.total, decodedRow.GetTotal())

				}
				rows.Close()
				db.mtx.Unlock()
				err = rows.Err()
				assert.NoError(err)
			})
		}
	}
}

func serializeData(ba *pb.RenterBandwidthAllocation_Data) []byte {
	data, _ := proto.Marshal(ba)

	return data
}
