// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { StoreModule } from '@/app/store';
import {
    PayoutInfoRange,
    PayoutState,
} from '@/app/types/payout';
import { TB } from '@/app/utils/converter';
import { getHeldPercentage } from '@/app/utils/payout';
import {
    EstimatedPayout,
    PayoutPeriod,
    SatelliteHeldHistory,
    SatellitePayoutForPeriod,
    TotalHeldAndPaid,
    TotalPaystubForPeriod,
} from '@/storagenode/payouts/payouts';
import { PayoutService } from '@/storagenode/payouts/service';

export const PAYOUT_MUTATIONS = {
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
    GET_PAYOUT_INFO: 'GET_PAYOUT_INFO',
    SET_PERIODS_RANGE: 'SET_PERIODS_RANGE',
    GET_TOTAL: 'GET_TOTAL',
    GET_HELD_HISTORY: 'GET_HELD_HISTORY',
    GET_ESTIMATION: 'GET_ESTIMATION',
    GET_PERIODS: 'GET_PERIODS',
    GET_PAYOUT_HISTORY: 'GET_PAYOUT_HISTORY',
    SET_PAYOUT_HISTORY_PERIOD: 'SET_PAYOUT_HISTORY_PERIOD',
};

// TODO: move to config in storagenode/payouts
export const BANDWIDTH_DOWNLOAD_PRICE_PER_TB = 2000;
export const BANDWIDTH_REPAIR_PRICE_PER_TB = 1000;
export const DISK_SPACE_PRICE_PER_TB = 150;

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
            [PAYOUT_MUTATIONS.SET_TOTAL](state: PayoutState, totalHeldAndPaid: TotalHeldAndPaid): void {
                state.totalHeldAndPaid = totalHeldAndPaid;
                state.currentMonthEarnings = totalHeldAndPaid.currentMonthEarnings;
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
            [PAYOUT_ACTIONS.GET_PAYOUT_INFO]: async function ({ commit, state, rootState }: any, satelliteId: string = ''): Promise<void> {
                const totalPaystubForPeriod = await service.paystubSummaryForPeriod(
                    state.periodRange.start,
                    state.periodRange.end,
                    satelliteId,
                );

                commit(PAYOUT_MUTATIONS.SET_HELD_PERCENT, getHeldPercentage(rootState.node.selectedSatellite.joinDate));
                commit(PAYOUT_MUTATIONS.SET_PAYOUT_INFO, totalPaystubForPeriod);
            },
            [PAYOUT_ACTIONS.GET_TOTAL]: async function ({ commit, rootState }: any, satelliteId: string = ''): Promise<void> {
                const now = new Date();
                const start = new PayoutPeriod(rootState.node.selectedSatellite.joinDate.getUTCFullYear(), rootState.node.selectedSatellite.joinDate.getUTCMonth());
                const end = new PayoutPeriod(now.getUTCFullYear(), now.getUTCMonth());

                const totalHeldAndPaid = await service.totalHeldAndPaid(start, end, satelliteId);

                // TODO: move to service
                const currentBandwidthDownload = (rootState.node.egressChartData || [])
                    .map(data => data.egress.usage)
                    .reduce((previous, current) => previous + current, 0);

                const currentBandwidthAuditAndRepair = (rootState.node.egressChartData || [])
                    .map(data => data.egress.audit + data.egress.repair)
                    .reduce((previous, current) => previous + current, 0);

                const approxHourInMonth = 720;
                const currentDiskSpace = (rootState.node.storageChartData || [])
                    .map(data => data.atRestTotal)
                    .reduce((previous, current) => previous + current, 0) / approxHourInMonth;

                const thisMonthEarnings = (currentBandwidthDownload * BANDWIDTH_DOWNLOAD_PRICE_PER_TB
                    + currentBandwidthAuditAndRepair * BANDWIDTH_REPAIR_PRICE_PER_TB
                    + currentDiskSpace * DISK_SPACE_PRICE_PER_TB) / TB;

                totalHeldAndPaid.setCurrentMonthEarnings(thisMonthEarnings);

                commit(PAYOUT_MUTATIONS.SET_HELD_PERCENT, getHeldPercentage(rootState.node.selectedSatellite.joinDate));
                commit(PAYOUT_MUTATIONS.SET_TOTAL, totalHeldAndPaid);
            },
            [PAYOUT_ACTIONS.SET_PERIODS_RANGE]: function ({ commit }: any, periodRange: PayoutInfoRange): void {
                commit(PAYOUT_MUTATIONS.SET_RANGE, periodRange);
            },
            [PAYOUT_ACTIONS.GET_HELD_HISTORY]: async function ({ commit }: any): Promise<void> {
                const heldHistory = await service.allSatellitesHeldHistory();

                commit(PAYOUT_MUTATIONS.SET_HELD_HISTORY, heldHistory);
            },
            [PAYOUT_ACTIONS.GET_PERIODS]: async function ({commit}: any, satelliteId: string = ''): Promise<void> {
                const periods = await service.availablePeriods(satelliteId);

                commit(PAYOUT_MUTATIONS.SET_PERIODS, periods);

                if (!satelliteId) {
                    commit(PAYOUT_MUTATIONS.SET_PAYOUT_HISTORY_AVAILABLE_PERIODS, periods);
                }
            },
            [PAYOUT_ACTIONS.GET_ESTIMATION]: async function ({ commit }: any, satelliteId: string = ''): Promise<void> {
                const estimatedInfo = await service.estimatedPayout(satelliteId);

                commit(PAYOUT_MUTATIONS.SET_ESTIMATION, estimatedInfo);
            },
            [PAYOUT_ACTIONS.GET_PAYOUT_HISTORY]: async function ({ commit, state }: any): Promise<void> {
                const payoutHistory = await service.payoutHistory(state.payoutHistoryPeriod);

                commit(PAYOUT_MUTATIONS.SET_PAYOUT_HISTORY, payoutHistory);
            },
            [PAYOUT_ACTIONS.SET_PAYOUT_HISTORY_PERIOD]: function ({ commit }: any, period: string): void {
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
