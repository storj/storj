// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package testcmd

import (
	"fmt"
	"os"
	"os/exec"
	"path"
)

// Build does stuff
func Build(packages ...string) (string, func() error, error) {
	root := os.TempDir()

	for _, pkg := range packages {
		exe := path.Base(pkg) + ".exe"
		cmd := exec.Command("go", "build", "-o", exe, pkg)
		out, err := cmd.CombinedOutput()
		if err != nil {
			fmt.Println(string(out))
			return "", func() error { return nil }, err
		}
	}

	return root, func() error {
		return os.RemoveAll(root)
	}, nil
}
