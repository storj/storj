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
        public version: string,
        public allowedVersion: string,
        public isUpToDate: boolean,
    ) { }
}

/**
 * SatelliteInfo encapsulates satellite ID, URL and disqualification
 */
export class SatelliteInfo {
    public constructor(
        public id: string,
        public url: string,
        public disqualified: Date | null,
        public suspended: Date | null,
    ) { }
}

/**
 * DiskSpaceInfo stores all info about storage node disk space usage
 */
export class DiskSpaceInfo {
    public remaining: number;

    public constructor(
        public used: number,
        public available: number,
    ) {
        this.remaining = available - used;
    }
}

/**
 * BandwidthInfo stores all info about storage node bandwidth usage
 */
export class BandwidthInfo {
    public constructor(
        public used: number,
    ) { }
}
