// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

const KB = 1e3;
const MB = 1e6;
const GB = 1e9;
const TB = 1e12;
const PB = 1e15;

/**
 * Used to display correct and convenient data
 */
export class Converter {
    /**
     * Used to format amount from bytes to more compact unit
     * @param bytes - holds amount of bytes
     * @returns bytes - amount of formatted bytes
     */
    public static formatBytes(bytes: number): string {
        if (bytes === 0) return '0.0000';

        const decimals = 4;
        const _bytes = Math.ceil(bytes);

        switch (true) {
            case _bytes < MB:
                return (bytes / KB).toFixed(decimals);
            case _bytes < GB:
                return (bytes / MB).toFixed(decimals);
            case _bytes < TB:
                return (bytes / GB).toFixed(decimals);
            case _bytes < PB:
                return (bytes / TB).toFixed(decimals);
            default:
                return (bytes / PB).toFixed(decimals);
        }
    }

    /**
     * gets data dimension depending on data size
     * @param data - holds array of data in numeric form
     * @returns dataDimension - string of data dimension
     */
    public static getDataDimension(data: number): string {
        if (data === 0) return 'Bytes';

        const maxBytes = Math.ceil(data);

        let dataDimension: string = '';
        switch (true) {
            case maxBytes < MB:
                dataDimension = 'KB';
                break;
            case maxBytes < GB:
                dataDimension = 'MB';
                break;
            case maxBytes < TB:
                dataDimension = 'GB';
                break;
            case maxBytes < PB:
                dataDimension = 'TB';
                break;
            default:
                dataDimension = 'PB';
        }

        return dataDimension;
    }
}
