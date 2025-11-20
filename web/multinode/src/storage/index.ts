// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Stamp is storage usage stamp for satellite at some point in time.
 */
export class Stamp {
    public atRestTotal: number;
    public atRestTotalBytes: number;
    public intervalStart: Date;

    public constructor(atRestTotal = 0, atRestTotalBytes = 0, intervalStart: Date = new Date()) {
        this.atRestTotal = atRestTotal;
        this.atRestTotalBytes = atRestTotalBytes;
        this.intervalStart = intervalStart;
    }

    /**
     * Creates new empty instance of stamp with defined date.
     * @param date - holds specific date of the month
     * @returns Stamp - new empty instance of stamp with defined date
     */
    public static emptyWithDate(date: number): Stamp {
        const now = new Date();

        now.setUTCDate(date);
        now.setUTCHours(0, 0, 0, 0);

        return new Stamp(0, 0, now);
    }
}

/**
 * DiskSpace is total storage usage for node if any selected.
 */
export class DiskSpace {
    public constructor(
        public allocated: number = 0,
        public used: number = 0,
        public usedPieces: number = 0,
        public usedTrash: number = 0,
        public usedReclaimable: number = 0,
        public free: number = 0,
        public available: number = 0,
        public overused: number = 0,
    ) {}
}

/**
 * DiskSpaceUsage contains daily and total disk space usage.
 */
export class DiskSpaceUsage {
    public constructor(
        public diskSpaceDaily: Stamp[] = [],
        public diskSpaceSummary: number = 0,
        public diskSpaceSummaryBytes: number = 0,
    ) {}
}
