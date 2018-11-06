// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package bwagreement

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/jedib0t/go-pretty/table"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
	"google.golang.org/grpc"

	"storj.io/storj/pkg/pb"
	"storj.io/storj/pkg/piecestore/rpc/server/psdb"
	"storj.io/storj/pkg/provider"
)

var (
	ctx = context.Background()
)

const (
	host     = "localhost"
	port     = 5432
	user     = "postgres"
	password = "your-password"
	dbname   = "pointerdb"
)

func getPSQLInfo() string {
	psqlInfo := fmt.Sprintf("host=%s port=%d user=%s "+
		"dbname=%s sslmode=disable",
		host, port, user, dbname)
	return psqlInfo
}

func TestBandwidthAgreements(t *testing.T) {
	TS := NewTestServer(t)
	defer TS.Stop()

	signature := []byte("iamthedummcxvignatureoftypebyteslice")
	data := []byte("iamthedummydataoftypebyteslice")

	bwAgreements, err := readSampleDataFromPsdb()
	assert.NoError(t, err)

	/* emulate sending the bwagreement stream from piecestore node */
	stream, err := TS.c.BandwidthAgreements(ctx)
	assert.NoError(t, err)

	for _, v := range bwAgreements {
		for i, j := range v {
			rbad := &pb.RenterBandwidthAllocation_Data{}
			if err := proto.Unmarshal(j.Agreement, rbad); err != nil {
				assert.Error(t, err)
			}
			signature = rbad.GetPayerAllocation().GetSignature()
			fmt.Println("signature=", fmt.Sprintf("%s", signature))
			data = rbad.GetPayerAllocation().GetData()
			fmt.Println("data=", fmt.Sprintf("%s", data))

			msg := &pb.RenterBandwidthAllocation{
				Signature: signature,
				Data:      j.Agreement,
			}

			err = stream.Send(msg)
			assert.NoError(t, err)
			fmt.Println("I=", i)

			time.Sleep(20 * time.Millisecond)

			// /* read back from the postgres db in bwagreement table */
			// retData, err := TS.s.DB.Get_Bwagreement_By_Signature(ctx, dbx.Bwagreement_Signature(signature))
			// assert.EqualValues(t, retData.Data, data)
			// assert.NoError(t, err)

			// /* delete the entry what you just wrote */
			// delBool, err := TS.s.DB.Delete_Bwagreement_By_Signature(ctx, dbx.Bwagreement_Signature(signature))
			// assert.True(t, delBool)
			// assert.NoError(t, err)
		}
	}
	_, _ = stream.CloseAndRecv()
}

type TestServer struct {
	s     *Server
	grpcs *grpc.Server
	conn  *grpc.ClientConn
	c     pb.BandwidthClient
	k     crypto.PrivateKey
}

func NewTestServer(t *testing.T) *TestServer {
	check := func(e error) {
		if !assert.NoError(t, e) {
			t.Fail()
		}
	}

	caS, err := provider.NewTestCA(context.Background())
	check(err)
	fiS, err := caS.NewIdentity()
	check(err)
	so, err := fiS.ServerOption()
	check(err)

	caC, err := provider.NewTestCA(context.Background())
	check(err)
	fiC, err := caC.NewIdentity()
	check(err)
	co, err := fiC.DialOption()
	check(err)

	s := newTestServerStruct(t)
	grpcs := grpc.NewServer(so)

	k, ok := fiC.Key.(*ecdsa.PrivateKey)
	assert.True(t, ok)
	ts := &TestServer{s: s, grpcs: grpcs, k: k}
	addr := ts.start()
	ts.c, ts.conn = connect(addr, co)

	return ts
}

func newTestServerStruct(t *testing.T) *Server {
	//psqlInfo := getPSQLInfo()
	psqlInfo := "postgres://postgres@localhost/pointerdb?sslmode=disable"
	s, err := NewServer("postgres", psqlInfo, zap.NewNop())
	assert.NoError(t, err)
	return s
}

