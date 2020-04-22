// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

/**
 * Holds request arguments for payout information.
 */
export class PaymentInfoParameters {
    public constructor(
        public start: PayoutPeriod | null = null,
        public end: PayoutPeriod = new PayoutPeriod(),
        public satelliteId: string = '',
    ) {}
}

/**
 * Holds payout information.
 */
export class HeldInfo {
    public constructor(
        public usageAtRest: number = 0,
        public usageGet: number = 0,
        public usagePut: number = 0,
        public usageGetRepair: number = 0,
        public usagePutRepair: number = 0,
        public usageGetAudit: number = 0,
        public compAtRest: number = 0,
        public compGet: number = 0,
        public compPut: number = 0,
        public compGetRepair: number = 0,
        public compPutRepair: number = 0,
        public compGetAudit: number = 0,
        public surgePercent: number = 0,
        public held: number = 0,
        public owed: number = 0,
        public disposed: number = 0,
        public paid: number = 0,
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
}

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
 * Holds accumulated held and earned payouts.
 */
export class TotalPayoutInfo {
    public constructor(
        public totalHeldAmount: number = 0,
        public totalEarnings: number = 0,
    ) {}
}

/**
 * Holds all payout module state.
 */
export class PayoutState {
    public constructor (
        public heldInfo: HeldInfo = new HeldInfo(),
        public periodRange: PayoutInfoRange = new PayoutInfoRange(),
        public totalHeldAmount: number = 0,
        public totalEarnings: number = 0,
        public heldPercentage: number = 0,
    ) {}
}

/**
 * Exposes all payout-related functionality.
 */
export interface PayoutApi {
    /**
     * Fetches held amount information by selected period.
     * @throws Error
     */
    getHeldInfoByPeriod(paymentInfoParameters: PaymentInfoParameters): Promise<HeldInfo>;

    /**
     * Fetches held amount information by selected month.
     * @throws Error
     */
    getHeldInfoByMonth(paymentInfoParameters: PaymentInfoParameters): Promise<HeldInfo>;

    /**
     * Fetches total payout information.
     * @throws Error
     */
    getTotal(paymentInfoParameters: PaymentInfoParameters): Promise<TotalPayoutInfo>;
}
