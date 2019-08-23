// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export const KB = 1e3;
export const MB = 1e6;
export const GB = 1e9;

export function formatBytes(bytes): string {
    if (bytes === 0) return '0 Bytes';

    const decimals = 2;

    let _bytes = Math.abs(bytes);

    switch (true) {
        case _bytes < MB:
            return `${(bytes / KB).toFixed(decimals)}KB`;
        case _bytes < GB:
            return `${(bytes / MB).toFixed(decimals)}MB`;
        default:
            return `${(bytes / GB).toFixed(decimals)}GB`;
    }
}
