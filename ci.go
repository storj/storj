// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build ci

package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"time"

	"github.com/loov/ci"
	. "github.com/loov/ci/dsl"
	"golang.org/x/sync/errgroup"
)

var pipelines = Pipelines(
	Pipeline("Default", "",
		Stage("Download",
			Run("go", "version"),
			WhenEnv("CI", "",
				CreateGlobalTempDir("SOURCE"),
				Copy("$SCRIPTDIR/*", "$SOURCE"),
				Run("go", "mod", "download"),
			),
			WhenEnv("CI", "travis",
				Run("go", "mod", "download"),
				SetGlobalEnv("SOURCE", "PWD"),
			),
		),
		Stage("Build",
			Run("go", "install", "-race", "./..."),
		),
		Parallel("Verification",
			Stage("Lint",
				TempGopath(
					Copy("$SOURCE/*", "$GOPATH/src/github.com/loov/cidemo"),
					CD("$GOPATH/src/github.com/loov/cidemo"),
					SetEnv("GO111MODULE", "on"),
					Run("go", "mod", "vendor"),
					Copy("./vendor/*", "$GOPATH/src"),
					Remove("./vendor"),
					SetEnv("GO111MODULE", "off"),
					Run("golangci-lint", "-j=4", "run"),
				),
			),
			Stage("Run",
				Run("go", "run", "main.go"),
			),
			Stage("Test",
				Run("go", "test", "-v", "-race", "./..."),
			),
		),
	),
)

func main() {
	flag.Parse()
	pipelineName := flag.Arg(0)
	if pipelineName == "" {
		pipelineName = "Default"
	}

	pipeline, ok := pipelines.Find(pipelineName)
	if !ok {
		fmt.Fprintf(os.Stderr, "did not find pipeline named %q", pipelineName)
		os.Exit(1)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	task := pipeline.Task()

	var group errgroup.Group
	group.Go(func() error {
		defer cancel()

		globalContext, err := ci.NewGlobalContext(".", nil)
		if err != nil {
			return err
		}

		return task.Run(&globalContext.Context)
	})
	group.Go(func() error {
		return nil
		return monitor(ctx, task)
	})

	err := group.Wait()

	printPipeline(task)

	if err != nil {
		fmt.Fprintf(os.Stderr, "run failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Fprintf(os.Stderr, "run succeeded: %v\n", err)
}

func clear() {
	switch runtime.GOOS {
	case "windows":
		cmd := exec.Command("cmd", "/c", "cls")
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		cmd.Run()
	case "linux", "darwin":
		cmd := exec.Command("clear")
		cmd.Stdout, cmd.Stderr = os.Stdout, os.Stderr
		cmd.Run()
	}
}

func monitor(ctx context.Context, pipeline *ci.Task) error {
	defer clear()
	for ctx.Err() == nil {
		clear()
		printPipeline(pipeline)
		time.Sleep(time.Second)
	}
	return nil
}

func printPipeline(pipeline *ci.Task) {
	for _, subtask := range pipeline.Tasks {
		subtask.PrintTo(os.Stdout, "")
	}
}
