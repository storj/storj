// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package testuplink_test

import (
	"context"
	"flag"
	"io/ioutil"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"

	"storj.io/storj/cmd/uplink/test/cmd"
	"storj.io/storj/internal/testplanet"
	"storj.io/storj/pkg/cfgstruct"
)

const (
	apiKey = "apiKey"
)

var ctx = context.Background()

func TestMB(t *testing.T) {
	tests := []struct {
		bucket string
	}{
		{
			bucket: "sj://bucket",
		},
	}

	for _, tt := range tests {
		t.Log("New")
		planet, err := testplanet.New(t, 1, 0, 1)
		assert.NoError(t, err)

		defer func() {
			t.Log("Shutdown")
			err = planet.Shutdown()
			if err != nil {
				t.Fatal(err)
			}
		}()

		err = flag.Set("pointer-db.auth.api-key", apiKey)
		assert.NoError(t, err)

		cfgstruct.Bind(&flag.FlagSet{}, &planet.Uplinks[0].Client)

		uplink := planet.Uplinks[0]

		uplink.Client.OverlayAddr = planet.Satellites[0].Addr()
		uplink.Client.PointerDBAddr = planet.Satellites[0].Addr()
		uplink.Client.APIKey = apiKey

		t.Log("Start")
		planet.Start(ctx)

		err = testuplink.MB(ctx, uplink, tt.bucket)
		assert.NoError(t, err)
	}
}

func TestCP(t *testing.T) {
	tests := []struct {
		bucket     string
		k, m, o, n int
	}{
		{
			bucket: "sj://bucket",
			k:      25,
			m:      29,
			o:      35,
			n:      40,
		},
	}

	for _, tt := range tests {
		t.Log("New")
		planet, err := testplanet.New(t, 1, 60, 1)
		assert.NoError(t, err)

		defer func() {
			t.Log("Shutdown")
			err = planet.Shutdown()
			if err != nil {
				t.Fatal(err)
			}
		}()

		err = flag.Set("pointer-db.auth.api-key", apiKey)
		assert.NoError(t, err)

		cfgstruct.Bind(&flag.FlagSet{}, &planet.Uplinks[0].Client)

		uplink := planet.Uplinks[0]

		uplink.Client.OverlayAddr = planet.Satellites[0].Addr()
		uplink.Client.PointerDBAddr = planet.Satellites[0].Addr()
		uplink.Client.APIKey = apiKey

		uplink.Client.MinThreshold = tt.k
		uplink.Client.RepairThreshold = tt.m
		uplink.Client.SuccessThreshold = tt.o
		uplink.Client.MaxThreshold = tt.n

		t.Log("Start")
		planet.Start(ctx)

		time.Sleep(5 * time.Second)

		err = testuplink.MB(ctx, uplink, tt.bucket)
		assert.NoError(t, err)

		content := []byte{}
		for i := 0; i < 5000; i++ {
			content = append(content, 'a')
		}

		tmpDir, err := ioutil.TempDir("", "test")
		assert.NoError(t, err)

		defer func() {
			err = os.RemoveAll(tmpDir)
			if err != nil {
				t.Log(err)
			}
		}()
		
		fpath := filepath.Join(tmpDir, "testfile")

		err = ioutil.WriteFile(fpath, content, 0666)
		assert.NoError(t, err)

		err = testuplink.CP(ctx, uplink, []string{fpath, tt.bucket})
		assert.NoError(t, err)

		// download file and verify data is the same

		// dwnld := filepath.Join(tmpDir, "testdownload")

		// err = testuplink.CP(ctx, uplink, []string{"sj://bucket/testfile", dwnld})
		// assert.NoError(t, err)

		// f, err := os.Open(dwnld)
		// assert.NoError(t, err)

		// buf := make([]byte, len(content))
		// _, err = f.Read(buf)
		// assert.NoError(t, err)

		//assert.Equal(t, content, buf)
	}
}
