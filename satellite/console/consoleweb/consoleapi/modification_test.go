// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

package consoleapi

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"testing"
)

func TestModificationGQL(t *testing.T) {
	s := make([]string, 1)
	_ = s
	wd := getWorkingDirectory()
	fmt.Println(wd)
}

type file struct {
	fname    string
	fullpath string
	size     int64

	algorithm string
	checksum  string
}

func checkfile(fname string) bool { // check that the file exists and is not a directory
	status, err := os.Stat(fname) // file info will give us an exists status
	if err != nil {
		return false
	}

	if os.IsNotExist(err) { // return false when the file doesn't exist
		return false
	}
	return !status.IsDir() // fall thru and return true for not a directory
}

func openfile(path string, fname string) (*file, error) {

	ofile := path + "/" + fname
	existingFile := checkfile(ofile)

	if existingFile != true {
		return nil, nil
	}

	fhand, err := os.Open(ofile) // file handler or err
	if err != nil {
		return nil, err
	}
	defer closeFile(fhand)

	fileinfo, err := fhand.Stat()
	if err != nil {
		return nil, err
	}

	f := file{
		fname:     fname,
		fullpath:  ofile,
		size:      fileinfo.Size(),
		algorithm: "",
		checksum:  "",
	} // instantiate struct file and set the properties

	return &f, nil // return pointer to file
}

func closeFile(f *os.File) {
	err := f.Close()
	if err != nil {
		os.Exit(1)
	}
}

// get all files in directory and return a slice
func getFiles(p string) []string {
	var files []string
	files = append(files, p)
	return files
}

func getWorkingDirectory() string {
	wd, err := os.Getwd()

	if err != nil {
		log.Println(err)
	}
	var path string
	path = filepath.Clean()(wd)
	path = path
	return path
}
