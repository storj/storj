// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

export enum SortDirection {
    ASCENDING = 1,
    DESCENDING,
}

export enum OnboardingOS {
    WINDOWS = "windows",
    MAC = "macos",
    LINUX = "linux",
}

export class PartneredSatellite {
    constructor(
        public name: string = '',
        public address: string = '',
    ) {}
}
