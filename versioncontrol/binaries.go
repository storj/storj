// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

package versioncontrol

// SupportedBinaries list of supported binary schemes.
var SupportedBinaries = []string{
	"identity_darwin_amd64",
	"identity_freebsd_amd64",
	"identity_linux_amd64",
	"identity_linux_arm",
	"identity_linux_arm64",
	"identity_windows_amd64",
	"storagenode-updater_linux_amd64",
	"storagenode-updater_linux_arm",
	"storagenode-updater_linux_arm64",
	"storagenode-updater_windows_amd64",
	"storagenode_freebsd_amd64",
	"storagenode_linux_amd64",
	"storagenode_linux_arm",
	"storagenode_linux_arm64",
	"storagenode_windows_amd64",
	"uplink_darwin_amd64",
	"uplink_freebsd_amd64",
	"uplink_linux_amd64",
	"uplink_linux_arm",
	"uplink_linux_arm64",
	"uplink_windows_amd64",
}

// isBinarySupported check if binary scheme matching provided service, os and arch is supported.
func isBinarySupported(service, os, arch string) (string, bool) {
	binary := service + "_" + os + "_" + arch
	for _, supportedBinary := range SupportedBinaries {
		if binary == supportedBinary {
			return binary, true
		}
	}
	return binary, false
}
