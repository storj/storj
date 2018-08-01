// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package psdb

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/stretchr/testify/assert"

	"golang.org/x/net/context"
)

var ctx = context.Background()

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

			tmp := os.TempDir()
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

	tmp := os.TempDir()
	dbpath := filepath.Join(tmp, "test.db")
	db, err := OpenPSDB(dbpath)
	if err != nil {
		t.Errorf("Failed to create database")
		return
	}
	defer os.Remove(dbpath)

	for _, tt := range tests {
		t.Run(tt.it, func(t *testing.T) {
			assert := assert.New(t)
			db.DB.Exec(fmt.Sprintf(`INSERT or REPLACE INTO ttl (id, created, expires) VALUES ("%s", "%d", "%d")`, tt.id, time.Now().Unix(), 0))

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

	tmp := os.TempDir()
	dbpath := filepath.Join(tmp, "test.db")
	db, err := OpenPSDB(dbpath)
	if err != nil {
		t.Errorf("Failed to create database")
		return
	}
	defer os.Remove(dbpath)

	for _, tt := range tests {
		t.Run(tt.it, func(t *testing.T) {
			assert := assert.New(t)
			db.DB.Exec(fmt.Sprintf(`INSERT or REPLACE INTO ttl (id, created, expires) VALUES ("%s", "%d", "%d")`, tt.id, time.Now().Unix(), tt.expiration))

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

	t.Run("should return 0 if ttl doesn't exist", func(t *testing.T) {
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
			it:         "should successfully Get TTL by ID",
			id:         "Butts",
			expiration: 666,
			err:        "",
		},
	}

	tmp := os.TempDir()
	dbpath := filepath.Join(tmp, "test.db")
	db, err := OpenPSDB(dbpath)
	if err != nil {
		t.Errorf("Failed to create database")
		return
	}
	defer os.Remove(dbpath)

	for _, tt := range tests {
		t.Run(tt.it, func(t *testing.T) {
			assert := assert.New(t)

			err := db.AddTTLToDB(tt.id, tt.expiration)
			if tt.err != "" {
				assert.NotNil(err)
				assert.Equal(tt.err, err.Error())
				return
			}
			assert.Nil(err)

			rows, err := db.DB.Query(fmt.Sprintf(`SELECT * FROM ttl WHERE id="%s"`, tt.id))
			assert.Nil(err)

			rows.Next()
			var expiration int64
			var id string
			var time int64
			err = rows.Scan(&id, &time, &expiration)
			assert.Nil(err)

			assert.Equal(tt.id, id)
			assert.True(time > 0)
			assert.Equal(tt.expiration, expiration)
		})
	}
}

func TestMain(m *testing.M) {
	m.Run()
}
