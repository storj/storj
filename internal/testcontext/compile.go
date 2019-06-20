// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testcontext

import (
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"testing"
)

var CLibMath = Include{Library: "m", Installed: true}

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
	ctx.test.Log("exec:", cmd.Args)

	out, err := cmd.CombinedOutput()
	if err != nil {
		ctx.test.Error(string(out))
		ctx.test.Fatal(err)
	}

	return exe
}

// CompileShared compiles pkg as c-shared.
// TODO: support inclusion from other directories
//  (cgo header paths are currently relative to package root)
func (ctx *Context) CompileShared(t *testing.T, name string, pkg string) Include {
	t.Helper()

	base := ctx.File("build", name)

	// not using race detector for c-shared
	cmd := exec.Command("go", "build", "-buildmode", "c-shared", "-o", base+".so", pkg)
	t.Log("exec:", cmd.Args)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Error(string(out))
		t.Fatal(err)
	}
	t.Log(string(out))

	return Include{Header: base + ".h", Library: base + ".so"}
}

// CompileC compiles file as with gcc and adds the includes.
func (ctx *Context) CompileC(t *testing.T, file string, includes ...Include) string {
	t.Helper()

	exe := ctx.File("build", filepath.Base(file)+".exe")

	var args = []string{}
	args = append(args, "-ggdb", "-Wall")
	args = append(args, "-o", exe)
	for _, inc := range includes {
		if inc.Header != "" {
			args = append(args, "-I", filepath.Dir(inc.Header))
		}
		if inc.Library != "" {
			if inc.Installed {
				args = append(args,
					"-l"+inc.Library,
				)
			} else {
				if runtime.GOOS == "windows" {
					args = append(args,
						"-L"+filepath.Dir(inc.Library),
						"-l:"+filepath.Base(inc.Library),
					)
				} else {
					args = append(args, inc.Library)
				}
			}
		}
	}
	args = append(args, file)

	cmd := exec.Command("gcc", args...)
	t.Log("exec:", cmd.Args)

	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Error(string(out))
		t.Fatal(err)
	}
	t.Log(string(out))

	return exe
}

// Include defines an includable library for gcc.
type Include struct {
	Header    string
	Library   string
	Installed bool
}
