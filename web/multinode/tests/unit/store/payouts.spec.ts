// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import { Expectations, PayoutsSummary } from '@/payouts';
import { createLocalVue } from '@vue/test-utils';

import store, { payoutsService } from '../mock/store';

const state = store.state as any;

const summary = new PayoutsSummary(5000000, 6000000, 9000000);
const expectations = new Expectations(4000000, 3000000);
const expectationsByNode = new Expectations(1000000, 2000000);
const period = '2021-04';

describe('mutations', () => {
    beforeEach(() => {
        createLocalVue().use(Vuex);
    });

    it('sets summary', () => {
        store.commit('payouts/setSummary', summary);

        expect(state.payouts.summary.totalPaid).toBe(summary.totalPaid);
    });

    it('sets payout period', () => {
        store.commit('payouts/setPayoutPeriod', period);

        expect(state.payouts.selectedPayoutPeriod).toBe(period);
    });

    it('sets total expectations', () => {
        store.commit('payouts/setTotalExpectation', expectations);

        expect(state.payouts.totalExpectations.currentMonthEstimation).toBe(expectations.currentMonthEstimation);
        expect(state.payouts.selectedNodeExpectations.currentMonthEstimation).toBe(0);
    });

    it('sets selected node expectations', () => {
        store.commit('payouts/setCurrentNodeExpectations', expectationsByNode);

        expect(state.payouts.selectedNodeExpectations.currentMonthEstimation).toBe(expectationsByNode.currentMonthEstimation);
    });
});

describe('actions', () => {
    beforeEach(() => {
        jest.resetAllMocks();
        store.commit('payouts/setSummary', new PayoutsSummary());
        store.commit('payouts/setCurrentNodeExpectations', new Expectations());
        store.commit('payouts/setTotalExpectation', new Expectations());
    });

    it('throws error on failed summary fetch', async () => {
        jest.spyOn(payoutsService, 'summary').mockImplementation(() => { throw new Error(); });

        try {
            await store.dispatch('payouts/summary');
            expect(true).toBe(false);
        } catch (error) {
            expect(state.payouts.summary.totalPaid).toBe(0);
        }
    });

    it('success fetches payouts summary', async () => {
        jest.spyOn(payoutsService, 'summary').mockReturnValue(
            Promise.resolve(summary),
        );

        await store.dispatch('payouts/summary');

        expect(state.payouts.summary.totalPaid).toBe(summary.totalPaid);
    });

    it('throws error on failed expectations fetch', async () => {
        jest.spyOn(payoutsService, 'expectations').mockImplementation(() => { throw new Error(); });

        try {
            await store.dispatch('payouts/expectations');
            expect(true).toBe(false);
        } catch (error) {
            expect(state.payouts.totalExpectations.undistributed).toBe(0);
        }
    });

    it('success fetches total expectations', async () => {
        jest.spyOn(payoutsService, 'expectations').mockReturnValue(
            Promise.resolve(expectations),
        );

        await store.dispatch('payouts/expectations');

        expect(state.payouts.totalExpectations.undistributed).toBe(expectations.undistributed);
    });

    it('success fetches by node expectations', async () => {
        jest.spyOn(payoutsService, 'expectations').mockReturnValue(
            Promise.resolve(expectationsByNode),
        );

        await store.dispatch('payouts/expectations', 'nodeId');

        expect(state.payouts.totalExpectations.undistributed).toBe(0);
        expect(state.payouts.selectedNodeExpectations.undistributed).toBe(expectationsByNode.undistributed);
    });
});

describe('getters', () => {
    it('getter monthsOnNetwork returns correct value',  () => {
        store.commit('payouts/setPayoutPeriod', period);

        expect(store.getters['payouts/periodString']).toBe('April, 2021');
    });
});
