// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import { makeNodeModule } from '@/app/store/modules/node';
import { getHeldPercentage, makePayoutModule, PAYOUT_ACTIONS, PAYOUT_MUTATIONS } from '@/app/store/modules/payout';
import { HeldInfo, PayoutInfoRange, PayoutPeriod, TotalPayoutInfo } from '@/app/types/payout';
import { PayoutHttpApi } from '@/storagenode/api/payout';
import { SNOApi } from '@/storagenode/api/storagenode';
import { createLocalVue } from '@vue/test-utils';

const Vue = createLocalVue();
const payoutApi = new PayoutHttpApi();
const payoutModule = makePayoutModule(payoutApi);

const nodeApi = new SNOApi();
const nodeModule = makeNodeModule(nodeApi);

Vue.use(Vuex);

const store = new Vuex.Store({ modules: { payoutModule, node: nodeModule } });

const state = store.state as any;

describe('mutations', () => {
    beforeEach(() => {
        createLocalVue().use(Vuex);
    });

    it('sets held information', () => {
        const heldInfo = new HeldInfo(13, 12, 11);

        store.commit(PAYOUT_MUTATIONS.SET_HELD_INFO, heldInfo);

        expect(state.payoutModule.heldInfo.usageAtRest).toBe(13);
        expect(state.payoutModule.heldInfo.usageGet).toBe(12);
        expect(state.payoutModule.heldInfo.usagePut).toBe(11);
    });

    it('sets total payout information', () => {
        const totalInfo = new TotalPayoutInfo(50, 100);

        store.commit(PAYOUT_MUTATIONS.SET_TOTAL, totalInfo);

        expect(state.payoutModule.totalHeldAmount).toBe(50);
        expect(state.payoutModule.totalEarnings).toBe(100);
    });

    it('sets period range', () => {
        const range = new PayoutInfoRange(new PayoutPeriod(2019, 2), new PayoutPeriod(2020, 3));

        store.commit(PAYOUT_MUTATIONS.SET_RANGE, range);

        if (!state.payoutModule.periodRange.start) {
            fail('periodRange.start is null');
        }

        expect(state.payoutModule.periodRange.start.period).toBe('2019-03');
        expect(state.payoutModule.periodRange.end.period).toBe('2020-04');
    });

    it('sets held percentage', () => {
        const expectedHeldPercentage = 75;

        store.commit(PAYOUT_MUTATIONS.SET_HELD_PERCENT, expectedHeldPercentage);

        expect(state.payoutModule.heldPercentage).toBe(expectedHeldPercentage);
    });
});

describe('actions', () => {
    beforeEach(() => {
        jest.resetAllMocks();
    });

    it('success get held info by month', async () => {
        jest.spyOn(payoutApi, 'getHeldInfoByMonth').mockReturnValue(
            Promise.resolve(new HeldInfo(1, 2 , 3, 4, 5)),
        );

        const range = new PayoutInfoRange(null, new PayoutPeriod(2020, 3));

        store.commit(PAYOUT_MUTATIONS.SET_RANGE, range);

        await store.dispatch(PAYOUT_ACTIONS.GET_HELD_INFO);

        expect(state.payoutModule.heldInfo.usagePut).toBe(3);
        expect(state.payoutModule.heldInfo.held).toBe(0);
        expect(state.payoutModule.heldPercentage).toBe(getHeldPercentage(new Date()));
    });

    it('get held info by month throws an error when api call fails', async () => {
        jest.spyOn(payoutApi, 'getHeldInfoByMonth').mockImplementation(() => { throw new Error(); });

        try {
            await store.dispatch(PAYOUT_ACTIONS.GET_HELD_INFO);
            expect(true).toBe(false);
        } catch (error) {
            expect(state.payoutModule.heldInfo.usagePut).toBe(3);
            expect(state.payoutModule.heldInfo.held).toBe(0);
        }
    });

    it('success get held info by period', async () => {
        jest.spyOn(payoutApi, 'getHeldInfoByPeriod').mockReturnValue(
            Promise.resolve(new HeldInfo(1, 2 , 3, 4, 5)),
        );

        const range = new PayoutInfoRange(new PayoutPeriod(2019, 2), new PayoutPeriod(2020, 3));

        store.commit(PAYOUT_MUTATIONS.SET_RANGE, range);

        await store.dispatch(PAYOUT_ACTIONS.GET_HELD_INFO);

        expect(state.payoutModule.heldInfo.usagePut).toBe(3);
        expect(state.payoutModule.heldInfo.held).toBe(0);
    });

    it('get held info by period throws an error when api call fails', async () => {
        jest.spyOn(payoutApi, 'getHeldInfoByPeriod').mockImplementation(() => { throw new Error(); });

        try {
            await store.dispatch(PAYOUT_ACTIONS.GET_HELD_INFO);
            expect(true).toBe(false);
        } catch (error) {
            expect(state.payoutModule.heldInfo.usagePut).toBe(3);
            expect(state.payoutModule.heldInfo.held).toBe(0);
        }
    });

    it('success get total', async () => {
        jest.spyOn(payoutApi, 'getTotal').mockReturnValue(
            Promise.resolve(new TotalPayoutInfo(10, 20)),
        );

        await store.dispatch(PAYOUT_ACTIONS.GET_TOTAL);

        expect(state.payoutModule.totalHeldAmount).toBe(10);
        expect(state.payoutModule.totalEarnings).toBe(0);
    });

    it('get total throws an error when api call fails', async () => {
        jest.spyOn(payoutApi, 'getTotal').mockImplementation(() => { throw new Error(); });

        try {
            await store.dispatch(PAYOUT_ACTIONS.GET_TOTAL);
            expect(true).toBe(false);
        } catch (error) {
            expect(state.payoutModule.totalHeldAmount).toBe(10);
            expect(state.payoutModule.totalEarnings).toBe(0);
        }
    });

    it('success sets period range', async () => {
        await store.dispatch(
            PAYOUT_ACTIONS.SET_PERIODS_RANGE,
            new PayoutInfoRange(
                new PayoutPeriod(2020, 1),
                new PayoutPeriod(2020, 2),
            ),
        );

        expect(state.payoutModule.periodRange.start.period).toBe('2020-02');
        expect(state.payoutModule.periodRange.end.period).toBe('2020-03');
    });
});

describe('utils functions', () => {
    it('get correct help percentage', () => {
        const nowTime = new Date().getTime();
        const testDifferencesInMilliseconds: number[] = [5e9, 1.4e10, 2.3e10, 4e10];
        const expectedHeldPercentages: number[] = [75, 50, 25, 0];

        for (let i = 0; i < testDifferencesInMilliseconds.length; i++) {
            const date = new Date(nowTime - testDifferencesInMilliseconds[i]);
            const heldPercentage = getHeldPercentage(date);

            expect(heldPercentage).toBe(expectedHeldPercentages[i]);
        }
    });
});
