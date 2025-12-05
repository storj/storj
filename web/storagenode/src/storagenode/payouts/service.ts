// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    EstimatedPayout,
    PaymentInfoParameters,
    PayoutApi,
    PayoutPeriod,
    Paystub,
    SatelliteHeldHistory,
    SatellitePayoutForPeriod,
    SatellitePricingModel,
    TotalPayments,
    TotalPaystubForPeriod,
} from '@/storagenode/payouts/payouts';

/**
 * PayoutService is used to store and handle node paystub information.
 * PayoutService exposes a business logic related to payouts.
 */
export class PayoutService {
    private readonly payouts: PayoutApi;

    public constructor(api: PayoutApi) {
        this.payouts = api;
    }

    /**
     * Gets summary of paystubs for given period.
     * @param start period start
     * @param end period end
     * @param satelliteId
     */
    public async paystubSummaryForPeriod(start: PayoutPeriod | null, end: PayoutPeriod, satelliteId?: string): Promise<TotalPaystubForPeriod> {
        const paystubs: Paystub[] = await this.payouts.getPaystubsForPeriod(new PaymentInfoParameters(start, end, satelliteId));

        return new TotalPaystubForPeriod(paystubs);
    }

    /**
     * Gets held and paid summary for given period.
     * @param start period start
     * @param end period end
     * @param satelliteId
     */
    public async totalPayments(start: PayoutPeriod, end: PayoutPeriod, satelliteId: string): Promise<TotalPayments> {
        const paystubs: Paystub[] = await this.payouts.getPaystubsForPeriod(new PaymentInfoParameters(start, end, satelliteId));

        return new TotalPayments(paystubs);
    }

    /**
     * Gets list of payout periods that have paystubs for selected satellite.
     * If satelliteId is not provided returns periods for all satellites.
     * @param satelliteId
     */
    public async availablePeriods(satelliteId: string): Promise<PayoutPeriod[]> {
        return await this.payouts.getPayoutPeriods(satelliteId);
    }

    /**
     * Gets list of payout history items for given period by satellites.
     * @param payoutHistoryPeriod year and month representation
     */
    public async payoutHistory(payoutHistoryPeriod: string): Promise<SatellitePayoutForPeriod[]> {
        return await this.payouts.getPayoutHistory(payoutHistoryPeriod);
    }

    /**
     * Gets list of held history for all satellites.
     */
    public async allSatellitesHeldHistory(): Promise<SatelliteHeldHistory[]> {
        return await this.payouts.getHeldHistory();
    }

    /**
     * Gets estimated payout when no data in paystub.
     * @param satelliteId
     */
    public async estimatedPayout(satelliteId: string): Promise<EstimatedPayout> {
        return await this.payouts.getEstimatedPayout(satelliteId);
    }

    /**
     * Gets satellite pricing model.
     * @param satelliteId
     */
    public async pricingModel(satelliteId: string): Promise<SatellitePricingModel> {
        return await this.payouts.getPricingModel(satelliteId);
    }
}
