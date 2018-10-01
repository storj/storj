// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package get

import (
	"fmt"
	"github.com/spf13/cobra"
	"storj.io/mirroring/global"
	gw "storj.io/mirroring/pkg/gateway"
	"github.com/minio/minio/pkg/auth"
	"storj.io/mirroring/cmd/get/downloader"
	"storj.io/mirroring/utils"
)

// getCmd represents the get command
var Cmd = &cobra.Command{
	Use:   "get [bucket name] [object name](opt) [OPTIONS]",
	Args: validateArgs,
	Short: "Download files and buckets",
	Long: ``,
	Run: run,
}

func run(cmd *cobra.Command, args []string) {
	fmt.Println("get called")

	//for i := range args {
	//	fmt.Printf("Arg%d: %s\n", i, args[i])
	//}

	fmt.Printf("Filename: %s\n", nameFlag)
	fmt.Printf("Cwd: %s\n", global.Params.GetCwd())

	cwd := global.Params.GetCwd()
	if cwd == "" {
		fmt.Printf("Unable to get current working directory\n")
		return
	}

	var mirrGateway gw.Mirroring = gw.Mirroring{Logger: &utils.StdOutLogger}
	var mirr, err =  mirrGateway.NewGatewayLayer(auth.Credentials{})
	if err != nil {
		fmt.Printf("Unable to start mirroring service...\n")
		return
	}

	params := downloader.NewDefaultParams()
	if nameFlag != "" {
		params.SetPath(nameFlag)
	}
	if prefixFlag != "" {
		params.SetPrefix(prefixFlag)
	}
	if recursiveFlag {
		params.SetRecursive(true)
	}

	dwn := downloader.NewDownloader(mirr, cwd, params)

	var bucketName string
	var objectName string
	bucketName = args[0]
	if len(args) == maxArg {
		objectName = args[1]
	}

	if objectName == "" {
		err = dwn.GetBucket(bucketName)
	} else {
		err = dwn.GetObject(bucketName, objectName)
	}

	if err != nil {
		fmt.Printf("Download error: %s\n", err)
		return
	}

	fmt.Println("The end")
}

func validateArgs(cmd *cobra.Command, args []string) error {
	argsLen := len(args)
	if argsLen < minArg || argsLen > maxArg {
		return NewArgsError(args)
	}

	return nil
}

var (
	minArg = 1
	maxArg = 2

	nameFlag string
	nameUsage = "Path of the file or folder to be downloaded. A raw filename can be used to download to current directory under that name.\n" +
		"If no objectname provided folder under that name will be created"

	prefixFlag string
	prefixUsage = ""

	recursiveFlag bool
	recursiveUsage = ""
)

func init() {
	Cmd.Flags().StringVarP(&nameFlag, "name", "n", "", nameUsage)
	Cmd.Flags().StringVarP(&prefixFlag, "prefix", "p", "", prefixUsage)
	Cmd.Flags().BoolVarP(&recursiveFlag, "recursive", "r", false, recursiveUsage)
}