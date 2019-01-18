// Copyright (C) 2018 Storj Labs, Inc.
// See LICENSE for copying information.

package testcontext

import (
	"os/exec"
	"path"
)

// Compile compiles the specified package and returns the executable name.
func (ctx *Context) Compile(pkg string) string {
	ctx.test.Helper()

	//TODO: only enable race when platform supports it
	exe := ctx.File("build", path.Base(pkg)+".exe")
	cmd := exec.Command("go", "build", "-race", "-o", exe, pkg)
	out, err := cmd.CombinedOutput()
	if err != nil {
		ctx.test.Error(string(out))
		ctx.test.Fatal(err)
	}

	return exe
}
