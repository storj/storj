// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package kademlia

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	proto "storj.io/storj/protos/overlay"
)

func TestWork(t *testing.T) {
	mu := &sync.Mutex{}
	ctx, cf := context.WithCancel(context.Background())
	cases := []struct {
		worker      *worker
		ctx         context.Context
		expected    map[string]*chore
		expectedErr error
	}{
		{
			ctx: ctx,
			worker: &worker{
				contacted: map[string]*chore{
					"foo": &chore{status: uncontacted, node: &proto.Node{Id: "foo"}},
				},
				mu:          mu,
				maxResponse: 1 * time.Second,
				cancel:      cf,
				find:        proto.Node{Id: "foo"},
				k:           2,
			},
			expectedErr: nil,
		},
	}

	for _, v := range cases {
		if err := v.worker.work(v.ctx); err != nil || v.expectedErr != nil {
			assert.EqualError(t, v.expectedErr, err.Error())
		}

	}
}
