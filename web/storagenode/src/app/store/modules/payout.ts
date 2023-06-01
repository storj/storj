// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { StoreModule } from '@/app/store';
import {
    PayoutInfoRange,
    PayoutState,
} from '@/app/types/payout';
import { StorageNodeState } from '@/app/types/sno';
import { getHeldPercentage } from '@/app/utils/payout';
import {
    EstimatedPayout,
    PayoutPeriod,
    SatelliteHeldHistory,
    SatellitePayoutForPeriod, SatellitePricingModel,
    TotalPayments,
    TotalPaystubForPeriod,
} from '@/storagenode/payouts/payouts';
import { PayoutService } from '@/storagenode/payouts/service';

export const PAYOUT_MUTATIONS = {
    SET_PRICING_MODEL: 'SET_PRICING_MODEL',
    SET_PAYOUT_INFO: 'SET_PAYOUT_INFO',
    SET_RANGE: 'SET_RANGE',
    SET_TOTAL: 'SET_TOTAL',
    SET_HELD_PERCENT: 'SET_HELD_PERCENT',
    SET_HELD_HISTORY: 'SET_HELD_HISTORY',
    SET_ESTIMATION: 'SET_ESTIMATION',
    SET_PERIODS: 'SET_PERIODS',
    SET_PAYOUT_HISTORY: 'SET_PAYOUT_HISTORY',
    SET_PAYOUT_HISTORY_PERIOD: 'SET_PAYOUT_HISTORY_PERIOD',
    SET_PAYOUT_HISTORY_AVAILABLE_PERIODS: 'SET_PAYOUT_HISTORY_AVAILABLE_PERIODS',
};

export const PAYOUT_ACTIONS = {
    GET_PRICING_MODEL: 'GET_PRICING_MODEL',
    GET_PAYOUT_INFO: 'GET_PAYOUT_INFO',
    SET_PERIODS_RANGE: 'SET_PERIODS_RANGE',
    GET_TOTAL: 'GET_TOTAL',
    GET_HELD_HISTORY: 'GET_HELD_HISTORY',
    GET_ESTIMATION: 'GET_ESTIMATION',
    GET_PERIODS: 'GET_PERIODS',
    GET_PAYOUT_HISTORY: 'GET_PAYOUT_HISTORY',
    SET_PAYOUT_HISTORY_PERIOD: 'SET_PAYOUT_HISTORY_PERIOD',
};

interface PayoutContext {
    rootState: {
        node: StorageNodeState;
    };
    state: PayoutState;
    commit: (string, ...unknown) => void;
}

/**
 * creates notifications module with all dependencies
 *
 * @param service - payments service
 */
