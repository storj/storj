// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package downloader

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/aleitner/FilePiece"
	"github.com/zeebo/errs"

	"storj.io/storj/examples/piecestore/http/client/utils"
)

var downError = errs.Class("downloadError")

// Prepare shard and download it
func downloadShard(i int, downloadState *utils.State) error {
	fileMeta := downloadState.FileMeta
	shard := &fileMeta.Shards[i-1]
	shard.Progress = utils.InProgress

	if i == fileMeta.TotalShards && fileMeta.TailShardSize > 0 {
		shard.Size = fileMeta.TailShardSize
	} else {
		shard.Size = fileMeta.AvgShardSize
	}

	form := url.Values{}
	form.Add("offset", "0")
	form.Add("hash", shard.Hash)
	form.Add("length", strconv.FormatInt(shard.Size, 10))
	req, err := http.NewRequest("POST", shard.Locations[0]+"/download", strings.NewReader(form.Encode()))

	req.Header.Add("Content-Type", "application/x-www-form-urlencoded")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}

	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// Write the body to file
	chunk := fpiece.NewChunk(downloadState.File, shard.Offset, shard.Size)
	_, err = io.Copy(chunk, resp.Body)
	if err != nil {
		return err
	}

	if err != nil {
		return err
	}

	if resp.StatusCode == 200 {
		shard.Progress = utils.Complete
		fmt.Printf("Successfully downloaded shard (%v) from farmer (%s)\n", i, shard.Locations[0])
	} else {
		shard.Progress = utils.Awaiting
		fmt.Printf("Failed to download shard (%v) from farmer (%s)\n", i, shard.Locations[0])
	}

	go queueDownload(downloadState)

	return nil
}

// work queue
func queueDownload(downloadState *utils.State) {
	fileMeta := downloadState.FileMeta

	for i := 1; i <= fileMeta.TotalShards; i++ {
		if fileMeta.Shards[i-1].Progress == utils.Awaiting {
			go downloadShard(i, downloadState)
		}
	}

	completed := 0
	for i := 1; i <= fileMeta.TotalShards; i++ {
		if fileMeta.Shards[i-1].Progress == utils.Complete {
			completed++
		}
	}

	if completed == fileMeta.TotalShards {
		fileMeta.Progress = utils.Complete
	}
}

// PrepareDownload -- Begin downloading file of hash to path
func PrepareDownload(hash string, path string) error {

	// Load file by Hash
	blacklist := []string{}
	fileMeta := &utils.FileMetaData{}
	err := utils.LoadProgress(hash, fileMeta)
	if err != nil {
		return err
	}

	if fileMeta.Progress != utils.Complete {
		return downError.New("Can't download data because it was not successfully uploaded.")
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	fileMeta.Progress = utils.InProgress
	for i := 1; i <= fileMeta.TotalShards; i++ {
		fileMeta.Shards[i-1].Progress = utils.Awaiting
	}

	downloadState := utils.State{blacklist, fileMeta, file, path}

	queueDownload(&downloadState)

	if downloadState.FileMeta.Progress == utils.Awaiting {
		fmt.Println("Successfully downloaded data!")
	} else {
		fmt.Println("Upload failed.")
	}

	return nil
}
