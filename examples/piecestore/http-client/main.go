package main

import (
	"log"
	"os"

	"github.com/urfave/cli"
	"github.com/zeebo/errs"
)

type Progress int

const (
	Awaiting    Progress = 0
	In_Progress Progress = 1
	Complete    Progress = 2
	Failed      Progress = 3
)

type shard struct {
	N         int
	Hash      string
	Offset    int64
	Size      int64
	Locations []string
	Progress  Progress
}

type fileMetaData struct {
	Size          int64
	Hash          string
	TotalShards   int
	AvgShardSize  int64
	TailShardSize int64
	Shards        []shard
	Progress      Progress
}

type state struct {
	blacklist []string
	fileMeta  *fileMetaData
	file      *os.File
	filePath  string
}

var ArgError = errs.Class("argError")

func main() {
	app := cli.NewApp()
	app.Name = "storj-client"
	app.Usage = ""
	app.Version = "1.0.0"

	app.Flags = []cli.Flag{}

	app.Commands = []cli.Command{
		{
			Name:      "upload",
			Aliases:   []string{"u"},
			Usage:     "Upload data",
			ArgsUsage: "[path]",
			Action: func(c *cli.Context) error {
				if c.Args().Get(0) == "" {
					return ArgError.New("No path provided")
				}

				err := prepareUpload(c.Args().Get(0))
				if err != nil {
					return err
				}

				return nil
			},
		},
		{
			Name:      "download",
			Aliases:   []string{"d"},
			Usage:     "Download data",
			ArgsUsage: "[hash] [path]",
			Action: func(c *cli.Context) error {
				if c.Args().Get(0) == "" {
					return ArgError.New("No hash provided")
				}

				if c.Args().Get(1) == "" {
					return ArgError.New("No path provided")
				}

				err := prepareDownload(c.Args().Get(0), c.Args().Get(1))
				if err != nil {
					return err
				}

				return nil
			},
		},
		{
			Name:    "list-files",
			Aliases: []string{"l"},
			Usage:   "List all files",
			Action: func(c *cli.Context) error {
				err := listFiles()
				if err != nil {
					return err
				}

				return nil
			},
		},
	}

	err := app.Run(os.Args)
	if err != nil {
		log.Fatal(err)
	}
}
