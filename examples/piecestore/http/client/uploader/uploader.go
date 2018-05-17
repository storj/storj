// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package uploader

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"

	"github.com/aleitner/FilePiece"

	"storj.io/storj/examples/piecestore/http/client/utils"
)

// Prepare an http Request for posting shard to a farmer
func prepareDataUploadReq(uri string, params map[string]string, offset int64, shardSize int64, path string, file *os.File) (*http.Request, error) {
	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	part, err := writer.CreateFormFile("uploadfile", filepath.Base(path))
	if err != nil {
		return nil, err
	}

	// Created a section reader so that we can concurrently retrieve the same file.
	chunk := fpiece.NewChunk(file, offset, shardSize)
	_, err = io.Copy(part, chunk)

	for key, val := range params {
		_ = writer.WriteField(key, val)
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("POST", uri, body)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	return req, err
}

// Get available farmers from Network
func getAvailableFarmers(exclude []string) []string {
	farmers := []string{
		"http://127.0.0.1:8080", "http://127.0.0.1:8081", "http://127.0.0.1:8082",
		"http://127.0.0.1:8083", "http://127.0.0.1:8084", "http://127.0.0.1:8085",
		"http://127.0.0.1:8086", "http://127.0.0.1:8087", "http://127.0.0.1:8088",
		"http://127.0.0.1:8089",
	}

	// Find and remove "two"
	for i, farmer := range farmers {
		if utils.StringInSlice(farmer, exclude) {
			farmers = append(farmers[:i], farmers[i+1:]...)
			break
		}
	}

	for i := range farmers {
		j := rand.Intn(i + 1)
		farmers[i], farmers[j] = farmers[j], farmers[i]
	}

	return farmers
}

// Determine total shards, and their sizes
func determineShardCount(size int64) (int, int64, int64) {
	var minSize int64 = 1048576
	shardCount := size / minSize
	remainder := size % minSize
	if remainder > 0 {
		shardCount++
	}

	return int(shardCount), minSize, remainder
}

// Prepare shard and upload it
func uploadShard(i int, uploadState *utils.State) error {
	fileMeta := uploadState.FileMeta
	shard := &fileMeta.Shards[i-1]
	shard.Progress = utils.InProgress

	if i == fileMeta.TotalShards && fileMeta.TailShardSize > 0 {
		shard.Size = fileMeta.TailShardSize
	} else {
		shard.Size = fileMeta.AvgShardSize
	}

	farmers := getAvailableFarmers(uploadState.Blacklist)
	if len(farmers) <= 0 {
		shard.Progress = utils.Failed
		return errors.New("Not enough farmers")
	}
	farmer := farmers[0]

	// Shard data start position
	shard.Offset = fileMeta.AvgShardSize * int64(i-1)

	hash, err := utils.DetermineHash(uploadState.File, shard.Offset, shard.Size)
	if err != nil {
		return err
	}
	shard.Hash = hash

	extraParams := map[string]string{
		"offset": "0", // This offset is not the shard data start position but rather where in the store file the data will be placed.
		"hash":   shard.Hash,
		"size":   strconv.FormatInt(shard.Size, 10),
	}

	request, err := prepareDataUploadReq(farmer+"/upload", extraParams, shard.Offset, shard.Size, uploadState.FilePath, uploadState.File)
	if err != nil {
		return err
	}

	client := &http.Client{}
	resp, err := client.Do(request)
	if err != nil {
		return err
	}

	body := &bytes.Buffer{}
	_, err = body.ReadFrom(resp.Body)
	if err != nil {
		return err
	}

	resp.Body.Close()

	if resp.StatusCode == 200 {
		shard.Locations = append(shard.Locations, farmer)
		shard.Progress = utils.Complete
		fmt.Printf("Successfully uploaded shard (%v) to farmer (%s)\n", i, farmer)
	} else {
		uploadState.Blacklist = append(uploadState.Blacklist, farmer)
		shard.Progress = utils.Awaiting
		fmt.Printf("Failed to upload shard (%v) to farmer (%s)\n", i, farmer)
	}

	go queueUpload(uploadState)
	return nil
}

// work queue
func queueUpload(uploadState *utils.State) {
	fileMeta := uploadState.FileMeta

	for i := 1; i <= fileMeta.TotalShards; i++ {
		if fileMeta.Shards[i-1].Progress == utils.Awaiting {
			go uploadShard(i, uploadState)
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

// PrepareUpload -- Begin uploading data from dataPath
func PrepareUpload(dataPath string) error {
	file, err := os.Open(dataPath)
	if err != nil {
		return err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}

	hash, err := utils.DetermineHash(file, 0, fileInfo.Size())
	if err != nil {
		return err
	}

	shardCount, avgShardSize, tailShardSize := determineShardCount(fileInfo.Size())

	// TODO: Check if upload has already begun by hash and load the state

	// create the state
	blacklist := []string{}
	fileMeta := utils.FileMetaData{
		Size: fileInfo.Size(),
		Hash: hash,
		TotalShards: shardCount,
		AvgShardSize: avgShardSize,
		TailShardSize: tailShardSize,
		Shards: []utils.Shard{},
		Progress: utils.InProgress,
	}

	for i := 1; i <= shardCount; i++ {
		fileMeta.Shards = append(fileMeta.Shards, utils.Shard{N: i, Hash: "", Offset: 0, Size: 0, Locations: []string{}, Progress: utils.Awaiting})
	}

	uploadState := utils.State{Blacklist: blacklist, FileMeta: &fileMeta, File: file, FilePath: dataPath}

	queueUpload(&uploadState)

	if uploadState.FileMeta.Progress == utils.Complete {
		fmt.Println("Successfully uploaded data!")
	} else {
		fmt.Println("Upload failed.")
	}

	err = utils.SaveProgress(*uploadState.FileMeta)
	if err != nil {
		return err
	}

	return nil
}
