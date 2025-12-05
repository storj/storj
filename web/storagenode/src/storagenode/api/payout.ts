// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    EstimatedPayout,
    PaymentInfoParameters,
    PayoutApi,
    PayoutPeriod,
    Paystub,
    PreviousMonthEstimatedPayout,
    SatelliteHeldHistory,
    SatellitePayoutForPeriod, SatellitePricingModel,
} from '@/storagenode/payouts/payouts';
import { HttpClient } from '@/storagenode/utils/httpClient';

/**
 * PayoutHttpApi is a http implementation of Payout API.
 * Exposes all payout-related functionality
 */
export class PayoutHttpApi implements PayoutApi {
    private readonly client: HttpClient = new HttpClient();
    private readonly ROOT_PATH: string = '/api/heldamount';

    /**
     * Fetch paystubs for selected period.
     *
     * @returns paystubs for given period
     * @throws Error
     */
    public async getPaystubsForPeriod(paymentInfoParameters: PaymentInfoParameters): Promise<Paystub[]> {
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
            throw new Error('can not get held information');
        }

        const responseBody = await response.json() || [];
        const data: any[] = !Array.isArray(responseBody) ? [ responseBody ] : responseBody; // eslint-disable-line @typescript-eslint/no-explicit-any

        return data.map((paystubJson: any) => { // eslint-disable-line @typescript-eslint/no-explicit-any
            return new Paystub(
                paystubJson.usageAtRest,
                paystubJson.usageGet,
                paystubJson.usagePut,
                paystubJson.usageGetRepair,
                paystubJson.usagePutRepair,
                paystubJson.usageGetAudit,
                paystubJson.compAtRest,
                paystubJson.compGet,
                paystubJson.compPut,
                paystubJson.compGetRepair,
                paystubJson.compPutRepair,
                paystubJson.compGetAudit,
                paystubJson.surgePercent,
                paystubJson.held,
                paystubJson.owed,
                paystubJson.disposed,
                paystubJson.paid,
                paystubJson.distributed,
            );
        });
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
     * Fetch payout history for given period.
     *
     * @returns payout information
     * @throws Error
     */
    public async getPayoutHistory(period: string): Promise<SatellitePayoutForPeriod[]> {
        const path = `${this.ROOT_PATH}/payout-history/${period}`;

        const response = await this.client.get(path);

        if (!response.ok) {
            throw new Error('can not get payout history information');
        }

        const data: any = await response.json() || []; // eslint-disable-line @typescript-eslint/no-explicit-any

        return data.map((payoutHistoryItem: any) => { // eslint-disable-line @typescript-eslint/no-explicit-any
            return new SatellitePayoutForPeriod(
                payoutHistoryItem.satelliteID,
                payoutHistoryItem.satelliteURL,
                payoutHistoryItem.age,
                payoutHistoryItem.earned,
                payoutHistoryItem.surge,
                payoutHistoryItem.surgePercent,
                payoutHistoryItem.held,
                payoutHistoryItem.afterHeld,
                payoutHistoryItem.disposed,
                payoutHistoryItem.paid,
                payoutHistoryItem.receipt,
                payoutHistoryItem.isExitComplete,
                payoutHistoryItem.heldPercent,
                payoutHistoryItem.distributed,
            );
        });
    }

    /**
     * Fetch total payout information.
     *
     * @returns total payout information
     * @throws Error
     */
    public async getHeldHistory(): Promise<SatelliteHeldHistory[]> {
        const path = `${this.ROOT_PATH}/held-history`;

        const response = await this.client.get(path);

        if (!response.ok) {
            throw new Error('can not get held history information');
        }

        const data: any = await response.json() || []; // eslint-disable-line @typescript-eslint/no-explicit-any

        return data.map((historyItem: any) => { // eslint-disable-line @typescript-eslint/no-explicit-any
            return new SatelliteHeldHistory(
                historyItem.satelliteID,
                historyItem.satelliteName,
                historyItem.holdForFirstPeriod,
                historyItem.holdForSecondPeriod,
                historyItem.holdForThirdPeriod,
                historyItem.totalHeld,
                historyItem.totalDisposed,
                new Date(historyItem.joinedAt),
            );
        });
    }

    /**
     * Fetch estimated payout information.
     *
     * @returns estimated payout information
     * @throws Error
     */
    public async getEstimatedPayout(satelliteId: string): Promise<EstimatedPayout> {
        let path = '/api/sno/estimated-payout';

        if (satelliteId) {
            path += '?id=' + satelliteId;
        }

        const response = await this.client.get(path);

        if (!response.ok) {
            throw new Error('can not get estimated payout information');
        }

        const data: any = await response.json() || new EstimatedPayout(); // eslint-disable-line @typescript-eslint/no-explicit-any

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
            data.currentMonthExpectations,
        );
    }

    public async getPricingModel(satelliteId: string): Promise<SatellitePricingModel> {
        if (!satelliteId) {
            return new SatellitePricingModel();
        }

        const path = '/api/sno/satellites/'+ satelliteId +'/pricing';

        const response = await this.client.get(path);

        if (!response.ok) {
            throw new Error('can not get satellite pricing information');
        }

        const data: any = await response.json() || new SatellitePricingModel(); // eslint-disable-line @typescript-eslint/no-explicit-any

        return new SatellitePricingModel(
            data.satelliteID,
            data.egressBandwidth,
            data.repairBandwidth,
            data.auditBandwidth,
            data.diskSpace,
        );
    }
}
