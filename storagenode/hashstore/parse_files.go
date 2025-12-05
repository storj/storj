// Copyright (C) 2024 Storj Labs, Inc.
// See LICENSE for copying information.

package hashstore

import (
	"errors"
	"fmt"
	"io/fs"
	"iter"
	"path/filepath"
	"strconv"
	"strings"
)

var errSentinel = errors.New("sentinel")

type parsedFile[T any] struct {
	path string
	name string
	data T
}

func parseFiles[T any](parse func(string) (T, bool), dirs ...string) iter.Seq2[parsedFile[T], error] {
	return func(yield func(parsedFile[T], error) bool) {
		for _, dir := range dirs {
			if err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
				if err != nil {
					return err
				} else if d.IsDir() {
					return nil
				}

				name := filepath.Base(path)

				data, ok := parse(name)
				if !ok {
					return nil
				}

				if !yield(parsedFile[T]{path: path, name: name, data: data}, nil) {
					return errSentinel
				}

				return nil
			}); err != nil {
				if !errors.Is(err, errSentinel) {
					yield(parsedFile[T]{}, err)
				}
				return
			}
		}
	}
}

//
// log files
//

func createLogName(id uint64, ttl uint32) string {
	return fmt.Sprintf("log-%016x-%08x", id, ttl)
}

type logInfo struct {
	id  uint64
	ttl uint32
}

func parseLog(name string) (logInfo, bool) {
	// skip any files that don't look like log files. log file names are either
	//     log-<16 bytes of id>
	//     log-<16 bytes of id>-<8 bytes of ttl>
	// so they always begin with "log-" and are either 20 or 29 bytes long.
	if (len(name) != 20 && len(name) != 29) || name[0:4] != "log-" {
		return logInfo{}, false
	}

	id, err := strconv.ParseUint(name[4:20], 16, 64)
	if err != nil {
		return logInfo{}, false
	}

	var ttl uint32
	if len(name) == 29 && name[20] == '-' {
		ttl64, err := strconv.ParseUint(name[21:29], 16, 32)
		if err != nil {
			return logInfo{}, false
		}
		ttl = uint32(ttl64)
	}

	return logInfo{id: id, ttl: ttl}, true
}

//
// hashtbl files
//

func createHashtblName(id uint64) string {
	return fmt.Sprintf("hashtbl-%016x", id)
}

func parseHashtbl(name string) (uint64, bool) {
	// skip any files that don't look like hashtbl files. hashtbl file names are always
	//     hashtbl-<16 bytes of id>
	// so they always begin with "hashtbl-" and are 24 bytes long.
	if len(name) != 24 || name[0:8] != "hashtbl-" {
		return 0, false
	}

	id, err := strconv.ParseUint(name[8:24], 16, 64)
	if err != nil {
		return 0, false
	}

	return id, true
}

//
// hint files
//

func createHintName(id uint64) string {
	return fmt.Sprintf("hint-%016x", id)
}

func parseHint(name string) (uint64, bool) {
	// skip any files that don't look like hint files. hint file names are always
	//     hint-<16 bytes of id>
	// so they always begin with "hint-" and are 21 bytes long.
	if len(name) != 21 || name[0:5] != "hint-" {
		return 0, false
	}

	id, err := strconv.ParseUint(name[5:21], 16, 64)
	if err != nil {
		return 0, false
	}

	return id, true
}

//
// temp files
//

func createTempName(base string) string {
	return base + ".tmp"
}

func parseTemp(name string) (string, bool) {
	if !strings.HasSuffix(name, ".tmp") {
		return "", false
	}
	return strings.TrimSuffix(name, ".tmp"), true
}
