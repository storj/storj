// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.
package overlay

import (
	"context"
	"net/url"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/zeebo/errs"
	"storj.io/storj/pkg/kademlia"
)

func TestRun(t *testing.T) {
	config := Config{}
	bctx := context.Background()
	kad := &kademlia.Kademlia{}
	var key kademlia.CtxKey

	cases := []struct {
		testName string
		testFunc func(t *testing.T)
	}{
		{
			testName: "Run with nil",
			testFunc: func(t *testing.T) {
				err := config.Run(bctx, nil)

				assert.Error(t, err)
				assert.Equal(t, err.Error(), "overlay error: programmer error: kademlia responsibility unstarted")
			},
		},
		{
			testName: "Run with nil, pass pointer to Kademlia in context",
			testFunc: func(t *testing.T) {
				ctx := context.WithValue(bctx, key, kad)
				err := config.Run(ctx, nil)

				assert.Error(t, err)
				assert.Equal(t, err.Error(), "overlay error: database scheme not supported: ")
			},
		},
		{
			testName: "db scheme redis conn fail",
			testFunc: func(t *testing.T) {
				ctx := context.WithValue(bctx, key, kad)
				var config = Config{DatabaseURL: "redis://somedir/overlay.db/?db=1"}
				err := config.Run(ctx, nil)

				assert.Error(t, err)
				assert.Equal(t, err.Error(), "redis error: ping failed: dial tcp: address somedir: missing port in address")
			},
		},
		{
			testName: "db scheme bolt conn fail",
			testFunc: func(t *testing.T) {
				ctx := context.WithValue(bctx, key, kad)
				var config = Config{DatabaseURL: "bolt://somedir/overlay.db"}
				err := config.Run(ctx, nil)

				assert.Error(t, err)
				if !os.IsNotExist(errs.Unwrap(err)) {
					t.Fatal(err.Error())
				}
			},
		},
	}

	for _, c := range cases {
		t.Run(c.testName, c.testFunc)
	}
}

func TestUrlPwd(t *testing.T) {
	res := GetUserPassword(nil)

	assert.Equal(t, res, "")

	uinfo := url.UserPassword("testUser", "testPassword")

	uri := url.URL{User: uinfo}

	res = GetUserPassword(&uri)

	assert.Equal(t, res, "testPassword")
}
