package audit

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"

	p "storj.io/storj/pkg/paths"
	"storj.io/storj/pkg/pb"
	pdbclient "storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/provider"
)

const (
	noLimitGiven        = "limit not given"
	pointerdbClientPort = ":8080"
)

var (
	ctx             = context.Background()
	ErrNoLimitGiven = errors.New(noLimitGiven)
	APIKey          = []byte("abc123")
)

func TestAuditSegment(t *testing.T) {
	ca, err := provider.NewCA(ctx, 12, 4)
	if err != nil {
		log.Fatal("Failed to create certificate authority: ", zap.Error(err))
		os.Exit(1)
	}
	identity, err := ca.NewIdentity()
	if err != nil {
		log.Fatal("Failed to create full identity: ", zap.Error(err))
		os.Exit(1)
	}

	client, err := pdbclient.NewClient(identity, pointerdbClientPort, APIKey)
	fmt.Println("this is  the client: ", client)
	if err != nil {
		log.Fatal("Failed to dial: ", zap.Error(err))
		os.Exit(1)
	}

	fmt.Println("this is the  client: ", client)

	t.Run("List", func(t *testing.T) {

		tests := []struct {
			bm         string
			path       p.Path
			APIKey     []byte
			startAfter p.Path
			limit      int
			items      []pdbclient.ListItem
			more       bool
			err        error
		}{
			{
				bm:         "should fail with no limit given",
				path:       p.New("file1/file2"),
				APIKey:     []byte("abc123"),
				startAfter: p.New("file3/file4"),
				limit:      0,
				items:      nil,
				more:       false,
				err:        ErrNoLimitGiven,
			},
		}

		for i, tt := range tests {
			t.Run(tt.bm, func(t *testing.T) {
				assert := assert.New(t)
				errTag := fmt.Sprintf("Test case #%d", i)

				// create a pointer and put in db
				fmt.Println("this is  the client again: ", client)
				putRequest := makePointer(tt.path, tt.APIKey)
				fmt.Println("this is hte pr: ", putRequest)

				err := client.Put(ctx, tt.path, putRequest.Pointer)
				fmt.Println("this is the err for put request: ", err)

				if err != nil {
					assert.NotNil(t, err, errTag)
				} else {
					assert.Nil(t, err, errTag)
				}

				// call LIST
				// a := NewAudit(client)
				// items, more, err := a.List(ctx, tt.startAfter, tt.limit)

				// if err != nil {
				// 	assert.NotNil(err)
				// 	//assert.Equal(tt.err, tt.err)
				// 	t.Errorf("Error: %s", err.Error())
				// }

				//fmt.Println("this is items: ", items, more)
				// write rest of  test
			})
		}
	})
}

func makePointer(path p.Path, auth []byte) pb.PutRequest {
	var rps []*pb.RemotePiece
	rps = append(rps, &pb.RemotePiece{
		PieceNum: 1,
		NodeId:   "testId",
	})
	pr := pb.PutRequest{
		Path: path.String(),
		Pointer: &pb.Pointer{
			Type: pb.Pointer_REMOTE,
			Remote: &pb.RemoteSegment{
				Redundancy: &pb.RedundancyScheme{
					Type:             pb.RedundancyScheme_RS,
					MinReq:           1,
					Total:            3,
					RepairThreshold:  2,
					SuccessThreshold: 3,
				},
				PieceId:      "testId",
				RemotePieces: rps,
			},
			Size: int64(1),
		},
		APIKey: auth,
	}
	return pr
}
