// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import { makeNodeModule } from '@/app/store/modules/node';
import { makePayoutModule, PAYOUT_ACTIONS, PAYOUT_MUTATIONS } from '@/app/store/modules/payout';
import {
    HeldHistory,
    HeldHistoryMonthlyBreakdownItem,
    HeldInfo,
    PayoutInfoRange,
    PayoutPeriod,
    TotalPayoutInfo,
} from '@/app/types/payout';
import { getHeldPercentage, getMonthsBeforeNow } from '@/app/utils/payout';
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

describe('mutations', (): void => {
    beforeEach(() => {
        createLocalVue().use(Vuex);
    });

    it('sets held information', (): void => {
        const heldInfo = new HeldInfo(13, 12, 11);

        store.commit(PAYOUT_MUTATIONS.SET_HELD_INFO, heldInfo);

        expect(state.payoutModule.heldInfo.usageAtRest).toBe(13);
        expect(state.payoutModule.heldInfo.usageGet).toBe(12);
        expect(state.payoutModule.heldInfo.usagePut).toBe(11);
    });

    it('sets total payout information', (): void => {
        const totalInfo = new TotalPayoutInfo(50, 100, 22);

        store.commit(PAYOUT_MUTATIONS.SET_TOTAL, totalInfo);

        expect(state.payoutModule.totalHeldAmount).toBe(50);
        expect(state.payoutModule.totalEarnings).toBe(100);
        expect(state.payoutModule.currentMonthEarnings).toBe(22);
    });

    it('sets period range', (): void => {
        const range = new PayoutInfoRange(new PayoutPeriod(2019, 2), new PayoutPeriod(2020, 3));

        store.commit(PAYOUT_MUTATIONS.SET_RANGE, range);

        if (!state.payoutModule.periodRange.start) {
            fail('periodRange.start is null');
        }

        expect(state.payoutModule.periodRange.start.period).toBe('2019-03');
        expect(state.payoutModule.periodRange.end.period).toBe('2020-04');
    });

    it('sets held percentage', (): void => {
        const expectedHeldPercentage = 75;

        store.commit(PAYOUT_MUTATIONS.SET_HELD_PERCENT, expectedHeldPercentage);

        expect(state.payoutModule.heldPercentage).toBe(expectedHeldPercentage);
    });

    it('sets held history', (): void => {
        const testHeldHistory = new HeldHistory([
            new HeldHistoryMonthlyBreakdownItem('1', 'name1', 1, 50000, 0, 0, 0),
            new HeldHistoryMonthlyBreakdownItem('2', 'name2', 5, 50000, 422280, 0, 0),
            new HeldHistoryMonthlyBreakdownItem('3', 'name3', 6, 50000, 7333880, 7852235, 0),
        ]);

        store.commit(PAYOUT_MUTATIONS.SET_HELD_HISTORY, testHeldHistory);

        expect(state.payoutModule.heldHistory.monthlyBreakdown.length).toBe(testHeldHistory.monthlyBreakdown.length);
        expect(state.payoutModule.heldHistory.monthlyBreakdown[1].satelliteName).toBe(testHeldHistory.monthlyBreakdown[1].satelliteName);
    });
});

