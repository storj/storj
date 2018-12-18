// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strings"
)

var rxModule = regexp.MustCompile(`(\S*) v\d+.\d+.\d+(.*)`)

func main() {
	flag.Parse()

	err := buildDeps()
	if err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
	os.Exit(0)
}

func buildDeps() error {
	modfile, err := ioutil.ReadFile(`go.mod`)
	if err != nil {
		panic(err)
	}

	for _, match := range rxModule.FindAllStringSubmatch(string(modfile), -1) {
		if strings.Contains(match[2], "// indirect") {
			continue
		}

		cmd := exec.Command("go", "build", match[1]+"/...")
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		_ = cmd.Run()
	}

	return nil
}
