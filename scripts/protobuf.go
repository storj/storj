// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build ignore

package main

import (
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

var ignoreProto = map[string]bool{
	"gogo.proto": true,
}

var protoc = flag.String("protoc", "protoc", "protoc location")

func main() {
	flag.Parse()

	root := flag.Arg(1)
	if root == "" {
		root = "."
	}

	err := run(flag.Arg(0), root)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed: %v\n", err)
		os.Exit(1)
	}
}

func run(command, root string) error {
	switch command {
	case "install":
		err := installGoBin()
		if err != nil {
			return err
		}

		gogoVersion, err := versionOf("github.com/gogo/protobuf")
		if err != nil {
			return err
		}

		return install(
			"github.com/ckaznocha/protoc-gen-lint@68a05858965b31eb872cbeb8d027507a94011acc",
			// See https://github.com/gogo/protobuf#most-speed-and-most-customization
			"github.com/gogo/protobuf/protoc-gen-gogo@"+gogoVersion,
		)
	case "generate":
		return walkdirs(root, generate)
	case "lint":
		return walkdirs(root, lint)
	default:
		return errors.New("unknown command " + command)
	}

	return nil
}

func installGoBin() error {
	// already installed?
	path, err := exec.LookPath("gobin")
	if path != "" && err == nil {
		return nil
	}

	cmd := exec.Command("go", "get", "-u", "github.com/myitcv/gobin")
	fmt.Println(strings.Join(cmd.Args, " "))
	cmd.Env = append(os.Environ(), "GO111MODULE=off")

	out, err := cmd.CombinedOutput()
	if len(out) > 0 {
		fmt.Println(string(out))
	}
	return err
}

func versionOf(dep string) (string, error) {
	moddata, err := ioutil.ReadFile("go.mod")
	if err != nil {
		return "", err
	}

	rxMatch := regexp.MustCompile(regexp.QuoteMeta(dep) + `\s+(.*)\n`)
	matches := rxMatch.FindAllStringSubmatch(string(moddata), 1)
	if len(matches) == 0 {
		return "", errors.New("go.mod missing github.com/gogo/protobuf entry")
	}

	return matches[0][1], nil
}

func install(deps ...string) error {
	cmd := exec.Command("gobin", deps...)
	fmt.Println(strings.Join(cmd.Args, " "))

	out, err := cmd.CombinedOutput()
	if len(out) > 0 {
		fmt.Println(string(out))
	}
	return err
}

func generate(dir string, dirs []string, files []string) error {
	defer switchdir(dir)()

	args := []string{"--gogo_out=plugins=grpc:.", "--lint_out=."}
	args = appendCommonArguments(args, dir, dirs, files)

	cmd := exec.Command(*protoc, args...)
	fmt.Println(strings.Join(cmd.Args, " "))
	out, err := cmd.CombinedOutput()
	if len(out) > 0 {
		fmt.Println(string(out))
	}
	return err
}

func appendCommonArguments(args []string, dir string, dirs []string, files []string) []string {
	for _, otherdir := range dirs {
		if otherdir == dir {
			args = append(args, "-I=.")
			continue
		}

		reldir, err := filepath.Rel(dir, otherdir)
		if err != nil {
			panic(err)
		}

		args = append(args, "-I="+reldir)
	}

	args = append(args, files...)

	return args
}

func lint(dir string, dirs []string, files []string) error {
	defer switchdir(dir)()

	args := []string{"--lint_out=."}
	args = appendCommonArguments(args, dir, dirs, files)

	cmd := exec.Command(*protoc, args...)
	fmt.Println(strings.Join(cmd.Args, " "))
	out, err := cmd.CombinedOutput()
	if len(out) > 0 {
		fmt.Println(string(out))
	}
	return err
}

func switchdir(to string) func() {
	local, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	if err := os.Chdir(to); err != nil {
		panic(err)
	}

	return func() {
		if err := os.Chdir(local); err != nil {
			panic(err)
		}
	}
}

func walkdirs(root string, fn func(dir string, dirs []string, files []string) error) error {
	matches, err := listProtoFiles(root)
	if err != nil {
		return err
	}

	byDir := map[string][]string{}
	for _, match := range matches {
		dir, file := filepath.Dir(match), filepath.Base(match)
		if ignoreProto[file] {
			continue
		}
		byDir[dir] = append(byDir[dir], file)
	}

	dirs := []string{}
	for dir := range byDir {
		dirs = append(dirs, dir)
	}
	sort.Strings(dirs)

	var errs []string
	for _, dir := range dirs {
		files := byDir[dir]
		sort.Strings(files)
		err := fn(dir, dirs, files)
		if err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}
	return nil
}

func listProtoFiles(root string) ([]string, error) {
	files := []string{}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			return nil
		}
		if info.IsDir() && info.Name() == ".git" {
			return filepath.SkipDir
		}
		if filepath.Ext(path) == ".proto" {
			files = append(files, path)
		}
		return nil
	})

	return files, err
}
