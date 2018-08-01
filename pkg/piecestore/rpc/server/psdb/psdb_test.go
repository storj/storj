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
	"time"

	"github.com/gogo/protobuf/proto"
	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"
	pb "storj.io/storj/protos/piecestore"

	"golang.org/x/net/context"
)

var ctx = context.Background()
var parallelCount = 100

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

			DB, err := OpenPSDB(dbpath)
			if tt.err != "" {
				assert.NotNil(err)
				assert.Equal(tt.err, err.Error())
				return
			}
			assert.Nil(err)
			assert.NotNil(DB)
			assert.NotNil(DB.DB)
		})
	}
}

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
	db, err := OpenPSDB(dbpath)
	if err != nil {
		t.Errorf("Failed to create database")
		return
	}
	defer os.Remove(dbpath)

	for _, tt := range tests {
		for i := 0; i < parallelCount; i++ {
			t.Run(tt.it, func(t *testing.T) {
				t.Parallel()
				assert := assert.New(t)

				db.mtx.Lock()
				db.DB.Exec(fmt.Sprintf(`INSERT or REPLACE INTO ttl (id, created, expires) VALUES ("%s", "%d", "%d")`, tt.id, time.Now().Unix(), 0))
				db.mtx.Unlock()

				err = db.DeleteTTLByID(tt.id)
				if tt.err != "" {
					assert.NotNil(err)
					assert.Equal(tt.err, err.Error())
					return
				}
				assert.Nil(err)

			})
		}
	}
}

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
	db, err := OpenPSDB(dbpath)
	if err != nil {
		t.Errorf("Failed to create database")
		return
	}
	defer os.Remove(dbpath)

	for _, tt := range tests {
		for i := 0; i < parallelCount; i++ {
			t.Run(tt.it, func(t *testing.T) {
				t.Parallel()
				assert := assert.New(t)
				db.mtx.Lock()
				db.DB.Exec(fmt.Sprintf(`INSERT or REPLACE INTO ttl (id, created, expires) VALUES ("%s", "%d", "%d")`, tt.id, time.Now().Unix(), tt.expiration))
				db.mtx.Unlock()

				expiration, err := db.GetTTLByID(tt.id)
				if tt.err != "" {
					assert.NotNil(err)
					assert.Equal(tt.err, err.Error())
					return
				}
				assert.Nil(err)
				assert.Equal(tt.expiration, expiration)
			})
		}
	}

	t.Run("should return 0 if ttl doesn't exist", func(t *testing.T) {
		t.Parallel()
		assert := assert.New(t)
		expiration, err := db.GetTTLByID("fake-id")
		assert.Nil(err)
		assert.Equal(int64(0), expiration)
	})

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
	db, err := OpenPSDB(dbpath)
	if err != nil {
		t.Errorf("Failed to create database")
		return
	}
	defer os.Remove(dbpath)

	for _, tt := range tests {
		for i := 0; i < parallelCount; i++ {
			t.Run(tt.it, func(t *testing.T) {
				t.Parallel()
				assert := assert.New(t)

				err := db.AddTTLToDB(tt.id, tt.expiration)
				if tt.err != "" {
					assert.NotNil(err)
					assert.Equal(tt.err, err.Error())
					return
				}
				assert.Nil(err)

				db.mtx.Lock()
				rows, err := db.DB.Query(fmt.Sprintf(`SELECT * FROM ttl WHERE id="%s"`, tt.id))
				assert.Nil(err)

				rows.Next()
				var expiration int64
				var id string
				var time int64
				err = rows.Scan(&id, &time, &expiration)
				assert.Nil(err)
				rows.Close()

				db.mtx.Unlock()

				assert.Equal(tt.id, id)
				assert.True(time > 0)
				assert.Equal(tt.expiration, expiration)
			})
		}
	}
}

func TestWriteBandwidthAllocToDB(t *testing.T) {
	tests := []struct {
		it            string
		id            string
		payer, renter string
		size, total   int64
		err           string
	}{
		{
			it:     "should successfully Put Bandwidth Allocation",
			payer:  "payer-id",
			renter: "renter-id",
			size:   5,
			total:  5,
			err:    "",
		},
	}

	tmp, err := ioutil.TempDir("", "example")
	if err != nil {
		log.Fatal(err)
	}
	defer os.RemoveAll(tmp)

	dbpath := filepath.Join(tmp, "test.db")
	db, err := OpenPSDB(dbpath)
	if err != nil {
		t.Errorf("Failed to create database")
		return
	}
	defer os.Remove(dbpath)

	for _, tt := range tests {
		for i := 0; i < parallelCount; i++ {
			t.Run(tt.it, func(t *testing.T) {
				t.Parallel()
				assert := assert.New(t)
				ba := &pb.BandwidthAllocation{
					Signature: []byte{'A', 'B'},
					Data: &pb.BandwidthAllocation_Data{
						Payer: tt.payer, Renter: tt.renter, Size: tt.size, Total: tt.total,
					},
				}
				err = db.WriteBandwidthAllocToDB(ba)
				if tt.err != "" {
					assert.NotNil(err)
					assert.Equal(tt.err, err.Error())
					return
				}
				assert.Nil(err)
				// check db to make sure agreement and signature were stored correctly
				db.mtx.Lock()
				rows, err := db.DB.Query(`SELECT * FROM bandwidth_agreements Limit 1`)
				assert.Nil(err)

				for rows.Next() {
					var (
						agreement []byte
						signature []byte
					)

					err = rows.Scan(&agreement, &signature)
					assert.Nil(err)

					decoded := &pb.BandwidthAllocation_Data{}

					err = proto.Unmarshal(agreement, decoded)
					assert.Nil(err)

					assert.Equal(ba.GetSignature(), signature)
					assert.Equal(ba.Data.GetPayer(), decoded.GetPayer())
					assert.Equal(ba.Data.GetRenter(), decoded.GetRenter())
					assert.Equal(ba.Data.GetSize(), decoded.GetSize())
					assert.Equal(ba.Data.GetTotal(), decoded.GetTotal())

				}
				rows.Close()
				db.mtx.Unlock()
				err = rows.Err()
				assert.Nil(err)
			})
		}
	}
}

func TestMain(m *testing.M) {
	m.Run()
}
