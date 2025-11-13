// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    EstimatedPayout,
    PayoutPeriod,
    SatelliteHeldHistory,
    SatellitePayoutForPeriod, SatellitePricingModel,
    TotalPayments,
    TotalPaystubForPeriod,
} from '@/storagenode/payouts/payouts';

/**
 * Holds 'start' and 'end' of payout period range.
 */
export class PayoutInfoRange {
    public constructor(
        public start: PayoutPeriod | null = null,
        public end: PayoutPeriod = new PayoutPeriod(),
    ) {}
}

/**
 * Holds all payout module state.
 */
export class PayoutState {
    public constructor (
        public totalPaystubForPeriod: TotalPaystubForPeriod = new TotalPaystubForPeriod(),
        public periodRange: PayoutInfoRange = new PayoutInfoRange(),
        public totalPayments: TotalPayments = new TotalPayments(),
        public currentMonthEarnings: number = 0,
        public heldPercentage: number = 0,
        public payoutPeriods: PayoutPeriod[] = [],
        public heldHistory: SatelliteHeldHistory[] = [],
        public payoutHistory: SatellitePayoutForPeriod[] = [],
        public payoutHistoryPeriod: string = '',
        public estimation: EstimatedPayout = new EstimatedPayout(),
        public payoutHistoryAvailablePeriods: PayoutPeriod[] = [],
        public pricingModel: SatellitePricingModel = new SatellitePricingModel(),
    ) {}
}

export interface StoredMonthsByYear {
    [key: number]: MonthButton[];
}

/**
 * Holds all months names.
 */
export const monthNames = [
    'January', 'February', 'March', 'April',
    'May', 'June', 'July', 'August',
    'September', 'October', 'November', 'December',
];

/**
 * Describes month button entity for calendar.
 */
export class MonthButton {
    public constructor(
        public year: number = 0,
        public index: number = 0,
        public active: boolean = false,
        public selected: boolean = false,
    ) {}

    /**
     * Returns month label depends on index.
     */
    public get name(): string {
        return monthNames[this.index] ? monthNames[this.index].slice(0, 3) : '';
    }
}
