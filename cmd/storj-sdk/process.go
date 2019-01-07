// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/processgroup"
	"storj.io/storj/internal/sync2"
	"storj.io/storj/pkg/utils"
)

// Processes contains list of processes
type Processes struct {
	Output *PrefixWriter
	List   []*Process
}

// NewProcesses returns a group of processes
func NewProcesses() *Processes {
	return &Processes{
		Output: NewPrefixWriter("sdk", os.Stdout),
		List:   nil,
	}
}

// Exec executes a command on all processes
func (processes *Processes) Exec(ctx context.Context, command string) error {
	var group errgroup.Group
	processes.Start(ctx, &group, command)
	return group.Wait()
}

// Start executes all processes using specified errgroup.Group
func (processes *Processes) Start(ctx context.Context, group *errgroup.Group, command string) {
	for _, p := range processes.List {
		process := p
		group.Go(func() error {
			return process.Exec(ctx, command)
		})
	}
}

// Env returns environment flags for other nodes
func (processes *Processes) Env() []string {
	var env []string
	for _, process := range processes.List {
		env = append(env, process.Info.Env()...)
	}
	return env
}

// Close closes all the processes and their resources
func (processes *Processes) Close() error {
	var errs []error
	for _, process := range processes.List {
		err := process.Close()
		if err != nil {
			errs = append(errs, err)
		}
	}
	return utils.CombineErrors(errs...)
}

// Info represents public information about the process
type Info struct {
	Name       string
	Executable string
	Address    string
	Directory  string
	ID         string
	Extra      []string
}

// Env returns process flags
func (info *Info) Env() []string {
	name := strings.ToUpper(info.Name)

	name = strings.Map(func(r rune) rune {
		switch {
		case '0' <= r && r <= '9':
			return r
		case 'a' <= r && r <= 'z':
			return r
		case 'A' <= r && r <= 'Z':
			return r
		default:
			return '_'
		}
	}, name)

	var env []string
	if info.ID != "" {
		env = append(env, name+"_ID="+info.ID)
	}
	if info.Address != "" {
		env = append(env, name+"_ADDR="+info.Address)
	}
	if info.Directory != "" {
		env = append(env, name+"_DIR="+info.Directory)
	}
	for _, extra := range info.Extra {
		env = append(env, name+"_"+extra)
	}
	return env
}

// Arguments contains arguments based on the main command
type Arguments map[string][]string

// Process is a type for monitoring the process
type Process struct {
	processes *Processes

	Info

	Delay  time.Duration
	Wait   []*sync2.Fence
	Status struct {
		Started sync2.Fence
		Exited  sync2.Fence
	}

	Arguments Arguments

	stdout io.Writer
	stderr io.Writer

	outfile *os.File
	errfile *os.File
}

// New creates a process which can be run in the specified directory
func (processes *Processes) New(info Info) (*Process, error) {
	outfile, err1 := os.OpenFile(filepath.Join(info.Directory, "stderr.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	errfile, err2 := os.OpenFile(filepath.Join(info.Directory, "stdout.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)

	err := utils.CombineErrors(err1, err2)
	if err != nil {
		return nil, err
	}

	output := processes.Output.Prefixed(info.Name)

	process := &Process{
		processes: processes,

		Info:      info,
		Arguments: Arguments{},

		stdout: io.MultiWriter(output, outfile),
		stderr: io.MultiWriter(output, errfile),

		outfile: outfile,
		errfile: errfile,
	}

	processes.List = append(processes.List, process)

	return process, nil
}

func (process *Process) WaitForStart(dependency *Process) {
	process.Wait = append(process.Wait, &dependency.Status.Started)
}

// Exec runs the process using the arguments for a given command
func (process *Process) Exec(ctx context.Context, command string) (err error) {
	cmd := exec.CommandContext(ctx, process.Executable, process.Arguments[command]...)
	cmd.Dir = process.Directory
	cmd.Env = append(os.Environ(), "STORJ_LOG_NOTIME=1")
	cmd.Stdout, cmd.Stderr = process.stdout, process.stderr

	processgroup.Setup(cmd)

	// ensure that we always release all status fences
	defer process.Status.Started.Release()
	defer process.Status.Exited.Release()

	// wait for dependencies to start
	for _, fence := range process.Wait {
		fence.Wait()
	}

	// in case we have an explicit delay then sleep
	if process.Delay > 0 {
		if !sync2.Sleep(ctx, process.Delay) {
			return ctx.Err()
		}
	}

	if printCommands {
		fmt.Fprintf(process.processes.Output, "%s running: %v\n", process.Name, strings.Join(cmd.Args, " "))
		defer func() {
			fmt.Fprintf(process.processes.Output, "%s exited: %v\n", process.Name, err)
		}()
	}

	// start the process
	err = cmd.Start()
	if err != nil {
		return err
	}

	switch command {
	case "setup":
		// during setup we aren't starting the addresses, so we can release the depdendencies immediately
		process.Status.Started.Release()
	default:
		// release started when we are able to connect to the process address
		go process.monitorAddress()
	}

	// wait for process completion
	err = cmd.Wait()
	return err
}

// monitorAddress will monitor starting when we are able to start the process.
func (process *Process) monitorAddress() {
	for !process.Status.Started.Released() {
		if process.tryConnect() {
			process.Status.Started.Release()
			return
		}
		// wait a bit before retrying to reduce load
		time.Sleep(50 * time.Millisecond)
	}
}

// tryConnect will try to connect to the process public address
func (process *Process) tryConnect() bool {
	conn, err := net.Dial("tcp", process.Info.Address)
	if err != nil {
		return false
	}
	_, err = conn.Write([]byte{})
	// ignoring error, because we only care about being able to connect
	_ = conn.Close()
	return true
}

// Close closes process resources
func (process *Process) Close() error {
	return utils.CombineErrors(
		process.outfile.Close(),
		process.errfile.Close(),
	)
}
