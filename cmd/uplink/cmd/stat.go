// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"storj.io/storj/internal/fpath"
	"storj.io/storj/pkg/process"
)

func init() {
	addCmd(&cobra.Command{
		Use:   "stat",
		Short: "stat a Storj object",
		RunE:  statObject,
	}, RootCmd)
}

// copyMain is the function executed when cpCmd is called
func statObject(cmd *cobra.Command, args []string) (err error) {
	ctx := process.Ctx(cmd)

	if len(args) == 0 {
		return fmt.Errorf("No object specified for stat")
	}

	dst, err := fpath.New(args[0])
	if err != nil {
		return err
	}

	if dst.IsLocal() {
		return fmt.Errorf("No bucket specified, use format sj://bucket/")
	}

	metainfo, _, err := cfg.Metainfo(ctx)
	if err != nil {
		return err
	}

	obj, err := metainfo.GetObject(ctx, dst.Bucket(), dst.Path())
	if err != nil {
		return err
	}

	// initialize the table header (fields)
	const padding = 3
	w := tabwriter.NewWriter(os.Stdout, 0, 0, padding, ' ', tabwriter.AlignRight|tabwriter.Debug)
	fmt.Fprintln(w, "Version\tBucket\tPath\tIsPrefix\tSize\t# of Segments\tSegment Size\tPieceID\tNeeded\tOnline\t")
	fmt.Fprint(w, obj.Version, "\t", obj.Bucket.Name, "\t", obj.Path, "\t", obj.IsPrefix, "\t",
		obj.Stream.Size, "\t", obj.Stream.SegmentCount, "\t", "-",
		"\t", "-", "\t", "-", "\t", "-", "\t\n")

	// populate the row fields
	for _, segInfo := range obj.SegmentList {
		fmt.Fprint(w, "-", "\t", "-", "\t", "-", "\t", "-", "\t",
			"-", "\t", segInfo.Index, "\t", segInfo.Size,
			"\t", segInfo.PieceID.String(), "\t", segInfo.Needed, "\t", segInfo.Online, "\t\n")
	}

	// display the data
	err = w.Flush()
	return err
}
