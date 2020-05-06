// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package main

import (
	"archive/zip"
	"bufio"
	"bytes"
	"context"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"strings"

	"github.com/zeebo/errs"
	"go.uber.org/zap"

	"storj.io/common/sync2"
	"storj.io/private/version"
)

func binaryVersion(location string) (version.SemVer, error) {
	out, err := exec.Command(location, "version").CombinedOutput()
	if err != nil {
		zap.L().Info("Command output.", zap.ByteString("Output", out))
		return version.SemVer{}, err
	}

	scanner := bufio.NewScanner(bytes.NewReader(out))
	for scanner.Scan() {
		line := scanner.Text()
		prefix := "Version: "
		if strings.HasPrefix(line, prefix) {
			line = line[len(prefix):]
			return version.NewSemVer(line)
		}
	}
	return version.SemVer{}, errs.New("unable to determine binary version")
}

func downloadBinary(ctx context.Context, url, target string) error {
	f, err := ioutil.TempFile("", createPattern(url))
	if err != nil {
		return errs.New("cannot create temporary archive: %v", err)
	}
	defer func() {
		err = errs.Combine(err,
			f.Close(),
			os.Remove(f.Name()),
		)
	}()

	zap.L().Info("Download started.", zap.String("From", url), zap.String("To", f.Name()))

	if err = downloadArchive(ctx, f, url); err != nil {
		return errs.Wrap(err)
	}
	if err = unpackBinary(ctx, f.Name(), target); err != nil {
		return errs.Wrap(err)
	}

	zap.L().Info("Download finished.", zap.String("From", url), zap.String("To", f.Name()))
	return nil
}

func downloadArchive(ctx context.Context, file io.Writer, url string) (err error) {
	resp, err := http.Get(url)
	if err != nil {
		return err
	}

	defer func() { err = errs.Combine(err, resp.Body.Close()) }()

	if resp.StatusCode != http.StatusOK {
		return errs.New("bad status: %s", resp.Status)
	}

	_, err = sync2.Copy(ctx, file, resp.Body)
	return err
}

// unpackBinary unpack zip compressed binary.
func unpackBinary(ctx context.Context, archive, target string) (err error) {
	zipReader, err := zip.OpenReader(archive)
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, zipReader.Close()) }()

	if len(zipReader.File) != 1 {
		return errs.New("archive should contain only one file")
	}

	zipedExec, err := zipReader.File[0].Open()
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, zipedExec.Close()) }()

	newExec, err := os.OpenFile(target, os.O_CREATE|os.O_EXCL|os.O_WRONLY, os.FileMode(0755))
	if err != nil {
		return err
	}
	defer func() { err = errs.Combine(err, newExec.Close()) }()

	_, err = sync2.Copy(ctx, newExec, zipedExec)
	if err != nil {
		return errs.Combine(err, os.Remove(newExec.Name()))
	}
	return nil
}
