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
			Run("go", "mod", "download"),
			SetGlobalEnv("SOURCE", "$SCRIPTDIR"),
		),
		Stage("Build",
			Run("go", "get", "-mod=readonly", "github.com/mattn/goveralls"),
			Run("go", "install", "-a", "-race", "./..."),
			Run("make", "install-sim"),
		),

		Parallel("Verification",
			Stage("Test bootstrap", Run("go", "test", "-vet=off", "-race", "-cover", "-coverprofile=.coverprofile", "-timeout=9m", "./bootstrap/..."),
			Stage("Test cmd", Run("go", "test", "-vet=off", "-race", "-cover", "-coverprofile=.coverprofile", "-timeout=9m", "./cmd/..."),
			Stage("Test internal", Run("go", "test", "-vet=off", "-race", "-cover", "-coverprofile=.coverprofile", "-timeout=9m", "./internal/..."),
			Stage("Test lib", Run("go", "test", "-vet=off", "-race", "-cover", "-coverprofile=.coverprofile", "-timeout=9m", "./lib/..."),
			Stage("Test pkg", Run("go", "test", "-vet=off", "-race", "-cover", "-coverprofile=.coverprofile", "-timeout=9m", "./pkg/..."),
			Stage("Test satellite", Run("go", "test", "-vet=off", "-race", "-cover", "-coverprofile=.coverprofile", "-timeout=9m", "./satellite/..."),
			Stage("Test scripts", Run("go", "test", "-vet=off", "-race", "-cover", "-coverprofile=.coverprofile", "-timeout=9m", "./scripts/..."),
			Stage("Test storage", Run("go", "test", "-vet=off", "-race", "-cover", "-coverprofile=.coverprofile", "-timeout=9m", "./storage/..."),
			Stage("Test storagenode", Run("go", "test", "-vet=off", "-race", "-cover", "-coverprofile=.coverprofile", "-timeout=9m", "./storagenode/..."),
			Stage("Test uplink", Run("go", "test", "-vet=off", "-race", "-cover", "-coverprofile=.coverprofile", "-timeout=9m", "./uplink/..."),
			Stage("Test versioncontrol", Run("go", "test", "-vet=off", "-race", "-cover", "-coverprofile=.coverprofile", "-timeout=9m", "./versioncontrol/..."),
			Stage("Test web", Run("go", "test", "-vet=off", "-race", "-cover", "-coverprofile=.coverprofile", "-timeout=9m", "./web/..."),

			Stage("Lint",
				TempGopath(
					Copy("$SOURCE/*", "$GOPATH/src/storj.io/storj"),
					CD("$GOPATH/src/storj.io/storj"),
					SetEnv("GO111MODULE", "on"),
					Run("go", "mod", "vendor"),
					Copy("./vendor/*", "$GOPATH/src"),
					Remove("./vendor"),
					SetEnv("GO111MODULE", "off"),
					Run("golangci-lint", "-j=4", "run"),
				),
			),
			Stage("Integration",
				Run("bash", "scripts/test-sim.sh"),
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
