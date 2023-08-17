// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export enum Memory {
    Bytes = 1e0,
    KB = 1e3,
    MB = 1e6,
    GB = 1e9,
    TB = 1e12,
    PB = 1e15,
    EB = 1e18,

    KiB = 2 ** 10,
    MiB = 2 ** 20,
    GiB = 2 ** 30,
    TiB = 2 ** 40,
    PiB = 2 ** 50,
    EiB = 2 ** 60,
}

export enum Dimensions {
    Bytes = 'B',
    KB = 'KB',
    MB = 'MB',
    GB = 'GB',
    TB = 'TB',
    PB = 'PB',
}

export class Size {
    private readonly precision: number;
    public readonly bytes: number;
    public readonly formattedBytes: string;
    public readonly label: Dimensions;

    public constructor(bytes: number, precision = 0) {
        const _bytes = Math.ceil(bytes);
        this.bytes = bytes;
        this.precision = precision;

        switch (true) {
        case _bytes === 0:
            this.formattedBytes = (bytes / Memory.Bytes).toFixed(this.precision);
            this.label = Dimensions.Bytes;
            break;
        case _bytes < Memory.MB:
            this.formattedBytes = (bytes / Memory.KB).toFixed(this.precision);
            this.label = Dimensions.KB;
            break;
        case _bytes < Memory.GB:
            this.formattedBytes = (bytes / Memory.MB).toFixed(this.precision);
            this.label = Dimensions.MB;
            break;
        case _bytes < Memory.TB:
            this.formattedBytes = (bytes / Memory.GB).toFixed(this.precision);
            this.label = Dimensions.GB;
            break;
        case _bytes < Memory.PB:
            this.formattedBytes = (bytes / Memory.TB).toFixed(this.precision);
            this.label = Dimensions.TB;
            break;
        default:
            this.formattedBytes = (bytes / Memory.PB).toFixed(this.precision);
            this.label = Dimensions.PB;
        }
    }

    /**
     * Base10String converts size to a string using base-10 prefixes.
     * @param size in bytes
     */
    public static toBase10String(size: number): string {
        const decimals = 2;

        const _size = Math.abs(size);

        switch (true) {
        case _size >= Memory.EB * 2 / 3:
            return `${parseFloat((size / Memory.EB).toFixed(decimals))}EB`;
        case _size >= Memory.PB * 2 / 3:
            return `${parseFloat((size / Memory.PB).toFixed(decimals))}PB`;
        case _size >= Memory.TB * 2 / 3:
            return `${parseFloat((size / Memory.TB).toFixed(decimals))}TB`;
        case _size >= Memory.GB * 2 / 3:
            return `${parseFloat((size / Memory.GB).toFixed(decimals))}GB`;
        case _size >= Memory.MB * 2 / 3:
            return `${parseFloat((size / Memory.MB).toFixed(decimals))}MB`;
        case _size >= Memory.KB * 2 / 3:
            return `${parseFloat((size / Memory.KB).toFixed(decimals))}KB`;
        default:
            return `${size}B`;
        }
    }
}
