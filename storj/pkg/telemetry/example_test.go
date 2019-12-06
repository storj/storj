// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package telemetry_test

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/zeebo/admission/admproto"
	"github.com/zeebo/errs"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
	monkit "gopkg.in/spacemonkeygo/monkit.v2"

	"storj.io/storj/pkg/telemetry"
)

// Example is an example of a receiver and sender
func Example() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var group errgroup.Group

	receiver, err := telemetry.Listen("127.0.0.1:0")
	if err != nil {
		log.Fatal(err)
	}

	// receiver
	group.Go(func() (err error) {
		defer func() { err = errs.Combine(err, receiver.Close()) }()
		err = receiver.Serve(ctx, telemetry.HandlerFunc(
			func(application, instance string, key []byte, val float64) {
				fmt.Printf("receive %s %s %s %v\n", application, instance, string(key), val)
			},
		))
		if err == context.Canceled {
			err = nil
		}
		return err
	})

	// sender
	group.Go(func() error {
		client, err := telemetry.NewClient(zap.L(), receiver.Addr(), telemetry.ClientOpts{
			Interval:      time.Second,
			Application:   "example",
			Instance:      telemetry.DefaultInstanceID(),
			Registry:      monkit.Default,
			FloatEncoding: admproto.Float32Encoding,
		})
		if err != nil {
			return err
		}

		client.Run(ctx)
		return nil
	})

	if err := group.Wait(); err != nil {
		fmt.Println(err)
	}
}