func (TS *TestServer) start() (addr string) {
	lis, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	pb.RegisterBandwidthServer(TS.grpcs, TS.s)

	go func() {
		if err := TS.grpcs.Serve(lis); err != nil {
			log.Fatalf("failed to serve: %v", err)
		}
	}()
	return lis.Addr().String()
}

func connect(addr string, o ...grpc.DialOption) (pb.BandwidthClient, *grpc.ClientConn) {
	conn, err := grpc.Dial(addr, o...)
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}

	c := pb.NewBandwidthClient(conn)

	return c, conn
}

func (TS *TestServer) Stop() {
	if err := TS.conn.Close(); err != nil {
		panic(err)
	}
	TS.grpcs.Stop()
}

// call this function to copy signature and data into postgres db
func readSampleDataFromPsdb() (map[string][]*psdb.Agreement, error) {
	// open the sql db
	dbpath := filepath.Join("/Users/kishore/.storj/capt/f37/data", "piecestore.db")
	db, err := psdb.Open(context.Background(), "", dbpath)
	if err != nil {
		fmt.Println("Storagenode database couldnt open:", dbpath)
		return nil, err
	}
	bwAgreements, err := db.GetBandwidthAllocations()
	if err != nil {
		return nil, err
	}

	//return bwAgreements, err
	// Agreement is a struct that contains a bandwidth agreement and the associated signature
	type SatAttributes struct {
		TotalBytes        int64
		PutActionCount    int64
		GetActionCount    int64
		TotalTransactions int64
		// additional attributes add here ...
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"SatelliteID", "Total", "UplinkID", "Put Action", "Get Action"})

	// attributes per satelliteid
	satelliteID := make(map[string]SatAttributes)
	satAtt := SatAttributes{}
	var currSatID, lastSatID string

	for _, v := range bwAgreements {
		for _, j := range v {
			// deserializing rbad you get payerbwallocation, total & storage node id
			rbad := &pb.RenterBandwidthAllocation_Data{}
			if err := proto.Unmarshal(j.Agreement, rbad); err != nil {
				return nil, err
			}
			total := rbad.GetTotal()

			// deserializing pbad you get satelliteID, uplinkID, max size, exp, serial# & action
			pbad := &pb.PayerBandwidthAllocation_Data{}
			fmt.Println("signature=", fmt.Sprintf("%s", rbad.GetPayerAllocation().GetSignature()))
			fmt.Println("data=", fmt.Sprintf("%s", rbad.GetPayerAllocation().GetData()))
			if err := proto.Unmarshal(rbad.GetPayerAllocation().GetData(), pbad); err != nil {
				return nil, err
			}
			currSatID = fmt.Sprintf("%s", pbad.GetSatelliteId())
			action := pbad.GetAction()

			if strings.Compare(currSatID, lastSatID) != 0 {
				// make an entry
				satAtt.TotalBytes = total
				if action == pb.PayerBandwidthAllocation_PUT {
					satAtt.PutActionCount++
				} else {
					satAtt.GetActionCount++
					fmt.Println(satAtt.GetActionCount)
				}
				satAtt.TotalTransactions++
				satelliteID[currSatID] = satAtt
				lastSatID = currSatID
			} else {
				//update the already existing entry
				for satIDKey, satIDVal := range satelliteID {
					if strings.Compare(satIDKey, currSatID) == 0 {
						satIDVal.TotalBytes = satIDVal.TotalBytes + total
						if action == pb.PayerBandwidthAllocation_PUT {
							satIDVal.PutActionCount++
						} else {
							satIDVal.GetActionCount++
						}
						satIDVal.TotalTransactions++
						satelliteID[satIDKey] = satIDVal
					}
				}
			}
		}
		t.AppendRow([]interface{}{currSatID, satelliteID[currSatID].TotalBytes, satelliteID[currSatID].TotalTransactions, satelliteID[currSatID].PutActionCount, satelliteID[currSatID].GetActionCount})
	}

	t.SetStyle(table.StyleLight)
	t.Render()
	return bwAgreements, err
}
