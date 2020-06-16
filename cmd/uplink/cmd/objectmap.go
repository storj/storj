// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/storj/storjmap"

	"storj.io/common/fpath"
)

func init() {
	objmapCmd := &cobra.Command{
		Use:   "objmap [sj://BUCKET/PATH]",
		Short: "Generate a map of geolocations of nodes holding object pieces",
		RunE:  objectMap,
		Args:  cobra.MaximumNArgs(1),
	}
	RootCmd.AddCommand(objmapCmd)
}

func objectMap(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := withTelemetry(cmd)

	path, err := fpath.New(args[0])
	if err != nil {
		return err
	}
	if path.IsLocal() {
		return fmt.Errorf("source must be Storj URL: %s", path)
	}

	project, err := cfg.getProject(ctx, false)
	if err != nil {
		return err
	}
	defer closeProject(project)

	loc, err := project.GetObjectLocation(ctx, path.Bucket(), path.String())
	if err != nil {
		return err
	}

	locations := make([]storjmap.Location, len(loc))
	for _, l := range loc {
		if l == nil {
			continue
		}
		locations = append(locations, storjmap.Location{
			Latitude:  l.Latitude,
			Longitude: l.Longitude,
		})
	}

	storjmap.GenMap(locations, "/tmp/out.html")
}
