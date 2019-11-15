// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// +build !windows

package main

import "storj.io/private/process"

// TODO: improve logging; other commands use zap but due to an apparent
// windows bug we're unable to use the existing process logging infrastructure.
func openLog() (closeFunc func() error, err error) {
	closeFunc = func() error { return nil }

	if runCfg.Log != "" {
		logFile, err := os.OpenFile(runCfg.Log, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Printf("error opening log file: %s", err)
			return closeFunc, err
		}
		log.SetOutput(logFile)
		return logFile.Close, nil
	}
	return closeFunc, nil
}

func main() {
	process.Exec(rootCmd)
}
