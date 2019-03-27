// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

package process // import "storj.io/storj/pkg/process"

import (
	"fmt"
	"github.com/spf13/cobra"
	"log"
	"os"

	// We use a blank import here to get the side effects from the init function in version
	_ "storj.io/storj/internal/version"
)

func init() {
	cobra.MousetrapHelpText = "This is a command line tool.\n\n" +
		"This needs to be run from a Command Prompt.\n"

	// Figure out the executable name.
	exe, err := os.Executable()
	if err == nil {
		cobra.MousetrapHelpText += fmt.Sprintf(
			"Try running \"%s help\" for more information\n", exe)
	}

}

// check if file exists, handle error correctly if it doesn't
func fileExists(path string) bool {
	_, err := os.Stat(path)
	if err != nil {
		if os.IsNotExist(err) {
			return false
		}
		log.Fatalf("failed to check for file existence: %v", err)
	}
	return true
}
