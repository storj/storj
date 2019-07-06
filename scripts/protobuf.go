// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build ignore

package main

import (
	"bytes"
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
			"github.com/nilslice/protolock/cmd/protolock@v0.12.0",
		)
	case "generate":
		return walkdirs(root, generate)
	case "lint":
		return walkdirs(root, lint)
	case "check-lock":
		return walkdirs(root, checklock)
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

	cmd := exec.Command("protolock", "status")
	local, err := os.Getwd()
	if err != nil {
		panic(err)
	}
	cmd.Dir = findProtolockDir(local)
	out, err := cmd.CombinedOutput()
	if len(out) > 0 {
		fmt.Println(string(out))
	}
	if err != nil {
		return err
	}

	args := []string{"--gogo_out=plugins=grpc:.", "--lint_out=."}
	args = appendCommonArguments(args, dir, dirs, files)

	cmd = exec.Command(*protoc, args...)
	fmt.Println(strings.Join(cmd.Args, " "))
	out, err = cmd.CombinedOutput()
	if len(out) > 0 {
		fmt.Println(string(out))
	}
	return err
}

func appendCommonArguments(args []string, dir string, dirs []string, files []string) []string {
	var includes []string
	prependDirInclude := false
	for _, otherdir := range dirs {
		if otherdir == dir {
			// the include for the directory must be added first... see below.
			prependDirInclude = true
			continue
		}

		reldir, err := filepath.Rel(dir, otherdir)
		if err != nil {
			panic(err)
		}

		includes = append(includes, "-I="+reldir)
	}

	if prependDirInclude {
		// The include for the directory needs to be added first, otherwise the
		// .proto files it contains will be shadowed by .proto files in the
		// other directories with matching names (causing protoc to bail).
		args = append(args, "-I=.")
	}
	args = append(args, includes...)
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

func checklock(dir string, dirs []string, files []string) error {
	defer switchdir(dir)()

	local, err := os.Getwd()
	if err != nil {
		panic(err)
	}

	protolockdir := findProtolockDir(local)

	tmpdir, err := ioutil.TempDir("", "protolock")
	if err != nil {
		return err
	}
	defer func() {
		_ = os.RemoveAll(tmpdir)
	}()

	original, err := ioutil.ReadFile(filepath.Join(protolockdir, "proto.lock"))
	if err != nil {
		return fmt.Errorf("unable to read proto.lock: %v", err)
	}

	err = ioutil.WriteFile(filepath.Join(tmpdir, "proto.lock"), original, 0755)
	if err != nil {
		return fmt.Errorf("unable to read proto.lock: %v", err)
	}

	cmd := exec.Command("protolock", "commit", "-lockdir", tmpdir)
	cmd.Dir = protolockdir
	out, err := cmd.CombinedOutput()
	if len(out) > 0 {
		fmt.Println(string(out))
	}
	if err != nil {
		return err
	}

	changed, err := ioutil.ReadFile(filepath.Join(tmpdir, "proto.lock"))
	if err != nil {
		return fmt.Errorf("unable to read new proto.lock: %v", err)
	}

	if !bytes.Equal(original, changed) {
		diff, err := diff(original, changed)
		if err != nil {
			fmt.Fprintln(os.Stderr, err.Error())
		}
		return fmt.Errorf("protolock is not up to date: %v", string(diff))
	}

	return nil
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

func findProtolockDir(dir string) string {
	protolock := filepath.Join(dir, "proto.lock")
	if _, err := os.Stat(protolock); err != nil {
		return findProtolockDir(filepath.Dir(dir))
	}

	return dir
}

func diff(b1, b2 []byte) (data []byte, err error) {
	f1, err := writeTempFile("protobuf-diff", b1)
	if err != nil {
		return
	}
	defer os.Remove(f1)

	f2, err := writeTempFile("protobuf-diff", b2)
	if err != nil {
		return
	}
	defer os.Remove(f2)

	data, err = exec.Command("diff", "-u", f1, f2).CombinedOutput()
	if len(data) > 0 {
		// diff exits with a non-zero status when the files don't match.
		// Ignore that failure as long as we get output.
		err = nil
	}
	return
}

func writeTempFile(prefix string, data []byte) (string, error) {
	file, err := ioutil.TempFile("", prefix)
	if err != nil {
		return "", err
	}
	_, err = file.Write(data)
	if err1 := file.Close(); err == nil {
		err = err1
	}
	if err != nil {
		os.Remove(file.Name())
		return "", err
	}
	return file.Name(), nil
}
