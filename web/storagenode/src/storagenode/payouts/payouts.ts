// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { getMonthsBeforeNow } from '@/app/utils/payout';

/**
 * Exposes all payout-related functionality.
 */
export interface PayoutApi {
    /**
     * Fetches paystubs by selected period.
     * @throws Error
     */
    getPaystubsForPeriod(paymentInfoParameters: PaymentInfoParameters): Promise<Paystub[]>;

    /**
     * Fetches available payout periods.
     * @throws Error
     */
    getPayoutPeriods(id: string): Promise<PayoutPeriod[]>;

    /**
     * Fetches held history for all satellites.
     * @throws Error
     */
    getHeldHistory(): Promise<SatelliteHeldHistory[]>;

    /**
     * Fetch estimated payout information.
     * @throws Error
     */
    getEstimatedPayout(satelliteId: string): Promise<EstimatedPayout>;

    /**
     * Fetch satellite payout rate.
     * @throws Error
     */
    getPricingModel(satelliteId: string): Promise<SatellitePricingModel>;

    /**
     * Fetches payout history for all satellites.
     * @throws Error
     */
    getPayoutHistory(period: string): Promise<SatellitePayoutForPeriod[]>;
}

// TODO: move to config.
/**
 * Divider to convert payout amounts to cents.
 */
const PRICE_DIVIDER = 10000;

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
 * PayStub is an entity that holds usage and cash amounts that will be paid to storagenode operator after for month period.
 */
export class Paystub {
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
        public distributed: number = 0,
    ) {}

    /**
     * Returns payout amount multiplier.
     */
    public get surgeMultiplier(): number {
        // 0 in backend uses instead of "without multiplier"
        return this.surgePercent === 0 ? 1 : this.surgePercent / 100;
    }
}

/**
 * Summary of paystubs by period.
 * Payout amounts converted to cents.
 */
export class TotalPaystubForPeriod {
    public usageAtRest = 0;
    public usageGet = 0;
    public usagePut = 0;
    public usageGetRepair = 0;
    public usagePutRepair = 0;
    public usageGetAudit = 0;
    public compAtRest = 0;
    public compGet = 0;
    public compPut = 0;
    public compGetRepair = 0;
    public compPutRepair = 0;
    public compGetAudit = 0;
    public surgePercent = 0;
    public held = 0;
    public owed = 0;
    public disposed = 0;
    public paid = 0;
    public paidWithoutSurge = 0;
    public grossWithSurge = 0;
    public distributed = 0;

    public constructor(
        paystubs: Paystub[] = [],
    ) {
        paystubs.forEach(paystub => {
            this.usageAtRest += paystub.usageAtRest;
            this.usageGet += paystub.usageGet;
            this.usagePut += paystub.usagePut;
            this.usageGetRepair += paystub.usageGetRepair;
            this.usagePutRepair += paystub.usagePutRepair;
            this.usageGetAudit += paystub.usageGetAudit;
            this.compAtRest += this.convertToCents(paystub.compAtRest);
            this.compGet += this.convertToCents(paystub.compGet);
            this.compPut += this.convertToCents(paystub.compPut);
            this.compGetRepair += this.convertToCents(paystub.compGetRepair);
            this.compPutRepair += this.convertToCents(paystub.compPutRepair);
            this.compGetAudit += this.convertToCents(paystub.compGetAudit);
            this.held += this.convertToCents(paystub.held);
            this.owed += this.convertToCents(paystub.owed);
            this.disposed += this.convertToCents(paystub.disposed);
            this.paid += this.convertToCents(paystub.paid);
            this.surgePercent = paystub.surgePercent;
            this.paidWithoutSurge += this.convertToCents(paystub.paid + paystub.held - paystub.disposed) / paystub.surgeMultiplier;
            this.grossWithSurge += this.convertToCents(paystub.paid + paystub.held - paystub.disposed);
            this.distributed += this.convertToCents(paystub.distributed);
        });
    }

    private convertToCents(value: number): number {
        return value / PRICE_DIVIDER;
    }
}

/**
 * Holds accumulated held and earned payouts.
 */
export class TotalPayments {
    public held = 0;
    public paid = 0;
    public disposed = 0;
    public balance = 0;

    public constructor(
        paystubs: Paystub[] = [],
    ) {
        paystubs.forEach(paystub => {
            this.paid += this.convertToCents(paystub.paid);
            this.disposed += this.convertToCents(paystub.disposed);
            this.held += this.convertToCents(paystub.held - paystub.disposed);
            this.balance += this.convertToCents(paystub.paid - paystub.distributed);
        });
    }

    private convertToCents(value: number): number {
        return value / PRICE_DIVIDER;
    }
}

