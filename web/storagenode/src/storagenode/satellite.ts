// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Satellite encapsulates satellite related data
 */
export class Satellite {
    public constructor(
        public id: string,
        public storageDaily: Stamp[],
        public bandwidthDaily: BandwidthUsed[],
        public egressDaily: EgressUsed[],
        public ingressDaily: IngressUsed[],
        public storageSummary: number,
        public bandwidthSummary: number,
        public egressSummary: number,
        public ingressSummary: number,
        public audit: Metric,
        public uptime: Metric) {}
}

/**
 * Stamp is storage usage stamp for satellite at some point in time
 */
export class Stamp {
    public atRestTotal: number;
    public intervalStart: Date;

    public constructor(atRestTotal: number = 0, intervalStart: Date = new Date()) {
        this.atRestTotal = atRestTotal;
        this.intervalStart = intervalStart;
    }

    /**
     * Creates new empty instance of stamp with defined date
     * @param date - holds specific date of the month
     * @returns Stamp - new empty instance of stamp with defined date
     */
    public static emptyWithDate(date: number): Stamp {
        const now = new Date();
        now.setUTCDate(date);
        now.setUTCHours(0, 0, 0, 0);

        return new Stamp(0, now);
    }
}

/**
 * Metric encapsulates storagenode reputation metrics
 */
export class Metric {
    public constructor(
        public totalCount: number = 0,
        public successCount: number = 0,
        public alpha: number = 0,
        public beta: number = 0,
        public unknownAlpha: number = 0,
        public unknownBeta: number = 0,
        public score: number = 0,
        public unknownScore: number = 0,
    ) {}
}

/**
 * Egress stores info about storage node egress usage
 */
export class Egress {
    public constructor(
        public audit: number = 0,
        public repair: number = 0,
        public usage: number = 0,
    ) {}
}

/**
 * Ingress stores info about storage node ingress usage
 */
export class Ingress {
    public constructor(
        public repair: number = 0,
        public usage: number = 0,
    ) {}
}

/**
 * BandwidthUsed stores bandwidth usage information over the period of time
 */
export class BandwidthUsed {
    public constructor(
        public egress: Egress,
        public ingress: Ingress,
        public intervalStart: Date,
    ) {}

    /**
     * Used to summarize all bandwidth usage data
     * @returns summary - sum of all bandwidth usage data
     */
    public summary(): number {
        return this.egress.audit + this.egress.repair + this.egress.usage +
        this.ingress.repair + this.ingress.usage;
    }

    /**
     * Creates new empty instance of used bandwidth with defined date
     * @param date - holds specific date of the month
     * @returns BandwidthUsed - new empty instance of used bandwidth with defined date
     */
    public static emptyWithDate(date: number): BandwidthUsed {
        const now = new Date();
        now.setUTCDate(date);
        now.setUTCHours(0, 0, 0, 0);

        return new BandwidthUsed(new Egress(0, 0, 0), new Ingress(0, 0), now);
    }
}

/**
 * EgressUsed stores egress bandwidth usage information over the period of time
 */
export class EgressUsed {
    public constructor(
        public egress: Egress,
        public intervalStart: Date) {}

    /**
     * Used to summarize all egress usage data
     * @returns summary - sum of all egress usage data
     */
    public summary(): number {
        return this.egress.audit + this.egress.repair + this.egress.usage;
    }

    /**
     * Creates new empty instance of used egress with defined date
     * @param date - holds specific date of the month
     * @returns EgressUsed - new empty instance of used egress with defined date
     */
    public static emptyWithDate(date: number): EgressUsed {
        const now = new Date();
        now.setDate(date);

        return new EgressUsed(new Egress(0, 0, 0), now);
    }
}

/**
 * IngressUsed stores ingress usage information over the period of time
 */
export class IngressUsed {
    public constructor(
        public ingress: Ingress,
        public intervalStart: Date) {}

    /**
     * Used to summarize all ingress usage data
     * @returns summary - sum of all ingress usage data
     */
    public summary(): number {
        return this.ingress.repair + this.ingress.usage;
    }

    /**
     * Creates new empty instance of used ingress with defined date
     * @param date - holds specific date of the month
     * @returns IngressUsed - new empty instance of used ingress with defined date
     */
    public static emptyWithDate(date: number): IngressUsed {
        const now = new Date();
        now.setDate(date);

        return new IngressUsed(new Ingress(0, 0), now);
    }
}

/**
 * Satellites encapsulate related data of all satellites
 */
export class Satellites {
    public constructor(
        public storageDaily: Stamp[],
        public bandwidthDaily: BandwidthUsed[],
        public egressDaily: EgressUsed[],
        public ingressDaily: IngressUsed[],
        public storageSummary: number,
        public bandwidthSummary: number,
        public egressSummary: number,
        public ingressSummary: number) {}
}
