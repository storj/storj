// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package telemetry

import (
	"context"
	"runtime"
	"testing"

	"github.com/spacemonkeygo/monkit/v3"
	"github.com/stretchr/testify/assert"
	"go.uber.org/zap/zaptest"
)

var (
	testMon = monkit.ScopeNamed("testpkg")
)

func TestMetrics(t *testing.T) {
	if runtime.GOOS == "windows" {
		//TODO (windows): currently closing doesn't seem to be shutting down the server
		t.Skip("broken")
	}

	s, err := Listen("127.0.0.1:0")
	assert.NoError(t, err)
	defer func() { _ = s.Close() }()

	c, err := NewClient(zaptest.NewLogger(t), s.Addr(), ClientOpts{
		Application: "testapp",
		Instance:    "testinst",
	})
	assert.NoError(t, err)

	testMon.IntVal("testint").Observe(3)

	errs := make(chan error, 3)
	go func() {
		errs <- c.Report(context.Background())
	}()
	go func() {
		errs <- s.Serve(context.Background(), HandlerFunc(
			func(application, instance string, key []byte, val float64) {
				assert.Equal(t, application, "testapp")
				assert.Equal(t, instance, "testinst")
				if string(key) == "testint,scope=testpkg recent" {
					assert.Equal(t, val, float64(3))
					errs <- nil
				}
			}))
	}()

	// three possible errors:
	//  * reporting will send an error or nil,
	//  * receiving will send an error or nil,
	//  * serving will return an error
	// in the good case serving should return last and should return a closed
	// error
	for i := 0; i < 2; i++ {
		err := <-errs
		assert.NoError(t, err)
	}
	assert.NoError(t, s.Close())

	err = <-errs
	assert.Error(t, err)
}
