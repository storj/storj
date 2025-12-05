// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { StatusOffline } from '@/app/store/modules/nodeStore';

/**
 * Hold common node information.
 */
export class Node {
    public constructor(
        public id: string = '',
        public status: string = StatusOffline,
        public lastPinged: Date = new Date(),
        public startedAt: Date = new Date(),
        public version: string = '',
        public allowedVersion: string = '',
        public wallet: string = '',
        public walletFeatures: string[] = [],
        public isLastVersion: boolean = false,
        public quicStatus: string = '',
        public configuredPort: string = '',
        public lastQuicPingedAt: Date = new Date(),
    ) {}
}

/**
 * Holds traffic usage information by type.
 */
export class Utilization {
    public constructor(
        public bandwidth: Traffic = new Traffic(),
        public diskSpace: DiskSpace = new DiskSpace(),
    ) {}
}

/**
 * Holds traffic usage information.
 */
export class Traffic {
    public constructor(
        public used: number = 0,
    ) {}
}

export class DiskSpace {
    public constructor(
        public used: number = 0,
        public allocated: number = 1,
        public trash: number = 0,
        public overused: number = 0,
        public reclaimable: number = 0,
        public reserved: number = 0,
    ) {}
}

/**
 * Holds audit and suspension checks.
 */
export class Checks {
    public audit = 0;
    public suspension = 0;

    public constructor(
        audit: Metric = new Metric(),
    ) {
        this.audit = parseFloat(parseFloat(`${audit.score * 100}`).toFixed(1));
        this.suspension = audit.unknownScore * 100;
    }
}

/**
 * Dashboard encapsulates dashboard stale data.
 */
export class Dashboard {
    public constructor(
        public nodeID: string,
        public wallet: string,
        public walletFeatures: string[],
        public satellites: SatelliteInfo[],
        public diskSpace: DiskSpace,
        public bandwidth: Traffic,
        public lastPinged: Date,
        public startedAt: Date,
        public version: string,
        public allowedVersion: string,
        public isUpToDate: boolean,
        public quicStatus: string,
        public configuredPort: string,
        public lastQuicPingedAt: Date,
    ) { }
}

/**
 * SatelliteInfo encapsulates satellite ID, URL, join date and disqualification.
 */
export class SatelliteInfo {
    public constructor(
        public id: string = '',
        public url: string = '',
        public disqualified: Date | null = null,
        public suspended: Date | null = null,
        public vettedAt: Date | null = null,
        public joinDate: Date = new Date(),
    ) { }
}

/**
 * Satellite encapsulates satellite related data
 */
export class Satellite {
    public constructor(
        public id: string = '',
        public storageDaily: Stamp[] = [],
        public bandwidthDaily: BandwidthUsed[] = [],
        public egressDaily: EgressUsed[] = [],
        public ingressDaily: IngressUsed[] = [],
        public storageSummary: number = 0,
        public averageUsageBytes: number = 0,
        public bandwidthSummary: number = 0,
        public egressSummary: number = 0,
        public ingressSummary: number = 0,
        public audits: SatelliteScores = new SatelliteScores(),
        public joinDate: Date = new Date(),
        public vettedAt: Date | null = null,
    ) {}
}

/**
 * Stamp is storage usage stamp for satellite at some point in time
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
     * Creates new empty instance of stamp with defined date
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
        public intervalStart: Date,
    ) {}

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
        now.setUTCDate(date);
        now.setUTCHours(0, 0, 0, 0);

        return new EgressUsed(new Egress(0, 0, 0), now);
    }
}

/**
 * IngressUsed stores ingress usage information over the period of time
 */
export class IngressUsed {
    public constructor(
        public ingress: Ingress,
        public intervalStart: Date,
    ) {}

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
        now.setUTCDate(date);
        now.setUTCHours(0, 0, 0, 0);

        return new IngressUsed(new Ingress(0, 0), now);
    }
}

/**
 * Satellites encapsulate related data of all satellites
 */
export class Satellites {
    public constructor(
        public storageDaily: Stamp[] = [],
        public bandwidthDaily: BandwidthUsed[] = [],
        public egressDaily: EgressUsed[] = [],
        public ingressDaily: IngressUsed[] = [],
        public storageSummary: number = 0,
        public averageUsageBytes: number = 0,
        public bandwidthSummary: number = 0,
        public egressSummary: number = 0,
        public ingressSummary: number = 0,
        public joinDate: Date = new Date(),
        public satellitesScores: SatelliteScores[] = [],
    ) {}
}

