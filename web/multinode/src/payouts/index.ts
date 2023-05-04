// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Divider to convert payout amounts to cents.
 */
const PRICE_DIVIDER = 10000;

/**
 * PayoutsSummary is a representation of summary of payout information for node.
 */
export class NodePayoutsSummary {
    public constructor(
        public nodeId: string = '',
        public nodeName: string = '',
        public held: number = 0,
        public paid: number = 0,
    ) {}

    /**
     * Returns node name for displaying.
     * If no provided returns id.
     */
    public get title(): string {
        return this.nodeName || this.nodeId;
    }
}

/**
 * PayoutsSummary is a representation of summary of payout information for all selected nodes and list of payout summaries by nodes.
 */
export class PayoutsSummary {
    public constructor(
        public totalEarned: number = 0,
        public totalHeld: number = 0,
        public totalPaid: number = 0,
        public nodeSummary: NodePayoutsSummary[] = [],
    ) {
        this.totalPaid = this.convertToCents(this.totalPaid);
        this.totalEarned = this.convertToCents(this.totalEarned);
        this.totalHeld = this.convertToCents(this.totalHeld);

        this.nodeSummary.forEach((summary: NodePayoutsSummary) => {
            summary.paid = this.convertToCents(summary.paid);
            summary.held = this.convertToCents(summary.held);
        });
    }

    private convertToCents(value: number): number {
        return value / PRICE_DIVIDER;
    }
}

/**
 * Represents payout period month and year.
 */
export class PayoutPeriod {
    public constructor(
        public year: number = new Date().getUTCFullYear(),
        public month: number = new Date().getUTCMonth(),
    ) {}

    public get period(): string {
        return this.month < 9 ? `${this.year}-0${this.month + 1}` : `${this.year}-${this.month + 1}`;
    }

    /**
     * Parses PayoutPeriod from string.
     * @param period string
     */
    public static fromString(period: string): PayoutPeriod {
        const periodArray = period.split('-');

        return new PayoutPeriod(parseInt(periodArray[0]), parseInt(periodArray[1]) - 1);
    }
}

/**
 * Contains all payout information of particular node.
 */
export class NodePayouts {
    public constructor(
        public totalEarned: number = 0,
        public totalHeld: number = 0,
        public totalPaid: number = 0,
        public heldHistory: HeldAmountSummary[] = [],
        public paystubForPeriod: Paystub = new Paystub(),
        public expectations: Expectation = new Expectation(),
    ) {}
}

/**
 * Holds held history information for all satellites.
 */
export class HeldAmountSummary {
    public constructor(
        public satelliteAddress: string = '',
        public firstQuarter: number = 0,
        public secondQuarter: number = 0,
        public thirdQuarter: number = 0,
        public periodCount: number = 0,
    ) {
        this.firstQuarter = this.convertToCents(this.firstQuarter);
        this.secondQuarter = this.convertToCents(this.secondQuarter);
        this.thirdQuarter = this.convertToCents(this.thirdQuarter);
    }

    /**
     * Returns node age depends on period count.
     */
    public get monthsCount(): string {
        return `${this.periodCount + 1} month${this.periodCount > 0 ? 's' : ''}`;
    }

    private convertToCents(value: number): number {
        return value / PRICE_DIVIDER;
    }
}

/**
 * Payouts month-term representation.
 */
export class Paystub {
    public gross = 0;

    public constructor(
        public usageAtRest: number = 0,
        public usageGet: number = 0,
        public usageGetRepair: number = 0,
        public usageGetAudit: number = 0,
        public compAtRest: number = 0,
        public compGet: number = 0,
        public compGetRepair: number = 0,
        public compGetAudit: number = 0,
        public held: number = 0,
        public paid: number = 0,
        public distributed: number = 0,
    ) {
        this.compAtRest = this.convertToCents(this.compAtRest);
        this.compGet = this.convertToCents(this.compGet);
        this.compGetRepair = this.convertToCents(this.compGetRepair);
        this.compGetAudit = this.convertToCents(this.compGetAudit);
        this.held = this.convertToCents(this.held);
        this.paid = this.convertToCents(this.paid);
        this.gross = this.convertToCents(this.paid + this.held);
        this.distributed = this.convertToCents(this.distributed);
    }

    public get repairAndAuditUsage(): number {
        return this.usageGetRepair + this.usageGetAudit;
    }

    public get repairAndAuditComp(): number {
        return this.compGetRepair + this.compGetAudit;
    }

    private convertToCents(value: number): number {
        return value / PRICE_DIVIDER;
    }
}

/**
 * Expectations is a representation of current month estimated payout and undistributed payouts.
 */
export class Expectation {
    public constructor(
        public currentMonthEstimation: number = 0,
        public undistributed: number = 0,
    ) {
        this.undistributed = this.convertToCents(this.undistributed);
    }

    private convertToCents(value: number): number {
        return value / PRICE_DIVIDER;
    }
}
