// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Dashboard encapsulates dashboard stale data
 */
export class Dashboard {
    public constructor(
        public nodeID: string,
        public wallet: string,
        public satellites: SatelliteInfo[],
        public diskSpace: DiskSpaceInfo,
        public bandwidth: BandwidthInfo,
        public lastPinged: Date,
        public startedAt: Date,
        public version: Version,
        public isUpToDate: boolean) {}
}

/**
 * Version represents a semantic version
 */
export class Version {
    public constructor(
        public major: number,
        public minor: number,
        public patch: number) {}

    /**
     * Converts version numbers to string type
     * @returns version - string of version value
     */
    public toString(): string {
        return `v${this.major}.${this.minor}.${this.patch}`;
    }
}

/**
 * SatelliteInfo encapsulates satellite ID and disqualification
 */
export class SatelliteInfo {
    public constructor(
        public id: string,
        public disqualified: Date | null,
    ) {}
}

/**
 * DiskSpaceInfo stores all info about storage node disk space usage
 */
export class DiskSpaceInfo {
    public remaining: number;

    public constructor(
        public used: number,
        public available: number) {
        this.remaining = available - used;
    }
}

/**
 * BandwidthInfo stores all info about storage node bandwidth usage
 */
export class BandwidthInfo {
    public remaining: number;

    public constructor(
        public used: number,
        public available: number) {
        this.remaining = available - used;
    }
}
