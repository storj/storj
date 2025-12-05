// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build ignore

package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

var (
	mainpkg = flag.String("pkg", "storj.io/storj/storagenode/internalpb", "main package name")
	protoc  = flag.String("protoc", "protoc", "protoc compiler")
)

var ignoreProto = map[string]bool{
	"gogo.proto": true,
}

func ignore(files []string) []string {
	xs := []string{}
	for _, file := range files {
		if !ignoreProto[file] {
			xs = append(xs, file)
		}
	}
	return xs
}

// Programs needed for code generation:
//
// github.com/ckaznocha/protoc-gen-lint
// storj.io/drpc/cmd/protoc-gen-drpc
// github.com/nilslice/protolock/cmd/protolock

func main() {
	flag.Parse()

	// TODO: protolock

	{
		// cleanup previous files
		localfiles, err := filepath.Glob("*.pb.go")
		check(err)

		all := []string{}
		all = append(all, localfiles...)
		for _, match := range all {
			_ = os.Remove(match)
		}
	}

	{
		protofiles, err := filepath.Glob("*.proto")
		check(err)

		protofiles = ignore(protofiles)

		commonPb := os.Getenv("STORJ_COMMON_PB")
		if commonPb == "" {
			commonPb = "../../../common/pb"
		}

		overrideImports := ",Mgoogle/protobuf/timestamp.proto=storj.io/storj/storagenode/internalpb"
		args := []string{
			"--lint_out=.",
			"--gogo_out=paths=source_relative" + overrideImports + ":.",
			"--go-drpc_out=protolib=github.com/gogo/protobuf,paths=source_relative:.",
			"-I=.",
			"-I=" + commonPb,
		}
		args = append(args, protofiles...)

		// generate new code
		cmd := exec.Command(*protoc, args...)
		fmt.Println(strings.Join(cmd.Args, " "))
		out, err := cmd.CombinedOutput()
		fmt.Println(string(out))
		check(err)
	}

	{
		files, err := filepath.Glob("*.pb.go")
		check(err)
		for _, file := range files {
			process(file)
		}
	}

	{
		// format code to get rid of extra imports
		out, err := exec.Command("goimports", "-local", "storj.io", "-w", ".").CombinedOutput()
		fmt.Println(string(out))
		check(err)
	}
}

func process(file string) {
	data, err := os.ReadFile(file)
	check(err)

	source := string(data)

	// When generating code to the same path as proto, it will
	// end up generating an `import _ "."`, the following replace removes it.
	source = strings.Replace(source, `_ "."`, "", -1)

	err = os.WriteFile(file, []byte(source), 0644)
	check(err)
}

func check(err error) {
	if err != nil {
		panic(err)
	}
}