export function newPayoutModule(service: PayoutService): StoreModule<PayoutState> {
    return {
        state: new PayoutState(),
        mutations: {
            [PAYOUT_MUTATIONS.SET_PAYOUT_INFO](state: PayoutState, totalPaystubForPeriod: TotalPaystubForPeriod): void {
                state.totalPaystubForPeriod = totalPaystubForPeriod;
            },
            [PAYOUT_MUTATIONS.SET_TOTAL](state: PayoutState, totalPayments: TotalPayments): void {
                state.totalPayments = totalPayments;
            },
            [PAYOUT_MUTATIONS.SET_RANGE](state: PayoutState, periodRange: PayoutInfoRange): void {
                state.periodRange = periodRange;
            },
            [PAYOUT_MUTATIONS.SET_HELD_PERCENT](state: PayoutState, heldPercentage: number): void {
                state.heldPercentage = heldPercentage;
            },
            [PAYOUT_MUTATIONS.SET_HELD_HISTORY](state: PayoutState, heldHistory: SatelliteHeldHistory[]): void {
                state.heldHistory = heldHistory;
            },
            [PAYOUT_MUTATIONS.SET_ESTIMATION](state: PayoutState, estimatedInfo: EstimatedPayout): void {
                state.estimation = estimatedInfo;
                state.currentMonthEarnings = estimatedInfo.currentMonth.payout + estimatedInfo.currentMonth.held;
            },
            [PAYOUT_MUTATIONS.SET_PRICING_MODEL](state: PayoutState, pricing: SatellitePricingModel): void {
                state.pricingModel = pricing;
            },
            [PAYOUT_MUTATIONS.SET_PERIODS](state: PayoutState, periods: PayoutPeriod[]): void {
                state.payoutPeriods = periods;
            },
            [PAYOUT_MUTATIONS.SET_PAYOUT_HISTORY](state: PayoutState, payoutHistory: SatellitePayoutForPeriod[]): void {
                state.payoutHistory = payoutHistory;
            },
            [PAYOUT_MUTATIONS.SET_PAYOUT_HISTORY_PERIOD](state: PayoutState, period: string): void {
                state.payoutHistoryPeriod = period;
            },
            [PAYOUT_MUTATIONS.SET_PAYOUT_HISTORY_AVAILABLE_PERIODS](state: PayoutState, periods: PayoutPeriod[]): void {
                state.payoutHistoryAvailablePeriods = periods;
            },
        },
        actions: {
            [PAYOUT_ACTIONS.GET_PAYOUT_INFO]: async function ({ commit, state, rootState }: PayoutContext, satelliteId = ''): Promise<void> {
                const totalPaystubForPeriod = await service.paystubSummaryForPeriod(
                    state.periodRange.start,
                    state.periodRange.end,
                    satelliteId,
                );

                commit(PAYOUT_MUTATIONS.SET_HELD_PERCENT, getHeldPercentage(rootState.node.selectedSatellite.joinDate));
                commit(PAYOUT_MUTATIONS.SET_PAYOUT_INFO, totalPaystubForPeriod);
            },
            [PAYOUT_ACTIONS.GET_TOTAL]: async function ({ commit, rootState }: PayoutContext, satelliteId = ''): Promise<void> {
                const now = new Date();
                const start = new PayoutPeriod(rootState.node.selectedSatellite.joinDate.getUTCFullYear(), rootState.node.selectedSatellite.joinDate.getUTCMonth());
                const end = new PayoutPeriod(now.getUTCFullYear(), now.getUTCMonth());

                const totalPayments = await service.totalPayments(start, end, satelliteId);

                commit(PAYOUT_MUTATIONS.SET_HELD_PERCENT, getHeldPercentage(rootState.node.selectedSatellite.joinDate));
                commit(PAYOUT_MUTATIONS.SET_TOTAL, totalPayments);
            },
            [PAYOUT_ACTIONS.SET_PERIODS_RANGE]: function ({ commit }: PayoutContext, periodRange: PayoutInfoRange): void {
                commit(PAYOUT_MUTATIONS.SET_RANGE, periodRange);
            },
            [PAYOUT_ACTIONS.GET_HELD_HISTORY]: async function ({ commit }: PayoutContext): Promise<void> {
                const heldHistory = await service.allSatellitesHeldHistory();

                commit(PAYOUT_MUTATIONS.SET_HELD_HISTORY, heldHistory);
            },
            [PAYOUT_ACTIONS.GET_PERIODS]: async function ({ commit }: PayoutContext, satelliteId = ''): Promise<void> {
                const periods = await service.availablePeriods(satelliteId);

                commit(PAYOUT_MUTATIONS.SET_PERIODS, periods);

                if (!satelliteId) {
                    commit(PAYOUT_MUTATIONS.SET_PAYOUT_HISTORY_AVAILABLE_PERIODS, periods);
                }
            },
            [PAYOUT_ACTIONS.GET_ESTIMATION]: async function ({ commit }: PayoutContext, satelliteId = ''): Promise<void> {
                const estimatedInfo = await service.estimatedPayout(satelliteId);

                commit(PAYOUT_MUTATIONS.SET_ESTIMATION, estimatedInfo);
            },
            [PAYOUT_ACTIONS.GET_PRICING_MODEL]: async function ({ commit }: PayoutContext, satelliteId): Promise<void> {
                const pricing = await service.pricingModel(satelliteId);

                commit(PAYOUT_MUTATIONS.SET_PRICING_MODEL, pricing);
            },
            [PAYOUT_ACTIONS.GET_PAYOUT_HISTORY]: async function ({ commit, state }: PayoutContext): Promise<void> {
                if (!state.payoutHistoryPeriod) return;

                const payoutHistory = await service.payoutHistory(state.payoutHistoryPeriod);

                commit(PAYOUT_MUTATIONS.SET_PAYOUT_HISTORY, payoutHistory);
            },
            [PAYOUT_ACTIONS.SET_PAYOUT_HISTORY_PERIOD]: function ({ commit }: PayoutContext, period: string): void {
                commit(PAYOUT_MUTATIONS.SET_PAYOUT_HISTORY_PERIOD, period);
            },
        },
        getters: {
            totalPaidForPayoutHistoryPeriod: (state: PayoutState): number => {
                return state.payoutHistory.map(data => data.paid)
                    .reduce((previous, current) => previous + current, 0);
            },
        },
    };
}
