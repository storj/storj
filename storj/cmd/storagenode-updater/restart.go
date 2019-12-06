// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build !unittest

package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"

	"github.com/zeebo/errs"
)

func restartService(name string) error {
	switch runtime.GOOS {
	case "windows":
		// TODO: cleanup temp .bat file
		restartSvcBatPath := filepath.Join(os.TempDir(), "restartservice.bat")
		restartSvcBat, err := os.Create(restartSvcBatPath)
		if err != nil {
			return err
		}

		restartStr := fmt.Sprintf("net stop %s && net start %s", name, name)
		_, err = restartSvcBat.WriteString(restartStr)
		if err != nil {
			return err
		}
		if err := restartSvcBat.Close(); err != nil {
			return err
		}

		out, err := exec.Command(restartSvcBat.Name()).CombinedOutput()
		if err != nil {
			return errs.New("%s", string(out))
		}
	default:
		return nil
	}
	return nil
}
