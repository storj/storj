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
	pdbclient "storj.io/storj/pkg/pointerdb/pdbclient"
	"storj.io/storj/pkg/provider"
)

const (
	noLimitGiven        = "limit not given"
	pointerdbClientPort = "8081"
)

var (
	ctx             = context.Background()
	ErrNoLimitGiven = errors.New(noLimitGiven)
	APIKey          = []byte("abc123")
	client          pdbclient.Client
)

func TestMain(m *testing.M) {
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

	if err != nil {
		log.Fatal("Failed to dial: ", zap.Error(err))
		os.Exit(1)
	}

	fmt.Println(client)
	os.Exit(m.Run())
}

func TestList(t *testing.T) {
	tests := []struct {
		bm         string
		startAfter p.Path
		limit      int
		items      []pdbclient.ListItem
		more       bool
		err        error
	}{
		{
			bm:         "should fail with no limit given",
			startAfter: p.New("file1/file2"),
			limit:      0,
			items:      nil,
			more:       false,
			err:        ErrNoLimitGiven,
		},
	}

	for _, tt := range tests {
		t.Run(tt.bm, func(t *testing.T) {
			assert := assert.New(t)

			a := NewAudit(client)
			items, more, err := a.List(ctx, tt.startAfter, tt.limit)

			// if err != nil {
			assert.NotNil(err)
			// 	assert.Equal(tt.err, err.Error())
			// 	t.Errorf("Error: %s", err.Error())
			// }

			fmt.Println(items, more, err)
			// write rest of  test
		})
	}
}
