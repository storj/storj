// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * PayoutsSummary is a representation of summary of payout information for node.
 */
export class NodePayoutsSummary {
    public constructor(
        public nodeID: string = '',
        public nodeName: string = '',
        public held: number = 0,
        public paid: number = 0,
    ) {}
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
    ) {}
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
