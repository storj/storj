// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information

package testuplink_test

import (
	"context"
	"flag"
	"fmt"
	"io/ioutil"
	"testing"
	"os"
	"path/filepath"

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
		planet, err := testplanet.New(1, 0, 1)
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
		bucket string
	}{
		{
			bucket: "sj://bucket",
		},
	}

	for _, tt := range tests {
		t.Log("New")
		planet, err := testplanet.New(1, 40, 1)
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
		
		//content := []byte("temporary file's content")
		tmpDir, err := ioutil.TempDir("", "test")
		assert.NoError(t, err)

		defer os.RemoveAll(tmpDir)

		fpath := filepath.Join(tmpDir, "testfile")
		// err = ioutil.WriteFile(fpath, content, 0666)
		// assert.NoError(t, err)
		f1, err := os.Create(fpath)
		assert.NoError(t, err)

		err = f1.Truncate(1e7)
		assert.NoError(t, err)

		err = testuplink.CP(ctx, uplink, []string{fpath, tt.bucket})
		assert.NoError(t, err)

		// dwnld := filepath.Join(tmpDir, "testdownload")
		
		// err = testuplink.CP(ctx, uplink, []string{"sj://bucket/testfile", dwnld})
		// assert.NoError(t, err)
		
		// f2, err := os.Open(dwnld)
		// assert.NoError(t, err)


		fmt.Println("HI")

		// buf := make([]byte, len(content))
		// n, err := f.Read(buf)
		// assert.NoError(t, err)

		// fmt.Printf("%d bytes: %s\n", n, string(buf))



		// srcInfo, err := f1.Stat()
		// assert.NoError(t, err)

		// dstInfo, err := f2.Stat()
		// assert.NoError(t, err)

		// fmt.Printf("src size: %d, dst size %d", srcInfo.Size(), dstInfo.Size())
	}
}
