// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    HeldInfo,
    PaymentInfoParameters,
    PayoutApi,
    PayoutInfoRange,
    PayoutPeriod,
    PayoutState,
    TotalPayoutInfo,
} from '@/app/types/payout';
import { TB } from '@/app/utils/converter';

export const PAYOUT_MUTATIONS = {
    SET_HELD_INFO: 'SET_HELD_INFO',
    SET_RANGE: 'SET_RANGE',
    SET_TOTAL: 'SET_TOTAL',
    SET_HELD_PERCENT: 'SET_HELD_PERCENT',
};

export const PAYOUT_ACTIONS = {
    GET_HELD_INFO: 'GET_HELD_INFO',
    SET_PERIODS_RANGE: 'SET_PERIODS_RANGE',
    GET_TOTAL: 'GET_TOTAL',
};

export const BANDWIDTH_DOWNLOAD_PRICE_PER_TB = 2000;
export const BANDWIDTH_REPAIR_PRICE_PER_TB = 1000;
export const DISK_SPACE_PRICE_PER_TB = 150;

/**
 * creates notifications module with all dependencies
 *
 * @param api - payments api
 */
export function makePayoutModule(api: PayoutApi) {
    return {
        state: new PayoutState(),
        mutations: {
            [PAYOUT_MUTATIONS.SET_HELD_INFO](state: PayoutState, heldInfo: HeldInfo): void {
                state.heldInfo = heldInfo;
            },
            [PAYOUT_MUTATIONS.SET_TOTAL](state: PayoutState, totalPayoutInfo: TotalPayoutInfo): void {
                state.totalEarnings = totalPayoutInfo.totalEarnings;
                state.totalHeldAmount = totalPayoutInfo.totalHeldAmount;
            },
            [PAYOUT_MUTATIONS.SET_RANGE](state: PayoutState, periodRange: PayoutInfoRange): void {
                state.periodRange = periodRange;
            },
            [PAYOUT_MUTATIONS.SET_HELD_PERCENT](state: PayoutState, heldPercentage: number): void {
                state.heldPercentage = heldPercentage;
            },
        },
        actions: {
            [PAYOUT_ACTIONS.GET_HELD_INFO]: async function ({commit, state, rootState}: any, satelliteId: string = ''): Promise<void> {
                const heldInfo = state.periodRange.start ? await api.getHeldInfoByPeriod(new PaymentInfoParameters(
                    state.periodRange.start,
                    state.periodRange.end,
                    satelliteId,
                )) : await api.getHeldInfoByMonth(new PaymentInfoParameters(
                    null,
                    state.periodRange.end,
                    satelliteId,
                ));

                commit(PAYOUT_MUTATIONS.SET_HELD_PERCENT, getHeldPercentage(rootState.node.selectedSatellite.joinDate));
                commit(PAYOUT_MUTATIONS.SET_HELD_INFO, heldInfo);
            },
            [PAYOUT_ACTIONS.GET_TOTAL]: async function ({commit, rootState}: any, satelliteId: string = ''): Promise<void> {
                const now = new Date();
                const totalPayoutInfo = await api.getTotal(new PaymentInfoParameters(
                    new PayoutPeriod(rootState.node.selectedSatellite.joinDate.getUTCFullYear(), rootState.node.selectedSatellite.joinDate.getUTCMonth()),
                    new PayoutPeriod(now.getUTCFullYear(), now.getUTCMonth()),
                    satelliteId,
                ));

                const currentBandwidthDownload = (rootState.node.egressChartData || [])
                    .map(data => data.egress.usage)
                    .reduce((previous, current) => previous + current, 0);

                const currentBandwidthAuditAndRepair = (rootState.node.egressChartData || [])
                    .map(data => data.egress.audit + data.egress.repair)
                    .reduce((previous, current) => previous + current, 0);

                const approxHourInMonth = 730;
                const currentDiskSpace = (rootState.node.storageChartData || [])
                    .map(data => data.atRestTotal)
                    .reduce((previous, current) => previous + current, 0) / approxHourInMonth;

                const thisMonthEarnings = (currentBandwidthDownload * BANDWIDTH_DOWNLOAD_PRICE_PER_TB
                    + currentBandwidthAuditAndRepair * BANDWIDTH_REPAIR_PRICE_PER_TB
                    + currentDiskSpace * DISK_SPACE_PRICE_PER_TB) / TB;

                commit(PAYOUT_MUTATIONS.SET_HELD_PERCENT, getHeldPercentage(rootState.node.selectedSatellite.joinDate));
                commit(PAYOUT_MUTATIONS.SET_TOTAL, new TotalPayoutInfo(totalPayoutInfo.totalHeldAmount, thisMonthEarnings));
            },
            [PAYOUT_ACTIONS.SET_PERIODS_RANGE]: function ({commit}: any, periodRange: PayoutInfoRange): void {
                commit(PAYOUT_MUTATIONS.SET_RANGE, periodRange);
            },
        },
    };
}

/**
 * Returns held percentage depends on number of months that node is online.
 * @param startedAt date since node is online.
 */
export function getHeldPercentage(startedAt: Date): number {
    const now = new Date();
    const secondsInMonthApproximately = 2628000;
    const differenceInSeconds = (now.getTime() - startedAt.getTime()) / 1000;

    const monthsOnline = Math.ceil(differenceInSeconds / secondsInMonthApproximately);

    switch (true) {
        case monthsOnline < 4:
            return 75;
        case monthsOnline < 7:
            return 50;
        case monthsOnline < 10:
            return 25;
        default:
            return 0;
    }
}
