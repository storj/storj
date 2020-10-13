// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

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
        public bandwidthSummary: number = 0,
        public egressSummary: number = 0,
        public ingressSummary: number = 0,
        public audit: Metric = new Metric(),
        public uptime: Metric = new Metric(),
        public joinDate: Date = new Date(),
    ) {}
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
        public bandwidthSummary: number = 0,
        public egressSummary: number = 0,
        public ingressSummary: number = 0,
        public joinDate: Date = new Date(),
        public satellitesScores: SatelliteScores[] = [],
    ) {}
}

// TODO: move and create domain types.
/**
 * Holds information about audit and suspension scores by satellite.
 */
export class SatelliteScores {
    public auditScore: Score;
    public suspensionScore: Score;
    public onlineScore: Score;
    public iconClassName: string = '';

    private readonly WARNING_CLASSNAME: string = 'warning';
    private readonly DISQUALIFICATION_CLASSNAME: string = 'disqualification';

    public constructor(
        public satelliteName: string = 'satellite-name',
        auditScore: number = 0,
        unknownScore: number = 0,
        onlineScore: number = 0,
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

    private readonly WARNING_MINIMUM_SCORE: number = 0.95;
    private readonly WARNING_CLASSNAME: string = 'warning';
    private readonly DISQUALIFICATION_MINIMUM_SCORE: number = 0.6;
    private readonly DISQUALIFICATION_CLASSNAME: string = 'disqualification';

    public constructor(
        score: number = 0,
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
