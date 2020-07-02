// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    EstimatedPayout,
    HeldHistory,
    HeldHistoryAllStatItem,
    HeldHistoryMonthlyBreakdownItem,
    HeldInfo,
    PaymentInfoParameters,
    PayoutApi,
    PayoutPeriod,
    PreviousMonthEstimatedPayout,
    TotalPayoutInfo,
} from '@/app/types/payout';
import { HttpClient } from '@/storagenode/utils/httpClient';

/**
 * NotificationsHttpApi is a http implementation of Notifications API.
 * Exposes all notifications-related functionality
 */
export class PayoutHttpApi implements PayoutApi {
    private readonly client: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/api/heldamount';
    private PRICE_DIVIDER: number = 10000;

    /**
     * Fetch held amount information by selected period.
     *
     * @returns held amount information
     * @throws Error
     */
    public async getHeldInfoByPeriod(paymentInfoParameters: PaymentInfoParameters): Promise<HeldInfo> {
        let path = `${this.ROOT_PATH}/paystubs/`;

        if (paymentInfoParameters.start) {
            path += paymentInfoParameters.start.period + '/';
        }

        path += paymentInfoParameters.end.period;

        if (paymentInfoParameters.satelliteId) {
            path += '?id=' + paymentInfoParameters.satelliteId;
        }

        return await this.getHeld(path);
    }

    /**
     * Fetch held amount information by selected month.
     *
     * @returns held amount information
     * @throws Error
     */
    public async getHeldInfoByMonth(paymentInfoParameters: PaymentInfoParameters): Promise<HeldInfo> {
        let path = `${this.ROOT_PATH}/paystubs/`;

        path += paymentInfoParameters.end.period;

        if (paymentInfoParameters.satelliteId) {
            path += '?id=' + paymentInfoParameters.satelliteId;
        }

        return await this.getHeld(path);
    }

    /**
     * Fetch total payout information.
     *
     * @returns total payout information
     * @throws Error
     */
    public async getTotal(paymentInfoParameters: PaymentInfoParameters): Promise<TotalPayoutInfo> {
        let path = `${this.ROOT_PATH}/paystubs/`;

        if (paymentInfoParameters.start) {
            path += paymentInfoParameters.start.period + '/';
        }

        path += paymentInfoParameters.end.period;

        if (paymentInfoParameters.satelliteId) {
            path += '?id=' + paymentInfoParameters.satelliteId;
        }

        const response = await this.client.get(path);

        if (!response.ok) {
            throw new Error('can not get total payout information');
        }

        const data: any = await response.json() || [];

        if (!Array.isArray(data)) {
            return new TotalPayoutInfo(data.held, data.paid);
        }

        let held: number = 0;
        let paid: number = 0;

        data.forEach((paystub: any) => {
            held += (paystub.held - paystub.disposed) / this.PRICE_DIVIDER;
            paid += paystub.paid / this.PRICE_DIVIDER;
        });

        return new TotalPayoutInfo(
            held,
            paid,
            0,
        );
    }

    /**
     * Fetches available payout periods.
     *
     * @returns payout periods list
     * @throws Error
     */
    public async getPayoutPeriods(id: string): Promise<PayoutPeriod[]> {
        let path = `${this.ROOT_PATH}/periods`;

        if (id) {
            path += '?id=' + id;
        }

        const response = await this.client.get(path);

        if (!response.ok) {
            throw new Error('can not get payout periods');
        }

        const result = await response.json() || [];

        return result.map(period => {
            return PayoutPeriod.fromString(period);
        });
    }

    /**
     * Fetch total payout information.
     *
     * @returns total payout information
     * @throws Error
     */
    public async getHeldHistory(): Promise<HeldHistory> {
        const path = `${this.ROOT_PATH}/held-history/`;

        const response = await this.client.get(path);

        if (!response.ok) {
            throw new Error('can not get held history information');
        }

        const data: any = await response.json() || [];

        // TODO: this will be changed with adding 'all stats' held history.
        const monthlyBreakdown = data.map((historyItem: any) => {
            return new HeldHistoryMonthlyBreakdownItem(
                historyItem.satelliteID,
                historyItem.satelliteName,
                historyItem.age,
                historyItem.firstPeriod / this.PRICE_DIVIDER,
                historyItem.secondPeriod / this.PRICE_DIVIDER,
                historyItem.thirdPeriod / this.PRICE_DIVIDER,
            );
        });

        const allStats = data.map((historyItem: any) => {
           return new HeldHistoryAllStatItem(
               historyItem.satelliteID,
               historyItem.satelliteName,
               historyItem.age,
               historyItem.totalHeld,
               historyItem.totalDisposed,
               new Date(historyItem.joinedAt),
           );
        });

        return new HeldHistory(monthlyBreakdown, allStats);
    }

