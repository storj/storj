// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

// +build ignore

package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

var ignoreProto = map[string]bool{
	"gogo.proto": true,
}

func main() {
	flag.Parse()

	root := flag.Arg(1)
	if root == "" {
		root = "."
	}

	var err error

	switch flag.Arg(0) {
	case "install":
		// TODO
	case "generate":
		err = walkdirs(root, generate)
	case "lint":
		err = walkdirs(root, lint)
	default:
		fmt.Fprintf(os.Stderr, "unknown command %q\n", flag.Arg(0))
		os.Exit(1)
	}

	if err != nil {
		fmt.Fprintf(os.Stderr, "failure: %v\n", err)
		os.Exit(1)
	}
}

func generate(dir string, dirs []string, files []string) error {
	local, err := os.Getwd()
	if err != nil {
		return err
	}
	defer func() {
		err := os.Chdir(local)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
		}
	}()

	if err := os.Chdir(dir); err != nil {
		return err
	}

	args := []string{"--gogo_out=plugins=grpc:."}

	for _, otherdir := range dirs {
		if otherdir == dir {
			args = append(args, "-I=.")
			continue
		}

		reldir, err := filepath.Rel(dir, otherdir)
		if err != nil {
			return err
		}

		args = append(args, "-I="+reldir)
	}

	args = append(args, files...)

	fmt.Println("protoc", args)
	cmd := exec.Command("protoc", args...)

	out, err := cmd.CombinedOutput()
	fmt.Println(string(out))
	return err
}

func lint(dir string, dirs []string, files []string) error {
	fmt.Println("linting", dir, files)
	return nil
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
		err := fn(dir, dirs, byDir[dir])
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