describe('actions', () => {
    beforeEach(() => {
        jest.resetAllMocks();
    });

    it('success get held info by month', async (): Promise<void> => {
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

    it('get held info by month throws an error when api call fails', async (): Promise<void> => {
        jest.spyOn(payoutApi, 'getHeldInfoByMonth').mockImplementation(() => { throw new Error(); });

        try {
            await store.dispatch(PAYOUT_ACTIONS.GET_HELD_INFO);
            expect(true).toBe(false);
        } catch (error) {
            expect(state.payoutModule.heldInfo.usagePut).toBe(3);
            expect(state.payoutModule.heldInfo.held).toBe(0);
        }
    });

    it('success get held info by period', async (): Promise<void> => {
        jest.spyOn(payoutApi, 'getHeldInfoByPeriod').mockReturnValue(
            Promise.resolve(new HeldInfo(1, 2 , 3, 4, 5)),
        );

        const range = new PayoutInfoRange(new PayoutPeriod(2019, 2), new PayoutPeriod(2020, 3));

        store.commit(PAYOUT_MUTATIONS.SET_RANGE, range);

        await store.dispatch(PAYOUT_ACTIONS.GET_HELD_INFO);

        expect(state.payoutModule.heldInfo.usagePut).toBe(3);
        expect(state.payoutModule.heldInfo.held).toBe(0);
    });

    it('get held info by period throws an error when api call fails', async (): Promise<void> => {
        jest.spyOn(payoutApi, 'getHeldInfoByPeriod').mockImplementation(() => { throw new Error(); });

        try {
            await store.dispatch(PAYOUT_ACTIONS.GET_HELD_INFO);
            expect(true).toBe(false);
        } catch (error) {
            expect(state.payoutModule.heldInfo.usagePut).toBe(3);
            expect(state.payoutModule.heldInfo.held).toBe(0);
        }
    });

    it('success get total', async (): Promise<void> => {
        jest.spyOn(payoutApi, 'getTotal').mockReturnValue(
            Promise.resolve(new TotalPayoutInfo(10, 20, 5)),
        );

        await store.dispatch(PAYOUT_ACTIONS.GET_TOTAL);

        expect(state.payoutModule.totalHeldAmount).toBe(10);
        expect(state.payoutModule.totalEarnings).toBe(20);
        expect(state.payoutModule.currentMonthEarnings).toBe(0);
    });

    it('get total throws an error when api call fails', async (): Promise<void> => {
        jest.spyOn(payoutApi, 'getTotal').mockImplementation(() => { throw new Error(); });

        try {
            await store.dispatch(PAYOUT_ACTIONS.GET_TOTAL);
            expect(true).toBe(false);
        } catch (error) {
            expect(state.payoutModule.totalHeldAmount).toBe(10);
            expect(state.payoutModule.totalEarnings).toBe(20);
            expect(state.payoutModule.currentMonthEarnings).toBe(0);
        }
    });

    it('success sets period range', async (): Promise<void> => {
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

    it('success get held history', async (): Promise<void> => {
        jest.spyOn(payoutApi, 'getHeldHistory').mockReturnValue(
            Promise.resolve(new HeldHistory([
                new HeldHistoryMonthlyBreakdownItem('1', 'name1', 1, 50000, 0, 0, 0),
                new HeldHistoryMonthlyBreakdownItem('2', 'name2', 5, 50000, 422280, 0, 0),
                new HeldHistoryMonthlyBreakdownItem('3', 'name3', 6, 50000, 7333880, 7852235, 0),
            ])),
        );

        await store.dispatch(PAYOUT_ACTIONS.GET_HELD_HISTORY);

        expect(state.payoutModule.heldHistory.monthlyBreakdown.length).toBe(3);
        expect(state.payoutModule.heldHistory.monthlyBreakdown[1].satelliteName).toBe('name2');
    });

    it('get total throws an error when api call fails', async (): Promise<void> => {
        jest.spyOn(payoutApi, 'getHeldHistory').mockImplementation(() => { throw new Error(); });

        try {
            await store.dispatch(PAYOUT_ACTIONS.GET_HELD_HISTORY);
            expect(true).toBe(false);
        } catch (error) {
            expect(state.payoutModule.heldHistory.monthlyBreakdown.length).toBe(3);
            expect(state.payoutModule.heldHistory.monthlyBreakdown[1].satelliteName).toBe('name2');
        }
    });
});

describe('utils functions', (): void => {
    const _Date = Date;

    // TODO: investigate reset mocks in config
    beforeEach(() => {
        jest.resetAllMocks();
    });

    afterEach(() => {
        global.Date = _Date;
    });

    it('get correct held percentage', (): void => {
        const testDates: Date[] = [
            new Date(Date.UTC(2020, 0, 30)),
            new Date(Date.UTC(2019, 10, 29)),
            new Date(Date.UTC(2019, 7, 24)),
            new Date(Date.UTC(2018, 1, 24)),
        ];
        const expectedHeldPercentages: number[] = [75, 50, 25, 0];

        const mockedDate = new Date(1580522290000); // Sat Feb 01 2020
        global.Date = jest.fn(() => mockedDate);

        for (let i = 0; i < testDates.length; i++) {
            const heldPercentage = getHeldPercentage(testDates[i]);

            expect(heldPercentage).toBe(expectedHeldPercentages[i]);
        }
    });

    it('get correct months difference', (): void => {
        const testDates: Date[] = [
            new Date(Date.UTC(2020, 0, 30)),
            new Date(Date.UTC(2019, 10, 29)),
            new Date(Date.UTC(2019, 7, 24)),
            new Date(Date.UTC(2018, 1, 24)),
        ];
        const expectedMonthsCount: number[] = [2, 4, 7, 25];

        const mockedDate = new Date(1580522290000); // Sat Feb 01 2020
        global.Date = jest.fn(() => mockedDate);

        for (let i = 0; i < testDates.length; i++) {
            const heldPercentage = getMonthsBeforeNow(testDates[i]);

            expect(heldPercentage).toBe(expectedMonthsCount[i]);
        }
    });
});