/**
 * Holds held history information for all satellites.
 */
export class SatelliteHeldHistory {
    public constructor(
        public satelliteID: string = '',
        public satelliteName: string = '',
        public holdForFirstPeriod: number = 0,
        public holdForSecondPeriod: number = 0,
        public holdForThirdPeriod: number = 0,
        public totalHeld: number = 0,
        public totalDisposed: number = 0,
        public joinedAt: Date = new Date(),
    ) {
        this.totalHeld = this.convertToCents(this.totalHeld - this.totalDisposed);
        this.totalDisposed = this.convertToCents(this.totalDisposed);
        this.holdForFirstPeriod = this.convertToCents(this.holdForFirstPeriod);
        this.holdForSecondPeriod = this.convertToCents(this.holdForSecondPeriod);
        this.holdForThirdPeriod = this.convertToCents(this.holdForThirdPeriod);
    }

    public get monthsWithNode(): number {
        return getMonthsBeforeNow(this.joinedAt);
    }

    private convertToCents(value: number): number {
        return value / PRICE_DIVIDER;
    }
}

/**
 * Contains estimated payout information for current and last periods.
 */
export class EstimatedPayout {
    public constructor(
        public currentMonth: PreviousMonthEstimatedPayout = new PreviousMonthEstimatedPayout(),
        public previousMonth: PreviousMonthEstimatedPayout = new PreviousMonthEstimatedPayout(),
        public currentMonthExpectations: number = 0,
    ) {}
}

/**
 * Contains last month estimated payout information.
 */
export class PreviousMonthEstimatedPayout {
    public constructor(
        public egressBandwidth: number = 0,
        public egressBandwidthPayout: number = 0,
        public egressRepairAudit: number = 0,
        public egressRepairAuditPayout: number = 0,
        public diskSpace: number = 0,
        public diskSpacePayout: number = 0,
        public heldRate: number = 0,
        public payout: number = 0,
        public held: number = 0,
    ) {}
}

/**
 * Contains payout information for payout history table.
 */
export class SatellitePayoutForPeriod {
    public constructor(
        public satelliteID: string = '',
        public satelliteName: string = '',
        public age: number = 1,
        public earned: number = 0,
        public surge: number = 0,
        public surgePercent: number = 0,
        public held: number = 0,
        public afterHeld: number = 0,
        public disposed: number = 0,
        public paid: number = 0,
        public receipt: string = '',
        public isExitComplete: boolean = false,
        public heldPercent: number = 0,
        public distributed: number = 0,
    ) {
        this.earned = this.convertToCents(this.earned);
        this.surge = this.convertToCents(this.surge);
        this.held = this.convertToCents(this.held);
        this.afterHeld = this.convertToCents(this.afterHeld);
        this.disposed = this.convertToCents(this.disposed);
        this.paid = this.convertToCents(this.paid);
        this.distributed = this.convertToCents(this.distributed);
    }

    public get transactionLink(): string {
        const prefixed = function (hash: string): string {
            if (hash.indexOf('0x') !== 0) {
                return '0x' + hash;
            }
            return hash;
        };
        if (!this.receipt) {
            return '';
        }

        if (this.receipt.indexOf('eth') !== -1) {
            return `https://etherscan.io/tx/${prefixed(this.receipt.slice(4))}`;
        }

        {
            const zkScanUrl = 'https://zkscan.io/explorer/transactions';

            if (this.receipt.indexOf('zksync-era') !== -1) {
                return `https://explorer.zksync.io/tx/${prefixed(this.receipt.slice(11))}`;
            } else if (this.receipt.indexOf('zksync') !== -1) {
                return `${zkScanUrl}/${prefixed(this.receipt.slice(7))}`;
            } else if (this.receipt.indexOf('zkwithdraw') !== -1) {
                return `${zkScanUrl}/${prefixed(this.receipt.slice(11))}`;
            }
        }

        if (this.receipt.indexOf('polygon') !== -1) {
            return `https://polygonscan.com/tx/${prefixed(this.receipt.slice(8))}`;
        }

        return this.receipt;
    }

    private convertToCents(value: number): number {
        return value / PRICE_DIVIDER;
    }
}

/**
 * Contains satellite payout rates.
 */
export class SatellitePricingModel {
    public constructor(
        public satelliteID: string = '',
        public egressBandwidth: number = 0,
        public repairBandwidth: number = 0,
        public auditBandwidth: number = 0,
        public diskSpace: number = 0,
    ) {
        this.egressBandwidth = this.egressBandwidth / 100;
        this.repairBandwidth = this.repairBandwidth / 100;
        this.auditBandwidth = this.auditBandwidth / 100;
        this.diskSpace = this.diskSpace / 100;
    }
}
