// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package bwagreement

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	dbx "storj.io/storj/pkg/agreementreceiver/dbx"
)

var (
	ctx = context.Background()
)

const (
	host     = "localhost"
	port     = 5432
	user     = "postgres"
	password = "your-password"
	dbname   = "calhounio_demo"
)

func getDBPath() string {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"password=%s dbname=%s sslmode=disable",
		host, port, user, password, dbname)
	return psqlInfo
}

func getServerAndDB(path string) (statdb *Server, db *dbx.DB, err error) {

	// db, err = dbx.Open("postgres", path)
	// if err != nil {
	// 	panic(err)
	// }
	// defer db.Close()
	// return nil, nil, err
	statdb, err = NewServer("postgres", path, zap.NewNop())
	if err != nil {
		return &Server{}, &dbx.DB{}, err
	}
	db, err = dbx.Open("postgres", path)
	if err != nil {
		return &Server{}, &dbx.DB{}, err
	}
	return statdb, db, err
}

func TestCreateExists(t *testing.T) {
	dbPath := getDBPath()
	_, _, err := getServerAndDB(dbPath)
	assert.NoError(t, err)
}
