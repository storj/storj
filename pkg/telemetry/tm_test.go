// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package telemetry

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/spacemonkeygo/monkit.v2"
)

var (
	testMon = monkit.ScopeNamed("testpkg")
)

func TestMetrics(t *testing.T) {
	s, err := Listen(":0")
	assert.NoError(t, err)
	defer s.Close()

	c, err := NewClient(s.Addr(), ClientOpts{
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
				if string(key) == "testpkg.testint.recent" {
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
	s.Close()
	err = <-errs
	assert.Error(t, err)
}
