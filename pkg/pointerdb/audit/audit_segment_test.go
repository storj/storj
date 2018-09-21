package audit

import (
	"context"
	"errors"
	"fmt"
	"testing"

	p "storj.io/storj/pkg/paths"
	pdbclient "storj.io/storj/pkg/pointerdb/pdbclient"

	"github.com/stretchr/testify/assert"
)

const (
	noLimitGiven = "limit not given"
)

var (
	ctx             = context.Background()
	ErrNoLimitGiven = errors.New(noLimitGiven)
)

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

			// create  mock for PDB client?
			a := NewAudit()
			items, more, err := List(ctx, nil, tt.startAfter, nil, tt.more, tt.limit, 0)

			if err != nil {
				assert.NotNil(err)
				assert.Equal(tt.err, err.Error())
				t.Errorf("Error: %s", err.Error())
			}

			fmt.Println(items, more, err)
			// write rest of  test
		})
	}
}
