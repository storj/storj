// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export const KB = 1e3;
export const MB = 1e6;
export const GB = 1e9;

/**
 * Used to format amount from bytes to more compact unit
 * @param bytes - holds amount of bytes
 * @returns bytes - amount of formatted bytes with unit name
 */
export function formatBytes(bytes): string {
    if (bytes === 0) return '0 Bytes';

    const decimals = 2;

    const _bytes = Math.abs(bytes);

    switch (true) {
        case _bytes < MB:
            return `${(bytes / KB).toFixed(decimals)}KB`;
        case _bytes < GB:
            return `${(bytes / MB).toFixed(decimals)}MB`;
        default:
            return `${(bytes / GB).toFixed(decimals)}GB`;
    }
}
