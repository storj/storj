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
	"storj.io/storj/pkg/pb"
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
	s, _, err := getServerAndDB(dbPath)
	assert.NoError(t, err)

	signature := []byte("iamthedummysignatureoftypebyteslice")
	data := []byte("iamthedummydataoftypebyteslice")

	createBwAgreement := &pb.RenterBandwidthAllocation{
		Signature: signature,
		Data:      data,
	}

	/* write to the postgres db in bwagreement table */
	_, err = s.Create(ctx, createBwAgreement)
	assert.NoError(t, err)

	/* read back from the postgres db in bwagreement table */
	retData, err := s.DB.Get_Bwagreement_By_Signature(ctx, dbx.Bwagreement_Signature(signature))
	assert.EqualValues(t, retData.Data, data)
	assert.NoError(t, err)

	/* delete the entry what you just wrote */
	delBool, err := s.DB.Delete_Bwagreement_By_Signature(ctx, dbx.Bwagreement_Signature(signature))
	assert.True(t, delBool)
	assert.NoError(t, err)

}
