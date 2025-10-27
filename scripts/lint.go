// Copyright (C) 2022 Storj Labs, Inc.
// See LICENSE for copying information.

//go:build ignore

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
	"syscall"
	"time"

	"storj.io/common/sync2"
)

func newCommand(ctx context.Context, directory string, name string, args ...string) *exec.Cmd {
	target := append([]string{name}, args...)
	if target[0] != "make" {
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
	flag.BoolVar(&checks.CheckTX, "check-tx", checks.CheckDowngrades, "run the check-tx tool")
	flag.BoolVar(&checks.GolangCI, "golangci", checks.GolangCI, "run the golangci-lint tool")

	flag.Parse()

	target := []string{"./..."}
	if args := flag.Args(); len(args) > 0 {
		target = args
	}

	ctx, halt := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer halt()

	submit := func(limiter *sync2.Limiter, cmd *exec.Cmd) bool {
		prefix := "[" + cmd.Dir + " " + strings.Join(cmd.Args, " ") + "]"

		return limiter.Go(ctx, func() {
			start := time.Now()

			log.Println(prefix, "running")
			defer func() {
				log.Println(prefix, "done", time.Since(start))
			}()

			out, _ := cmd.CombinedOutput()
			exitCode := cmd.ProcessState.ExitCode()
			if exitCode > 0 {
				log.Fatalln(prefix, "error", string(out))
			}
		})
	}

	// separate commands into two tiers to handle commands that can not be run in parallel (like staticcheck and
	// golangci-lint).
	commands := [][]*exec.Cmd{
		make([]*exec.Cmd, 0, 10),
		make([]*exec.Cmd, 0, 1),
	}

	if checks.Modules {
		commands[0] = append(commands[0], newCommand(ctx, workDir, "check-mod-tidy"))
	}

	if checks.Copyright {
		commands[0] = append(commands[0], newCommand(ctx, workDir, "check-copyright"))
	}

	if checks.Imports {
		args := make([]string, 0, 2)
		if *race {
			args = append(args, "-race")
		}

		args = append(args, target...)
		commands[0] = append(commands[0], newCommand(ctx, workDir, "check-imports", args...))
	}

	if checks.PeerConstraints {
		args := make([]string, 0, 1)
		if *race {
			args = append(args, "-race")
		}

		commands[0] = append(commands[0], newCommand(ctx, workDir, "check-peer-constraints", args...))
	}

	if checks.AtomicAlign {
		commands[0] = append(commands[0], newCommand(ctx, workDir, "check-atomic-align", target...))
	}

	if checks.Monkit {
		commands[0] = append(commands[0], newCommand(ctx, workDir, "check-monkit", target...))
	}

	if checks.Errors {
		commands[0] = append(commands[0], newCommand(ctx, workDir, "check-errs", target...))
	}

	if checks.Static {
		commands[0] = append(commands[0], newCommand(ctx, workDir, "staticcheck", target...))
	}

	if checks.WASMSize {
		commands[0] = append(commands[0], newCommand(ctx, workDir, "make", "test-wasm-size"))
	}

	if checks.Protolock {
		commands[0] = append(commands[0], newCommand(ctx, workDir, "protolock", "status"))
	}

	if checks.CheckDowngrades {
		commands[0] = append(commands[0], newCommand(ctx, workDir, "check-downgrades", target...))
	}

	if checks.CheckTX {
		commands[0] = append(commands[0], newCommand(ctx, workDir, "check-tx", target...))
	}

	if checks.GolangCI {
		args := append([]string{"--config", ".golangci.yml", "--skip-dirs", "(^|/)node_modules($|/)", "-j=2", "run"}, target...)
		commands[1] = append(commands[1], newCommand(ctx, workDir, "golangci-lint", args...))
	}

	start := time.Now()
	defer func() {
		log.Println("total time", time.Since(start))
	}()

	for _, tier := range commands {
		limiter := sync2.NewLimiter(*parallel)
		for _, cmd := range tier {
			ok := submit(limiter, cmd)
			if !ok {
				log.Fatalln("error", "failed to submit task to queue")
			}
		}

		limiter.Wait()
	}

}
