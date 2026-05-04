// Copyright (C) 2026 Storj Labs, Inc.
// See LICENSE for copying information.

package load

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/unix"
)

// rawDiskStats holds the raw cumulative counters from /proc/diskstats for a single device.
// Field names match the kernel documentation (Documentation/admin-guide/iostats.rst).
type rawDiskStats struct {
	ReadsCompleted  uint64
	ReadsMerged     uint64
	SectorsRead     uint64
	MsReading       uint64
	WritesCompleted uint64
	WritesMerged    uint64
	SectorsWritten  uint64
	MsWriting       uint64
	IOsInProgress   uint64
	MsDoingIO       uint64
	WeightedMsIO    uint64
}

// readDiskStats reads /proc/diskstats and returns raw stats for the given device name (e.g. "sda", "dm-0", "nvme0n1").
func readDiskStats(deviceName string) (rawDiskStats, error) {
	f, err := os.Open("/proc/diskstats")
	if err != nil {
		return rawDiskStats{}, err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var major, minor uint64
		var name string
		var s rawDiskStats

		n, _ := fmt.Sscanf(scanner.Text(), "%d %d %s %d %d %d %d %d %d %d %d %d %d %d",
			&major, &minor, &name,
			&s.ReadsCompleted, &s.ReadsMerged, &s.SectorsRead, &s.MsReading,
			&s.WritesCompleted, &s.WritesMerged, &s.SectorsWritten, &s.MsWriting,
			&s.IOsInProgress, &s.MsDoingIO, &s.WeightedMsIO,
		)
		if n < 14 {
			continue
		}
		if name == deviceName {
			return s, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return rawDiskStats{}, err
	}
	return rawDiskStats{}, fmt.Errorf("device %q not found in /proc/diskstats", deviceName)
}

// deviceNameFromPath resolves a filesystem path to its underlying block device name
// by looking up the device major:minor from stat(2) and finding the matching
// entry in /proc/diskstats.
func deviceNameFromPath(dir string) (string, error) {
	var stat unix.Stat_t
	err := unix.Stat(dir, &stat)
	if err != nil {
		return "", fmt.Errorf("stat %q: %w", dir, err)
	}

	targetMajor := unix.Major(stat.Dev)
	targetMinor := unix.Minor(stat.Dev)

	f, err := os.Open("/proc/diskstats")
	if err != nil {
		return "", err
	}
	defer func() { _ = f.Close() }()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		var major, minor uint64
		var name string
		n, _ := fmt.Sscanf(scanner.Text(), "%d %d %s", &major, &minor, &name)
		if n < 3 {
			continue
		}
		if uint32(major) == targetMajor && uint32(minor) == targetMinor {
			return name, nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}

	// If we didn't find a direct match, the device might be a partition.
	// Try to find it via /sys/dev/block.
	sysPath := fmt.Sprintf("/sys/dev/block/%d:%d", targetMajor, targetMinor)
	resolved, err := filepath.EvalSymlinks(sysPath)
	if err != nil {
		return "", fmt.Errorf("device %d:%d not found in /proc/diskstats", targetMajor, targetMinor)
	}
	// resolved is something like /sys/devices/.../dm-0 or /sys/devices/.../sda1
	devName := filepath.Base(resolved)

	// Check if this device name has a dm- prefix (device mapper) or is a partition.
	// For partitions (e.g. sda1), we want the whole device (sda) for disk-level stats.
	// But device-mapper devices (dm-*) are already the right level.
	if strings.HasPrefix(devName, "dm-") {
		return devName, nil
	}

	// For regular partitions, trim trailing digits to get the disk device.
	diskName := strings.TrimRight(devName, "0123456789")
	if diskName == "" {
		diskName = devName
	}

	return diskName, nil
}
