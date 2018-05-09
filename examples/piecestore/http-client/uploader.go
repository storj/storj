package main

import (
	"bytes"
	"fmt"
	"io"
	"math/rand"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
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
	dataFileSection := io.NewSectionReader(file, offset, shardSize)
	_, err = io.Copy(part, dataFileSection)

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
		if stringInSlice(farmer, exclude) {
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
		shardCount += 1
	}

	return int(shardCount), minSize, remainder
}

// Prepare shard and upload it
func uploadShard(i int, uploadState *state) error {
	fileMeta := uploadState.fileMeta
	shard := &fileMeta.Shards[i-1]
	shard.Progress = In_Progress

	if i == fileMeta.TotalShards && fileMeta.TailShardSize > 0 {
		shard.Size = fileMeta.TailShardSize
	} else {
		shard.Size = fileMeta.AvgShardSize
	}

	farmers := getAvailableFarmers(uploadState.blacklist)
	farmer := farmers[0]

	// Shard data start position
	shard.Offset = fileMeta.AvgShardSize * int64(i-1)

	hash, err := determineHash(uploadState.file, shard.Offset, shard.Size)
	if err != nil {
		return err
	}
	shard.Hash = hash

	extraParams := map[string]string{
		"offset": "0", // This offset is not the shard data start position but rather where in the store file the data will be placed.
		"hash":   shard.Hash,
		"size":   strconv.FormatInt(shard.Size, 10),
	}

	request, err := prepareDataUploadReq(farmer+"/upload", extraParams, shard.Offset, shard.Size, uploadState.filePath, uploadState.file)
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
		shard.Progress = Complete
		fmt.Printf("Successfully uploaded shard (%v) to farmer (%s)\n", i, farmer)
		return nil
	} else {
		uploadState.blacklist = append(uploadState.blacklist, farmer)
		shard.Progress = Awaiting
		fmt.Printf("Failed to upload shard (%v) to farmer (%s)\n", i, farmer)
		return nil
	}
}

// work queue
func queue_upload(uploadState *state) {
	fileMeta := uploadState.fileMeta

	for i := 1; i <= fileMeta.TotalShards; i++ {
		if fileMeta.Shards[i-1].Progress == Awaiting {
			// TODO Separate into go subroutines
			uploadShard(i, uploadState)
		}
	}

	completed := 0
	for i := 1; i <= fileMeta.TotalShards; i++ {
		if fileMeta.Shards[i-1].Progress == Complete {
			completed += 1
		}
	}

	if completed == fileMeta.TotalShards {
		fileMeta.Progress = Complete
	}
}

// Prepare the upload meta data
func prepareUpload(dataPath string) error {
	file, err := os.Open(dataPath)
	if err != nil {
		return err
	}
	defer file.Close()

	fileInfo, err := file.Stat()
	if err != nil {
		return err
	}

	hash, err := determineHash(file, 0, fileInfo.Size())
	if err != nil {
		return err
	}

	shardCount, avgShardSize, tailShardSize := determineShardCount(fileInfo.Size())

	// TODO: Check if upload has already begun by hash and load the state

	// create the state
	blacklist := []string{}
	fileMeta := fileMetaData{fileInfo.Size(), hash, shardCount, avgShardSize, tailShardSize, []shard{}, In_Progress}
	for i := 1; i <= shardCount; i++ {
		fileMeta.Shards = append(fileMeta.Shards, shard{i, "", 0, 0, []string{}, Awaiting})
	}

	uploadState := state{blacklist, &fileMeta, file, dataPath}

	queue_upload(&uploadState)

	if uploadState.fileMeta.Progress == Complete {
		fmt.Println("Successfully uploaded data!")
	} else {
		fmt.Println("Upload failed.")
	}

	err = saveProgress(*uploadState.fileMeta)
	if err != nil {
		return err
	}

	return nil
}
