// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Base10 sizes.
 */
export enum SizeBreakpoints {
    KB = 1e3,
    MB = 1e6,
    GB = 1e9,
    TB = 1e12,
    PB = 1e15,
    EB = 1e18,
}

/**
 * Size class contains size related functionality such as convertation.
 */
export class Size {
    /**
     * Base10String converts size to a string using base-10 prefixes.
     * @param size in bytes
     */
    public static toBase10String(size: number): string {
        const decimals = 2;

        const _size = Math.abs(size);

        switch (true) {
        case _size >= SizeBreakpoints.EB * 2 / 3:
            return `${parseFloat((size / SizeBreakpoints.EB).toFixed(decimals))}EB`;
        case _size >= SizeBreakpoints.PB * 2 / 3:
            return `${parseFloat((size / SizeBreakpoints.PB).toFixed(decimals))}PB`;
        case _size >= SizeBreakpoints.TB * 2 / 3:
            return `${parseFloat((size / SizeBreakpoints.TB).toFixed(decimals))}TB`;
        case _size >= SizeBreakpoints.GB * 2 / 3:
            return `${parseFloat((size / SizeBreakpoints.GB).toFixed(decimals))}GB`;
        case _size >= SizeBreakpoints.MB * 2 / 3:
            return `${parseFloat((size / SizeBreakpoints.MB).toFixed(decimals))}MB`;
        case _size >= SizeBreakpoints.KB * 2 / 3:
            return `${parseFloat((size / SizeBreakpoints.KB).toFixed(decimals))}KB`;
        default:
            return `${size ? size.toFixed(decimals) : size}B`;
        }
    }
}