/**
 * Holds information about audit and suspension scores by satellite.
 */
export class SatelliteScores {
    public auditScore: Score;
    public suspensionScore: Score;
    public onlineScore: Score;
    public iconClassName = '';

    private readonly WARNING_CLASSNAME: string = 'warning';
    private readonly DISQUALIFICATION_CLASSNAME: string = 'disqualification';

    public constructor(
        public satelliteName: string = 'satellite-name',
        auditScore = 0,
        unknownScore = 0,
        onlineScore = 0,
    ) {
        this.auditScore = new Score(auditScore);
        this.suspensionScore = new Score(unknownScore);
        this.onlineScore = new Score(onlineScore);
        const scores = [this.auditScore, this.onlineScore, this.suspensionScore];

        if (scores.some(score => score.statusClassName === this.DISQUALIFICATION_CLASSNAME)) {
            this.iconClassName = this.DISQUALIFICATION_CLASSNAME;

            return;
        }

        if (scores.some(score => score.statusClassName === this.WARNING_CLASSNAME)) {
            this.iconClassName = this.WARNING_CLASSNAME;
        }
    }
}

/**
 * Score in percents and className for view.
 */
export class Score {
    public label: string;
    public statusClassName: string;

    private readonly WARNING_MINIMUM_SCORE: number = 0.99;
    private readonly WARNING_CLASSNAME: string = 'warning';
    private readonly DISQUALIFICATION_MINIMUM_SCORE: number = 0.96;
    private readonly DISQUALIFICATION_CLASSNAME: string = 'disqualification';

    public constructor(
        score = 0,
    ) {
        this.label = `${parseFloat((score * 100).toFixed(2))} %`;

        switch (true) {
        case (score < this.DISQUALIFICATION_MINIMUM_SCORE):
            this.statusClassName = this.DISQUALIFICATION_CLASSNAME;

            break;
        case (score < this.WARNING_MINIMUM_SCORE):
            this.statusClassName = this.WARNING_CLASSNAME;

            break;
        default:
            this.statusClassName = '';
        }
    }
}

/**
 * SatelliteByDayInfo holds by day bandwidth metrics.
 */
export class SatelliteByDayInfo {
    public storageDaily: Stamp[];
    public bandwidthDaily: BandwidthUsed[];
    public egressDaily: EgressUsed[];
    public ingressDaily: IngressUsed[];

    public constructor(json: unknown) {
        const data = json as any; // eslint-disable-line @typescript-eslint/no-explicit-any
        const storageDailyJson = data.storageDaily || [];
        const bandwidthDailyJson = data.bandwidthDaily || [];

        this.storageDaily = storageDailyJson.map((stamp: any) => { // eslint-disable-line @typescript-eslint/no-explicit-any
            return new Stamp(stamp.atRestTotal, stamp.atRestTotalBytes, new Date(stamp.intervalStart));
        });

        this.bandwidthDaily = bandwidthDailyJson.map((bandwidth: any) => { // eslint-disable-line @typescript-eslint/no-explicit-any
            const egress = new Egress(bandwidth.egress.audit, bandwidth.egress.repair, bandwidth.egress.usage);
            const ingress = new Ingress(bandwidth.ingress.repair, bandwidth.ingress.usage);

            return new BandwidthUsed(egress, ingress, new Date(bandwidth.intervalStart));
        });

        this.egressDaily = bandwidthDailyJson.map((bandwidth: any) => { // eslint-disable-line @typescript-eslint/no-explicit-any
            const egress = new Egress(bandwidth.egress.audit, bandwidth.egress.repair, bandwidth.egress.usage);

            return new EgressUsed(egress, new Date(bandwidth.intervalStart));
        });

        this.ingressDaily = bandwidthDailyJson.map((bandwidth: any) => { // eslint-disable-line @typescript-eslint/no-explicit-any
            const ingress = new Ingress(bandwidth.ingress.repair, bandwidth.ingress.usage);

            return new IngressUsed(ingress, new Date(bandwidth.intervalStart));
        });
    }
}
