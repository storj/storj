// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

enum Memory {
    Kib = 1e3,
    Mib = 1e6,
    Gib = 1e9,
    Tib = 1e12,
    Pib = 1e15,
}

enum Dimensions {
    Kib = 'KB',
    Mib = 'MB',
    Gib = 'GB',
    Tib = 'TB',
    Pib = 'PB',
}

export class Size {
    private readonly bytes: number;
    private readonly precision: number = 4;
    public readonly formattedBytes: string;
    public readonly label: Dimensions;

    public constructor(bytes: number) {
        const _bytes = Math.ceil(bytes);
        this.bytes = bytes;

        switch (true) {
            case _bytes < Memory.Mib:
                this.formattedBytes = (bytes / Memory.Kib).toFixed(this.precision);
                this.label = Dimensions.Kib;
                break;
            case _bytes < Memory.Gib:
                this.formattedBytes = (bytes / Memory.Mib).toFixed(this.precision);
                this.label = Dimensions.Mib;
                break;
            case _bytes < Memory.Tib:
                this.formattedBytes = (bytes / Memory.Gib).toFixed(this.precision);
                this.label = Dimensions.Gib;
                break;
            case _bytes < Memory.Pib:
                this.formattedBytes = (bytes / Memory.Tib).toFixed(this.precision);
                this.label = Dimensions.Tib;
                break;
            default:
                this.formattedBytes = (bytes / Memory.Pib).toFixed(this.precision);
                this.label = Dimensions.Pib;
        }
    }
}
