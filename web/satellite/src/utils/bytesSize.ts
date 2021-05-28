// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

enum Memory {
    Bytes = 1e0,
    KB = 1e3,
    MB = 1e6,
    GB = 1e9,
    TB = 1e12,
    PB = 1e15,
}

export enum Dimensions {
    Bytes = 'Bytes',
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

    public constructor(bytes: number, precision: number = 0) {
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
}
