// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export function formatBytes(bytes, decimals = 2) {
    if (bytes === 0) return '0 Bytes';

    const k = 1024;
    const dm = decimals < 0 ? 0 : decimals;

    return parseFloat((bytes / Math.pow(k, 3)).toFixed(dm)) + ' GB';
}