    /**
     * Fetch estimated payout information.
     *
     * @returns estimated payout information
     * @throws Error
     */
    public async getEstimatedInfo(satelliteId: string): Promise<EstimatedPayout> {
        let path = '/api/sno/estimated-payout';

        if (satelliteId) {
            path += '?id=' + satelliteId;
        }

        const response = await this.client.get(path);

        if (!response.ok) {
            throw new Error('can not get estimated payout information');
        }

        const data: any = await response.json() || [];

        return new EstimatedPayout(
            new PreviousMonthEstimatedPayout(
                data.currentMonth.egressBandwidth,
                data.currentMonth.egressBandwidthPayout,
                data.currentMonth.egressRepairAudit,
                data.currentMonth.egressRepairAuditPayout,
                data.currentMonth.diskSpace,
                data.currentMonth.diskSpacePayout,
                data.currentMonth.heldRate,
                data.currentMonth.payout,
                data.currentMonth.held,
            ),
            new PreviousMonthEstimatedPayout(
                data.previousMonth.egressBandwidth,
                data.previousMonth.egressBandwidthPayout,
                data.previousMonth.egressRepairAudit,
                data.previousMonth.egressRepairAuditPayout,
                data.previousMonth.diskSpace,
                data.previousMonth.diskSpacePayout,
                data.previousMonth.heldRate,
                data.previousMonth.payout,
                data.previousMonth.held,
            ),
        );
    }

    /**
     * Fetch total payout information depends on month.
     *
     * @returns total payout information
     * @throws Error
     */
    public async getHeld(path): Promise<HeldInfo> {
        const response = await this.client.get(path);

        if (!response.ok) {
            throw new Error('can not get held information');
        }

        const data: any[] = await response.json();

        if (!data || data.length === 0) {
            throw new Error('no payout data for selected period');
        }

        let usageAtRest: number = 0;
        let usageGet: number = 0;
        let usagePut: number = 0;
        let usageGetRepair: number = 0;
        let usagePutRepair: number = 0;
        let usageGetAudit: number = 0;
        let compAtRest: number = 0;
        let compGet: number = 0;
        let compPut: number = 0;
        let compGetRepair: number = 0;
        let compPutRepair: number = 0;
        let compGetAudit: number = 0;
        let held: number = 0;
        let owed: number = 0;
        let disposed: number = 0;
        let paid: number = 0;

        data.forEach((paystub: any) => {
            const surge = paystub.surgePercent === 0 ? 1 : paystub.surgePercent / 100;

            usageAtRest += paystub.usageAtRest;
            usageGet += paystub.usageGet;
            usagePut += paystub.usagePut;
            usageGetRepair += paystub.usageGetRepair;
            usagePutRepair += paystub.usagePutRepair;
            usageGetAudit += paystub.usageGetAudit;
            compAtRest += paystub.compAtRest / this.PRICE_DIVIDER * surge;
            compGet += paystub.compGet / this.PRICE_DIVIDER * surge;
            compPut += paystub.compPut / this.PRICE_DIVIDER;
            compGetRepair += paystub.compGetRepair / this.PRICE_DIVIDER * surge;
            compPutRepair += paystub.compPutRepair / this.PRICE_DIVIDER;
            compGetAudit += paystub.compGetAudit / this.PRICE_DIVIDER * surge;
            held += paystub.held / this.PRICE_DIVIDER;
            owed += paystub.owed / this.PRICE_DIVIDER;
            disposed += paystub.disposed / this.PRICE_DIVIDER;
            paid += paystub.paid / this.PRICE_DIVIDER;
        });

        return new HeldInfo(
            usageAtRest,
            usageGet,
            usagePut,
            usageGetRepair,
            usagePutRepair,
            usageGetAudit,
            compAtRest,
            compGet,
            compPut,
            compGetRepair,
            compPutRepair,
            compGetAudit,
            0,
            held,
            owed,
            disposed,
            paid,
        );
    }
}
