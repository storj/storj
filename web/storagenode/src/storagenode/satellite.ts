// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

// Satellite encapsulates satellite related data.
export class Satellite {
    public constructor(
        public id: string,
        public storageDaily: Stamp[],
        public bandwidthDaily: BandwidthUsed[],
        public storageSummary: number,
        public bandwidthSummary: number,
        public audit: Metric,
        public uptime: Metric) {}
}

// Stamp is storage usage stamp for satellite at some point in time
export class Stamp {
    public atRestTotal: number;
    public timestamp: Date;

    public constructor(atRestTotal: number, timestamp: Date) {
        this.atRestTotal = atRestTotal;
        this.timestamp = timestamp;
    }

    public static emptyWithDate(date: number): Stamp {
        const now = new Date();
        let timestamp = new Date(now.getUTCFullYear(), now.getUTCMonth());

        return new Stamp(0, timestamp);
    }
}

// Metric encapsulates storagenode reputation metrics
export class Metric {
    public constructor(
        public totalCount: number,
        public successCount: number,
        public alpha: number,
        public beta: number,
        public score: number) {}
}

// Egress stores info about storage node egress usage
export class Egress {
    public constructor(
        public repair: number,
        public audit: number,
        public usage: number) {}
}

// Ingress stores info about storage node ingress usage
export class Ingress {
    public constructor(
        public repair: number,
        public usage: number) {}
}

// BandwidthUsed stores bandwidth usage information over the period of time
export class BandwidthUsed {
    public constructor(
        public egress: Egress,
        public ingress: Ingress,
        public from: Date,
        public to: Date) {}

    public summary(): number {
        return this.egress.audit + this.egress.repair + this.egress.usage +
        this.ingress.repair + this.ingress.usage;
    }

    public static emptyWithDate(date: number): BandwidthUsed {
        const now = new Date();

        return new BandwidthUsed(new Egress(0, 0, 0), new Ingress(0, 0), now, now);
    }
}

export class Satellites {
    public constructor(
        public storageDaily: Stamp[],
        public bandwidthDaily: BandwidthUsed[],
        public storageSummary: number,
        public bandwidthSummary: number) {}
}
