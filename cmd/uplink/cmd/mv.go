// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package cmd

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"
	"github.com/zeebo/errs"

	"storj.io/common/fpath"
	"storj.io/uplink"
)

func init() {
	addCmd(&cobra.Command{
		Use:   "mv SOURCE DESTINATION",
		Short: "Moves a Storj object to another location in Storj",
		RunE:  move,
		Args:  cobra.ExactArgs(2),
	}, RootCmd)

}

func move(cmd *cobra.Command, args []string) (err error) {
	ctx, _ := withTelemetry(cmd)

	src, err := fpath.New(args[0])
	if err != nil {
		return err
	}

	dst, err := fpath.New(args[1])
	if err != nil {
		return err
	}

	if src.IsLocal() || dst.IsLocal() {
		return errors.New("the source and the destination must be a Storj URL")
	}

	project, err := cfg.getProject(ctx, false)
	if err != nil {
		return err
	}
	defer closeProject(project)

	sourceIsPrefix := strings.HasSuffix(src.String(), "/")
	destinationIsPrefix := strings.HasSuffix(dst.String(), "/")

	if destinationIsPrefix != sourceIsPrefix {
		return errs.New("both source and destination should be a prefixes")
	}

	if destinationIsPrefix && sourceIsPrefix {
		return moveObjects(ctx, project, src.Bucket(), src.Path(), dst.Bucket(), dst.Path())
	}

	return moveObject(ctx, project, src.Bucket(), src.Path(), dst.Bucket(), dst.Path())
}

func moveObject(ctx context.Context, project *uplink.Project, oldbucket, oldkey, newbucket, newkey string) error {
	err := project.MoveObject(ctx, oldbucket, oldkey, newbucket, newkey, nil)
	if err != nil {
		return err
	}

	fmt.Printf("sj://%s/%s moved to sj://%s/%s\n", oldbucket, oldkey, newbucket, newkey)
	return nil
}

func moveObjects(ctx context.Context, project *uplink.Project, oldbucket, oldkey, newbucket, newkey string) error {
	oldPrefix := oldkey
	if oldPrefix != "" && !strings.HasSuffix(oldPrefix, "/") {
		oldPrefix += "/"
	}

	objectsIterator := project.ListObjects(ctx, oldbucket, &uplink.ListObjectsOptions{
		Prefix: oldPrefix,
	})
	for objectsIterator.Next() {
		object := objectsIterator.Item()
		if object.IsPrefix {
			continue
		}

		objectKeyWithNewPrefix := strings.TrimPrefix(object.Key, oldPrefix)
		if newkey != "" {
			objectKeyWithNewPrefix = newkey + "/" + objectKeyWithNewPrefix
		}

		err := moveObject(ctx, project, oldbucket, object.Key, newbucket, objectKeyWithNewPrefix)
		if err != nil {
			return err
		}
	}

	return objectsIterator.Err()
}
