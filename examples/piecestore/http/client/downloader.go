package main

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/zeebo/errs"
)

var downError = errs.Class("downloadError")

// Prepare shard and download it
func downloadShard(i int, downloadState *state) error {
	fileMeta := downloadState.fileMeta
	shard := &fileMeta.Shards[i-1]
	shard.Progress = In_Progress

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
	downloadState.file.Seek(shard.Offset, 0)
	_, err = io.Copy(downloadState.file, resp.Body)
	if err != nil {
		return err
	}

	// Write the body to file
	// var totalWritten int64 = 0
	// buffer := make([]byte, 4096)
	// for totalWritten < shard.Size {
	// 	// Read data from read stream into buffer
	// 	n, err := resp.Body.Read(buffer)
	// 	if err == io.EOF {
	// 		break
	// 	}
	//
	// 	// Write the buffer to the file we opened earlier
	// 	writtenBytes, err := downloadState.file.WriteAt(buffer[:n], totalWritten + shard.Offset)
	//   totalWritten += int64(writtenBytes)
	// }
	//
	// fmt.Println("Total Written ", totalWritten)

	if err != nil {
		return err
	}

	if resp.StatusCode == 200 {
		shard.Progress = Complete
		fmt.Printf("Successfully downloaded shard (%v) from farmer (%s)\n", i, shard.Locations[0])
		return nil
	} else {
		shard.Progress = Awaiting
		fmt.Printf("Failed to download shard (%v) from farmer (%s)\n", i, shard.Locations[0])
		return nil
	}

}

// work queue
func queue_download(downloadState *state) {
	fileMeta := downloadState.fileMeta

	for i := 1; i <= fileMeta.TotalShards; i++ {
		if fileMeta.Shards[i-1].Progress == Awaiting {
			// TODO Separate into go subroutines
			downloadShard(i, downloadState)
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

func prepareDownload(hash string, path string) error {

	// Load file by Hash
	blacklist := []string{}
	fileMeta := &fileMetaData{}
	err := loadProgress(hash, fileMeta)
	if err != nil {
		return err
	}

	if fileMeta.Progress != Complete {
		return downError.New("Can't download data because it was not successfully uploaded.")
	}

	file, err := os.Create(path)
	if err != nil {
		return err
	}
	defer file.Close()

	fileMeta.Progress = In_Progress
	for i := 1; i <= fileMeta.TotalShards; i++ {
		fileMeta.Shards[i-1].Progress = Awaiting
	}

	downloadState := state{blacklist, fileMeta, file, path}

	queue_download(&downloadState)

	if downloadState.fileMeta.Progress == Complete {
		fmt.Println("Successfully downloaded data!")
	} else {
		fmt.Println("Upload failed.")
	}

	return nil
}
