//  Copyright (C) 2021 Storj Labs, Inc.
//  See LICENSE for copying information.

package endpoints

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
)

type testfile struct {
	name     string
	contents []byte
}

func newtestfile(filename string, data []byte) *testfile {
	f := testfile{name: filename, contents: data} // instantiate struct testfile
	writefile(&f)                                 // write the content to a file by passing the testfile pointer
	return &f                                     // return pointer
}

func writefile(t *testfile) {
	err := ioutil.WriteFile(t.name, t.contents, 0644) // write the structs properties to the file and set permissions
	check(err)
}

func openfile(fname string) *testfile {
	fhand, err := os.Open(fname) // file handler or err
	check(err)
	defer closeFile(fhand)

	fscan := bufio.NewScanner(fhand) // scan file and place lines in buffer
	var lines []byte
	for fscan.Scan() {
		lines = append(lines, fscan.Bytes()...) // walk the buffer and create []byte
	}
	f := testfile{name: fname, contents: lines} // instantiate struct testfile and set the properties
	check(err)
	return &f // return pointer to testfile
}

func checkfile(fn string) bool { // check that the file exists and is not  directory
	status, err := os.Stat(fn) // file info will give us an exists status
	// check(err)

	if os.IsNotExist(err) { // return false when the file doesn't exist
		return false
	}
	return !status.IsDir() // fall thru and return true for not a directory
}

func closeFile(f *os.File) {
	fmt.Println("closing")
	err := f.Close()
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}
}

func deletefile(fn string) {
	err := os.Remove(fn)
	check(err)
}
