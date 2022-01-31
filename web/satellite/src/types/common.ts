// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export enum SortDirection {
    ASCENDING = 1,
    DESCENDING,
}

export class PartneredSatellite {
    constructor(
        public name: string = '',
        public address: string = '',
    ) {}
}
