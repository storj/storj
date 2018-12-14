// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"

	"golang.org/x/sync/errgroup"

	"storj.io/storj/internal/processgroup"
	"storj.io/storj/pkg/utils"
)

// Processes contains list of processes
type Processes struct {
	List []*Process
}

// NewProcesses creates a process-set with satellites and storage nodes
func NewProcesses(dir string, satelliteCount, storageNodeCount int) (*Processes, error) {
	processes := &Processes{}

	for i := 0; i < satelliteCount; i++ {
		name := fmt.Sprintf("satellite/%d", i)

		dir := filepath.Join(dir, "satellite", fmt.Sprint(i))
		if err := os.MkdirAll(dir, 0644); err != nil {
			return nil, err
		}

		process, err := NewProcess(name, "satellite", dir)
		if err != nil {
			return nil, utils.CombineErrors(err, processes.Close())
		}
		processes.List = append(processes.List, process)

		process.Arguments["run"] = []string{"run", "--base-path", "."}
		process.Arguments["setup"] = []string{"--base-path", ".", "--overwrite"}
	}

	for i := 0; i < storageNodeCount; i++ {
		name := fmt.Sprintf("storage/%d", i)

		dir := filepath.Join(dir, "storagenode", fmt.Sprint(i))
		if err := os.MkdirAll(dir, 0644); err != nil {
			return nil, err
		}

		process, err := NewProcess(name, "storagenode", dir)
		if err != nil {
			return nil, utils.CombineErrors(err, processes.Close())
		}
		processes.List = append(processes.List, process)

		process.Arguments["run"] = []string{"run", "--base-path", "."}
		process.Arguments["setup"] = []string{"--base-path", ".", "--overwrite"}
	}

	return processes, nil
}

// Exec executes a command on all processes
func (processes *Processes) Exec(ctx context.Context, command string) error {
	var group errgroup.Group
	for _, p := range processes.List {
		process := p
		group.Go(func() error {
			return process.Exec(ctx, command)
		})
	}

	return group.Wait()
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

// Process is a type for monitoring the process
type Process struct {
	Name       string
	Directory  string
	Executable string

	Arguments map[string][]string

	Stdout io.Writer
	Stderr io.Writer

	stdout *os.File
	stderr *os.File
}

// NewProcess creates a process which can be run in the specified directory
func NewProcess(name, executable, directory string) (*Process, error) {
	stdout, err1 := os.OpenFile(filepath.Join(directory, "stderr.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)
	stderr, err2 := os.OpenFile(filepath.Join(directory, "stdout.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0600)

	return &Process{
		Name:       name,
		Directory:  directory,
		Executable: executable,

		Arguments: map[string][]string{},

		Stdout: io.MultiWriter(os.Stdout, stdout),
		Stderr: io.MultiWriter(os.Stderr, stderr),

		stdout: stdout,
		stderr: stderr,
	}, utils.CombineErrors(err1, err2)
}

// Exec runs the process using the arguments for a given command
func (process *Process) Exec(ctx context.Context, command string) error {
	cmd := exec.Command(process.Executable, process.Arguments[command]...)
	cmd.Dir = process.Directory
	cmd.Stdout, cmd.Stderr = process.Stdout, process.Stderr

	processgroup.Setup(cmd)

	return cmd.Run()
}

// Close closes process resources
func (process *Process) Close() error {
	return utils.CombineErrors(
		process.stdout.Close(),
		process.stderr.Close(),
	)
}
