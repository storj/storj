package main

import (
	"flag"
	"fmt"
	"log"
	"sort"

	"gopkg.in/src-d/go-git.v4"
	"gopkg.in/src-d/go-git.v4/plumbing"
	"gopkg.in/src-d/go-git.v4/plumbing/object"
)

const DontCareSize = 0 << 10 // 15kb

func main() {
	flag.Parse()

	target := flag.Arg(0)
	if target == "" {
		log.Fatal("target not specified")
	}

	repo, err := git.PlainOpen(target)
	if err != nil {
		log.Fatal(err)
	}

	blobs, err := repo.BlobObjects()
	if err != nil {
		log.Fatal(err)
	}

	infos := make([]BlobInfo, 0, 100<<10)

	err = blobs.ForEach(func(blob *object.Blob) error {
		if blob.Size < DontCareSize {
			return nil
		}

		infos = append(infos, BlobInfo{
			Hash: blob.Hash,
			Size: blob.Size,
		})
		return nil
	})
	if err != nil {
		log.Fatal(err)
	}

	sort.Slice(infos, func(i, k int) bool {
		return infos[i].Size > infos[k].Size
	})

	fmt.Println("total count", len(infos))
	// Examine(30, infos)

	var accumulated int64
	for _, info := range infos {
		accumulated += info.Size
		fmt.Println(info.Hash, accumulated, info.Size)
	}
}

type BlobInfo struct {
	Hash plumbing.Hash
	Size int64
}

func Examine(n int, infos []BlobInfo) {
	if n < len(infos) {
		infos = infos[:n]
	}
	for _, info := range infos {
		fmt.Printf("%v\n", info)
	}
}
