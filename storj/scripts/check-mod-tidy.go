// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build ignore

package main

import (
	"bytes"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strings"

	"github.com/kylelemons/godebug/diff"
)

var modfile = flag.String("mod", "go.mod", "original mod file")

func main() {
	flag.Parse()

	tempdir, err := ioutil.TempDir("", "check-mod-tidy")
	checkf(err, "failed to create a temporary directory: %v\n", err)

	defer func() {
		err := os.RemoveAll(tempdir)
		fmt.Fprintf(os.Stderr, "failed to delete temporary directory: %v\n", err)
	}()

	err = copyDir(".", tempdir)
	checkf(err, "failed to copy directory: %v\n", err)

	workingDir, err := os.Getwd()
	checkf(err, "failed to get working directory: %v\n", err)

	err = os.Chdir(tempdir)
	checkf(err, "failed to change directory: %v\n", err)

	defer os.Chdir(workingDir)

	original, err := ioutil.ReadFile(*modfile)
	checkf(err, "failed to read %q: %v\n", *modfile, err)

	err = ioutil.WriteFile("go.mod", original, 0755)
	checkf(err, "failed to write go.mod: %v\n", err)

	err = tidy()
	checkf(err, "failed to tidy go.mod: %v\n", err)

	changed, err := ioutil.ReadFile("go.mod")
	checkf(err, "failed to read go.mod: %v\n", err)

	if !bytes.Equal(original, changed) {
		diff, removed := difflines(string(original), string(changed))
		fmt.Fprintln(os.Stderr, "go.mod is not tidy")
		fmt.Fprintln(os.Stderr, diff)
		if removed {
			os.Exit(1)
		}
	}
}

func tidy() error {
	var err error
	for repeat := 2; repeat > 0; repeat-- {
		cmd := exec.Command("go", "mod", "tidy")
		cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
		err = cmd.Run()
		if err != nil {
			fmt.Fprintf(os.Stderr, "go mod tidy failed, retrying: %v", err)
			continue
		}
		break
	}
	return err
}

func copyDir(src, dst string) error {
	cmd := exec.Command("cp", "-a", src, dst)
	cmd.Stdout, cmd.Stderr = os.Stderr, os.Stderr
	return cmd.Run()
}

func checkf(err error, format string, args ...interface{}) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, format, args...)
	os.Exit(1)
}

func difflines(a, b string) (patch string, removed bool) {
	alines, blines := strings.Split(a, "\n"), strings.Split(b, "\n")

	chunks := diff.DiffChunks(alines, blines)

	buf := new(bytes.Buffer)
	for _, c := range chunks {
		for _, line := range c.Added {
			fmt.Fprintf(buf, "+%s\n", line)
		}
		for _, line := range c.Deleted {
			fmt.Fprintf(buf, "-%s\n", line)
			removed = true
		}
	}

	return strings.TrimRight(buf.String(), "\n"), removed
}
