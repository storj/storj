// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"flag"
	"log"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"storj.io/common/sync2"
)

func newCommand(ctx context.Context, directory string, name string, args ...string) *exec.Cmd {
	target := append([]string{name}, args...)
	if target[0] != "make" && target[0] != "go" {
		target = append([]string{"go", "tool", "-modfile", "./scripts/go.mod"}, target...)
	}
	cmd := exec.CommandContext(ctx, target[0], target[1:]...)
	cmd.Dir = directory

	return cmd
}

type Checks struct {
	Modules         bool
	Copyright       bool
	Imports         bool
	PeerConstraints bool
	AtomicAlign     bool
	Monkit          bool
	Errors          bool
	Static          bool
	Monitoring      bool
	WASMSize        bool
	Protolock       bool
	CheckDowngrades bool
	CheckTX         bool
	CheckRetry      bool
	CheckZapFields  bool
	GolangCI        bool
}

func main() {
	workDir, err := os.Getwd()
	if err != nil {
		log.Fatalln("error", err)
	}
	checks := Checks{}

	parallel := flag.Int("parallel", runtime.NumCPU(), "specify the number of tasks to run concurrently")
	race := flag.Bool("race", false, "pass race to appropriate linters")
	flag.StringVar(&workDir, "work-dir", workDir, "specify the working directory")

	flag.BoolVar(&checks.Modules, "modules", checks.Modules, "check module tidiness")
	flag.BoolVar(&checks.Copyright, "copyright", checks.Copyright, "ensure copyright")
	flag.BoolVar(&checks.Imports, "imports", checks.Imports, "check import usage")
	flag.BoolVar(&checks.PeerConstraints, "peer-constraints", checks.PeerConstraints, "check peer constraints")
	flag.BoolVar(&checks.AtomicAlign, "atomic-align", checks.AtomicAlign, "ensure atomic alignment")
	flag.BoolVar(&checks.Monkit, "monkit", checks.Monkit, "check monkit usage")
	flag.BoolVar(&checks.Errors, "errs", checks.Errors, "check error usage")
	flag.BoolVar(&checks.Static, "staticcheck", checks.Static, "perform static analysis checks against the code base")
	flag.BoolVar(&checks.WASMSize, "wasm-size", checks.WASMSize, "check the wasm file size for optimal performance")
	flag.BoolVar(&checks.Protolock, "protolock", checks.Protolock, "check the status of the protolock file")
	flag.BoolVar(&checks.CheckDowngrades, "check-downgrades", checks.CheckDowngrades, "run the check-downgrades tool")
	flag.BoolVar(&checks.CheckTX, "check-tx", checks.CheckTX, "run the check-tx tool")
	flag.BoolVar(&checks.CheckRetry, "check-retry", checks.CheckRetry, "run the check-retry tool")
	flag.BoolVar(&checks.CheckZapFields, "check-zap-fields", checks.CheckZapFields, "run the check-zap-fields tool")
	flag.BoolVar(&checks.GolangCI, "golangci", checks.GolangCI, "run the golangci-lint tool")

	flag.Parse()

	target := []string{"./..."}
	if args := flag.Args(); len(args) > 0 {
		target = args
	}

	ctx, halt := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer halt()

	submit := func(limiter *sync2.Limiter, cmd *exec.Cmd, done func()) bool {
		prefix := "[" + cmd.Dir + " " + strings.Join(cmd.Args, " ") + "]"

		return limiter.Go(ctx, func() {
			start := time.Now()

			log.Println(prefix, "running")
			defer func() {
				log.Println(prefix, "done", time.Since(start))
				if done != nil {
					done()
				}
			}()

			out, _ := cmd.CombinedOutput()
			exitCode := cmd.ProcessState.ExitCode()
			if exitCode > 0 {
				log.Fatalln(prefix, "error", string(out)) //nolint:gocritic, it's fine to early stop
			}
		})
	}

	// Most linters load packages with export data, which means compiling all packages (with tests). When
	// they run at the same time with a cold build cache, each of them compiles the same packages. To avoid
	// that, we compile all packages once and start those linters after it. Commands that only read
	// source files run in the meantime.
	var sourceOnly, compiled []*exec.Cmd

	if checks.Modules {
		sourceOnly = append(sourceOnly, newCommand(ctx, workDir, "check-mod-tidy"))
	}

	if checks.Copyright {
		sourceOnly = append(sourceOnly, newCommand(ctx, workDir, "check-copyright"))
	}

	if checks.Imports {
		args := make([]string, 0, 2)
		if *race {
			args = append(args, "-race")
		}

		args = append(args, target...)
		sourceOnly = append(sourceOnly, newCommand(ctx, workDir, "check-imports", args...))
	}

	if checks.PeerConstraints {
		args := make([]string, 0, 1)
		if *race {
			args = append(args, "-race")
		}

		sourceOnly = append(sourceOnly, newCommand(ctx, workDir, "check-peer-constraints", args...))
	}

	if checks.AtomicAlign {
		compiled = append(compiled, newCommand(ctx, workDir, "check-atomic-align", target...))
	}

	if checks.Monkit {
		compiled = append(compiled, newCommand(ctx, workDir, "check-monkit", target...))
	}

	if checks.Errors {
		compiled = append(compiled, newCommand(ctx, workDir, "check-errs", target...))
	}

	if checks.Static {
		compiled = append(compiled, newCommand(ctx, workDir, "staticcheck", target...))
	}

	if checks.WASMSize {
		sourceOnly = append(sourceOnly, newCommand(ctx, workDir, "make", "test-wasm-size"))
	}

	if checks.Protolock {
		sourceOnly = append(sourceOnly, newCommand(ctx, workDir, "protolock", "status"))
	}

	if checks.CheckDowngrades {
		sourceOnly = append(sourceOnly, newCommand(ctx, workDir, "check-downgrades", target...))
	}

	if checks.CheckTX {
		sourceOnly = append(sourceOnly, newCommand(ctx, workDir, "check-tx", target...))
	}

	if checks.CheckRetry {
		compiled = append(compiled, newCommand(ctx, workDir, "check-retry", target...))
	}

	if checks.CheckZapFields {
		compiled = append(compiled, newCommand(ctx, workDir, "check-zap-fields", target...))
	}

	if checks.GolangCI {
		args := append([]string{"--config", ".golangci.yml", "--skip-dirs", "(^|/)node_modules($|/)", "run"}, target...)
		// golangci-lint takes the longest, so start it first.
		compiled = append([]*exec.Cmd{newCommand(ctx, workDir, "golangci-lint", args...)}, compiled...)
	}

	start := time.Now()
	defer func() {
		log.Println("total time", time.Since(start))
	}()

	limiter := sync2.NewLimiter(*parallel)
	mustSubmit := func(cmd *exec.Cmd, done func()) {
		if !submit(limiter, cmd, done) {
			log.Fatalln("error", "failed to submit task to queue")
		}
	}

	// first compile all packages, then run the source only commands and build the linters in the meantime.
	var prepared sync.WaitGroup
	if len(compiled) > 0 {
		// use the same flags as golang.org/x/tools/go/packages, so the build cache is reused.
		args := append([]string{"list", "-e", "-compiled=true", "-test=true", "-export=true", "-deps=true",
			"-find=false", "-pgo=off", "-f", "{{.ImportPath}}"}, target...)
		prepared.Add(1)
		mustSubmit(newCommand(ctx, workDir, "go", args...), prepared.Done)
	}

	for _, cmd := range sourceOnly {
		mustSubmit(cmd, nil)
	}

	for _, cmd := range compiled {
		// "go tool -n" only builds the tool and prints its path.
		prepared.Add(1)
		mustSubmit(newCommand(ctx, workDir, "go", "tool", "-modfile", "./scripts/go.mod", "-n", cmd.Args[4]), prepared.Done)
	}

	prepared.Wait()
	for _, cmd := range compiled {
		mustSubmit(cmd, nil)
	}

	limiter.Wait()
}
