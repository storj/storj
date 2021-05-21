// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

// TODO: move to config.
/**
 * Divider to convert payout amounts to cents.
 */
const PRICE_DIVIDER: number = 10000;

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
 * PayoutsSummary is a representation of current month estimated payout and undistributed payouts.
 */
export class Expectations {
    public constructor(
        public currentMonthEstimation: number = 0,
        public undistributed: number = 0,
    ) {
        this.currentMonthEstimation = this.convertToCents(this.currentMonthEstimation);
        this.undistributed = this.convertToCents(this.undistributed);
    }

    private convertToCents(value: number): number {
        return value / PRICE_DIVIDER;
    }
}
