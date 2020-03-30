// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import {
    HeldInfo,
    PaymentInfoParameters,
    PayoutApi,
    PayoutInfoRange, PayoutPeriod,
    PayoutState, TotalPayoutInfo,
} from '@/app/types/payout';

export const PAYOUT_MUTATIONS = {
    SET_HELD_INFO: 'SET_HELD_INFO',
    SET_RANGE: 'SET_RANGE',
    SET_TOTAL: 'SET_TOTAL',
};

export const PAYOUT_ACTIONS = {
    GET_HELD_INFO: 'GET_HELD_INFO',
    SET_PERIODS_RANGE: 'SET_PERIODS_RANGE',
    GET_TOTAL: 'GET_TOTAL',
};

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
        },
        actions: {
            [PAYOUT_ACTIONS.GET_HELD_INFO]: async function ({commit, state, rootState}: any, satelliteId: string = ''): Promise<void> {
                const heldInfo = await api.getHeldInfo(new PaymentInfoParameters(
                    state.periodRange.start,
                    state.periodRange.end,
                    satelliteId,
                ));

                heldInfo.surgePercent = getSurgePercentage(rootState.node.info.startedAt);

                commit(PAYOUT_MUTATIONS.SET_HELD_INFO, heldInfo);
            },
            [PAYOUT_ACTIONS.GET_TOTAL]: async function ({commit, rootState}: any, satelliteId: string = ''): Promise<void> {
                const now = new Date();
                const totalPayoutInfo = await api.getTotal(new PaymentInfoParameters(
                    new PayoutPeriod(rootState.node.info.startedAt.getUTCFullYear(), rootState.node.info.startedAt.getUTCMonth()),
                    new PayoutPeriod(now.getUTCFullYear(), now.getUTCMonth()),
                    satelliteId,
                ));

                commit(PAYOUT_MUTATIONS.SET_TOTAL, totalPayoutInfo);
            },
            [PAYOUT_ACTIONS.SET_PERIODS_RANGE]: function ({commit}: any, periodRange: PayoutInfoRange): void {
                commit(PAYOUT_MUTATIONS.SET_RANGE, periodRange);
            },
        },
        getters: {
            totalPeriodPayout: function (state: PayoutState): number {
                return state.heldInfo.compAtRest + state.heldInfo.compGet + state.heldInfo.compGetAudit
                        + state.heldInfo.compGetRepair - state.heldInfo.held;
            }
        }
    };
}

/**
 * Returns held percentage depends on number of months that node is online.
 * @param startedAt date since node is online.
 */
function getSurgePercentage(startedAt: Date): number {
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
