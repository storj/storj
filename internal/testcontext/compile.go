// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testcontext

import (
	"os/exec"
	"path"
	"path/filepath"
)

// Compile compiles the specified package and returns the executable name.
func (ctx *Context) Compile(pkg string) string {
	ctx.test.Helper()

	exe := ctx.File("build", path.Base(pkg)+".exe")

	var cmd *exec.Cmd
	if raceEnabled {
		cmd = exec.Command("go", "build", "-race", "-o", exe, pkg)
	} else {
		cmd = exec.Command("go", "build", "-o", exe, pkg)
	}

	out, err := cmd.CombinedOutput()
	if err != nil {
		ctx.test.Error(string(out))
		ctx.test.Fatal(err)
	}

	return exe
}

func (ctx *Context) CompileC(libPath, glob string) string {
	ctx.test.Helper()

	exe := ctx.File("build", path.Base(glob)+".exe")

	files, err := filepath.Glob(glob)
	if err != nil {
		panic(err)
	}

	cmdString := append(append([]string{"-ggdb"}, files...), "-o", exe, libPath)
	cmd := exec.Command("gcc", cmdString...)

	out, err := cmd.CombinedOutput()
	if err != nil {
		ctx.test.Error(string(out))
		ctx.test.Fatal(err)
	}

	return exe
}
