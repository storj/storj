// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package cp

import (
	"context"
	"errors"
	"fmt"
	"github.com/minio/minio-go/pkg/s3utils"
	minio "github.com/minio/minio/cmd"
	"github.com/spf13/cobra"
	cmdUtils "storj.io/mirroring/cmd/utils"
	"storj.io/mirroring/utils"
)

var mirroring, _ = cmdUtils.GetObjectLayer()
var missingArgsErrorMessage = "at least three arguments required."

var Cmd = &cobra.Command {
	Use: "copy [cp] srcBucket, srcObj, dstBucket, dstObj(OPTIONAL).",

	Args: validateArgs,
	Short: "Creates a cp of an object.",
	Long:  "Creates a cp of an object that is already stored in a bucket. dstObj is optional. " +
		   "If not specified, dstObj name will be as srcObj.",
	RunE: exec,
}

func exec(cmd *cobra.Command, args []string) error {

	ctx := context.Background()

	objectInfo, _ := mirroring.GetObjectInfo(ctx, args[0], args[1], minio.ObjectOptions{})

	dstObj := args[1]

	if len(args) == 4 {
		dstObj = args[3]
	}

	//TODO: enable object options in future
	_, err := mirroring.CopyObject(ctx, args[0], args[1], args[2], dstObj, objectInfo, minio.ObjectOptions{}, minio.ObjectOptions{})

	if err != nil {
		return err
	}

	fmt.Printf("Object %s/%s copied\n", args[0], args[1])

	return nil
}

func validateArgs(cmd *cobra.Command, args []string) error {

	switch len(args) {
		case 0, 1, 2:
			return errors.New(missingArgsErrorMessage)
		case 3, 4:
			srcBucketNameErr := s3utils.CheckValidBucketName(args[0])
			dstBucketNameErr := s3utils.CheckValidBucketName(args[2])
			srcObjectNameErr := s3utils.CheckValidObjectName(args[1])

			var dstObjectNameErr error = nil

			if len(args) == 4 {
				dstObjectNameErr = s3utils.CheckValidObjectName(args[3])
			}

			err := utils.CombineErrors([]error {
				utils.NewError(srcBucketNameErr, "srcBucket - "),
				utils.NewError(dstBucketNameErr, "dstBucket - "),
				utils.NewError(srcObjectNameErr, "srcObject - "),
				utils.NewError(dstObjectNameErr, "dstObject - "),
			})

			if err != nil {
				return err
			}

			return nil

		default:
			return errors.New("too many arguments")
	}

	return nil
}

func init() {

}
