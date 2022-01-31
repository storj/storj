// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"
	"time"

	"github.com/spf13/cobra"

	"storj.io/common/fpath"
)

var (
	putProgress *bool
	putExpires  *string
	putMetadata *string
)

func init() {
	putCmd := addCmd(&cobra.Command{
		Use:   "put sj://BUCKET/KEY",
		Short: "Copies data from standard in to a Storj object",
		RunE:  putMain,
		Args:  cobra.ExactArgs(1),
	}, RootCmd)

	putProgress = putCmd.Flags().Bool("progress", false, "if true, show upload progress")
	putExpires = putCmd.Flags().String("expires", "", "optional expiration date of the new object. Please use format (yyyy-mm-ddThh:mm:ssZhh:mm)")
	putMetadata = putCmd.Flags().String("metadata", "", "optional metadata for the object. Please use a single level JSON object of string to string only")

	setBasicFlags(putCmd.Flags(), "progress", "expires", "metadata")
}

// putMain is the function executed when putCmd is called.
func putMain(cmd *cobra.Command, args []string) (err error) {
	if len(args) == 0 {
		return fmt.Errorf("no object specified for copy")
	}

	ctx, _ := withTelemetry(cmd)

	dst, err := fpath.New(args[0])
	if err != nil {
		return err
	}

	if dst.IsLocal() {
		return fmt.Errorf("no bucket specified, use format sj://bucket/")
	}

	src, err := fpath.New("-")
	if err != nil {
		return err
	}

	var expiration time.Time
	if *putExpires != "" {
		expiration, err = time.Parse(time.RFC3339, *putExpires)
		if err != nil {
			return err
		}
		if expiration.Before(time.Now()) {
			return fmt.Errorf("invalid expiration date: (%s) has already passed", *putExpires)
		}
	}

	return upload(ctx, src, dst, expiration, []byte(*putMetadata), *putProgress)
}
