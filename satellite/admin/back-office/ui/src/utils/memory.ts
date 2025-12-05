// Copyright (C) 2023 Storj Labs, Inc.
// See LICENSE for copying information.

export enum Memory {
    B = 1e0,
    KB = 1e3,
    MB = 1e6,
    GB = 1e9,
    TB = 1e12,
    PB = 1e15,
    EB = 1e18,
}

/**
 * sizeToBase10String converts size to a string using base-10 prefixes.
 * @param size - size in bytes
 */
export function sizeToBase10String(size: number, decimals = 2): string {
    const _size = Math.abs(size);

    const amounts = Object.values(Memory).filter((v): v is number => typeof v === 'number').sort().reverse();
    for (const amount of Object.values(amounts)) {
        if (_size >= amount * 2 / 3) {
            return `${(size / amount).toLocaleString(undefined, { maximumFractionDigits: decimals })} ${Memory[amount]}`;
        }
    }

    return size.toLocaleString(undefined, { maximumFractionDigits: decimals }) + ' B';
}
