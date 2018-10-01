// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package list

import (
	"context"
	"errors"
	"fmt"
	"net/url"

	minio "github.com/minio/minio/cmd"
	"github.com/spf13/cobra"
	"storj.io/mirroring/cmd/utils"
	"storj.io/mirroring/pkg/config"
)

// Function listed as var for testing purposes only
var mirroring = utils.GetObjectLayer

var configuration *config.Config
var Cmd = &cobra.Command{
	Use:   "list",
	Short: "Displays bucket list and all files at specified bucket",
	Long:  `Displays bucket list and all files at specified bucket`,
	Args:  validateArgs,
	RunE:  exec,
}

func validateArgs(cmd *cobra.Command, args []string) error {
	if len(args) > 1 {
		return errors.New("illegal argument length")
	}

	return nil
}
func exec(cmd *cobra.Command, args []string) error {
	objLayer, err := mirroring()
	if err != nil {
		return err
	}

	switch len(args) {
	case 0:
		return listBuckets(objLayer)
	case 1:
		return listObjects(objLayer, args[0])
	default:
		return nil
	}
}

func listBuckets(layer minio.ObjectLayer) error {
	fmt.Println("list buckets:")

	buckets, err := layer.ListBuckets(context.Background())

	if err != nil {

		return errors.New(fmt.Sprintf("Error while creating Listing Buckets Layer: %s\n", err.Error()))
	}

	if buckets != nil {
		for _, bucket := range buckets {
			fmt.Println(bucket.Name)
		}
	}

	return nil
}

func listObjects(layer minio.ObjectLayer, bucketName string) error {
	fmt.Printf("list files at %s\n", bucketName)

	u, err := url.Parse(bucketName)
	if err != nil {

		return errors.New(fmt.Sprintf("Error while parsing bucketName: %s", err.Error()))
	}

	result, err := layer.ListObjectsV2(context.Background(), u.Path, "", "", "", 1000, false, "")
	// result, err := layer.ListObjects(context.Background(), u.Path, "", "", "", 1000)

	if err != nil {

		return errors.New(fmt.Sprintf("Error while creating List Objects Layer for bucket %s,\nError: %s\n", bucketName, err.Error()))
	}

	for _, object := range result.Objects {
		fmt.Println(object.Name)
	}
	// if logger.Err != nil {
	// 	fmt.Printf("error: %s\n", logger.Err.Error())
	// }

	return nil
}

func init() {
	//viper.Set("ListOptions.DefaultOptions.DefaultSource", "test_source")
	//viper.Set("ListOptions.DefaultOptions.ThrowImmediately", true)
	//viper.Set("ListOptions.Merge", false)
}
