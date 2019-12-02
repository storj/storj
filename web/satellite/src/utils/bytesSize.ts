// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

const KB = 1e3;
const MB = 1e6;
const GB = 1e9;
const TB = 1e12;
const PB = 1e15;
const decimals = 4;

/**
 * Used to display correct and convenient bytes size
 */
export class BytesSize {
    /**
     * Used to format amount from bytes to more compact unit
     * @param bytes - holds amount of bytes
     * @returns bytes - amount of formatted bytes
     */
    public static formatBytes(bytes: number): string {
        if (bytes === 0) return '0.0000';

        const _bytes = Math.ceil(bytes);
        switch (true) {
            case _bytes < MB:
                return BytesSize.KiB(bytes);
            case _bytes < GB:
                return BytesSize.MiB(bytes);
            case _bytes < TB:
                return BytesSize.GiB(bytes);
            case _bytes < PB:
                return BytesSize.TiB(bytes);
            default:
                return BytesSize.PiB(bytes);
        }
    }

    /**
     * gets bytes dimension depending on bytes size
     * @param bytes - holds array of bytes in numeric form
     * @returns string of bytes dimension
     */
    public static getBytesDimension(bytes: number): string {
        if (bytes === 0) return 'Bytes';

        const _bytes = Math.ceil(bytes);
        switch (true) {
            case _bytes < MB:
                return 'KB';
            case _bytes < GB:
                return 'MB';
            case _bytes < TB:
                return 'GB';
            case _bytes < PB:
                return 'TB';
            default:
                return 'PB';
        }
    }

    /**
     * Used to format amount from bytes to KB
     * @param bytes - holds amount of bytes
     * @returns string of formatted bytes
     */
    private static KiB(bytes: number): string {
        return (bytes / KB).toFixed(decimals);
    }

    /**
     * Used to format amount from bytes to MB
     * @param bytes - holds amount of bytes
     * @returns string of formatted bytes
     */
    private static MiB(bytes: number): string {
        return (bytes / MB).toFixed(decimals);
    }

    /**
     * Used to format amount from bytes to GB
     * @param bytes - holds amount of bytes
     * @returns string of formatted bytes
     */
    private static GiB(bytes: number): string {
        return (bytes / GB).toFixed(decimals);
    }

    /**
     * Used to format amount from bytes to TB
     * @param bytes - holds amount of bytes
     * @returns string of formatted bytes
     */
    private static TiB(bytes: number): string {
        return (bytes / TB).toFixed(decimals);
    }

    /**
     * Used to format amount from bytes to PB
     * @param bytes - holds amount of bytes
     * @returns string of formatted bytes
     */
    private static PiB(bytes: number): string {
        return (bytes / PB).toFixed(decimals);
    }
}
