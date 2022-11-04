// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue } from '@vue/test-utils';

import store, { payoutsService } from '../mock/store';

import { RootState } from '@/app/store';
import { Expectation, HeldAmountSummary, PayoutsSummary, Paystub } from '@/payouts';

const state = store.state as RootState;

const summary = new PayoutsSummary(5000000, 6000000, 9000000);
const expectations = new Expectation(4000000, 3000000);
const expectationsByNode = new Expectation(1000000, 2000000);
const period = '2021-04';
const totalPaystub = new Paystub(2000);

totalPaystub.paid = 40000;

const paystub = new Paystub(3000);
const heldHistory = [
    new HeldAmountSummary('satelliteName', 1000, 2000, 3000, 10),
    new HeldAmountSummary('satelliteName', 2000, 3000, 4000, 20),
];

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
    });

    it('sets selected node expectations', () => {
        store.commit('payouts/setCurrentNodeExpectations', expectationsByNode);

        expect(state.payouts.selectedNodePayouts.expectations.currentMonthEstimation).toBe(expectationsByNode.currentMonthEstimation);
    });
});

describe('actions', () => {
    beforeEach(() => {
        jest.resetAllMocks();
        store.commit('payouts/setSummary', new PayoutsSummary());
        store.commit('payouts/setCurrentNodeExpectations', new Expectation());
        store.commit('payouts/setPayoutPeriod', null);
        store.commit('payouts/setTotalExpectation', new Expectation());
        store.commit('payouts/setNodeTotals', new Paystub());
        store.commit('payouts/setNodePaystub', new Paystub());
        store.commit('payouts/setNodeHeldHistory', []);
    });

    it('throws error on failed summary fetch', async() => {
        jest.spyOn(payoutsService, 'summary').mockImplementation(() => { throw new Error(); });

        try {
            await store.dispatch('payouts/summary');
            expect(true).toBe(false);
        } catch (error) {
            expect(state.payouts.summary.totalPaid).toBe(0);
        }
    });

    it('success fetches payouts summary', async() => {
        jest.spyOn(payoutsService, 'summary').mockReturnValue(
            Promise.resolve(summary),
        );

        await store.dispatch('payouts/summary');

        expect(state.payouts.summary.totalPaid).toBe(summary.totalPaid);
    });

    it('throws error on failed expectations fetch', async() => {
        jest.spyOn(payoutsService, 'expectations').mockImplementation(() => { throw new Error(); });

        try {
            await store.dispatch('payouts/expectations');
            expect(true).toBe(false);
        } catch (error) {
            expect(state.payouts.totalExpectations.undistributed).toBe(0);
        }
    });

    it('success fetches total expectations', async() => {
        jest.spyOn(payoutsService, 'expectations').mockReturnValue(
            Promise.resolve(expectations),
        );

        await store.dispatch('payouts/expectations');

        expect(state.payouts.totalExpectations.undistributed).toBe(expectations.undistributed);
    });

    it('success fetches by node expectations', async() => {
        jest.spyOn(payoutsService, 'expectations').mockReturnValue(
            Promise.resolve(expectationsByNode),
        );

        await store.dispatch('payouts/expectations', 'nodeId');

        expect(state.payouts.totalExpectations.undistributed).toBe(0);
        expect(state.payouts.selectedNodePayouts.expectations.undistributed).toBe(expectationsByNode.undistributed);
    });

    it('success fetches paystubs for period', async() => {
        jest.spyOn(payoutsService, 'paystub').mockReturnValue(
            Promise.resolve(paystub),
        );

        store.commit('payouts/setPayoutPeriod', period);
        await store.dispatch('payouts/paystub', 'nodeId');

        expect(state.payouts.selectedNodePayouts.paystubForPeriod.usageAtRest).toBe(paystub.usageAtRest);
    });

    it('success fetches total paystub', async() => {
        jest.spyOn(payoutsService, 'paystub').mockReturnValue(
            Promise.resolve(totalPaystub),
        );

        await store.dispatch('payouts/nodeTotals', 'nodeId');

        expect(state.payouts.selectedNodePayouts.totalEarned).toBe(totalPaystub.paid);
    });

    it('success fetches total paystub', async() => {
        jest.spyOn(payoutsService, 'heldHistory').mockReturnValue(
            Promise.resolve(heldHistory),
        );

        await store.dispatch('payouts/heldHistory', 'nodeId');

        expect(state.payouts.selectedNodePayouts.heldHistory.length).toBe(heldHistory.length);
        expect(state.payouts.selectedNodePayouts.heldHistory[0].periodCount).toBe(heldHistory[0].periodCount);
    });
});

describe('getters', () => {
    it('getter monthsOnNetwork returns correct value', () => {
        store.commit('payouts/setPayoutPeriod', period);

        expect(store.getters['payouts/periodString']).toBe('April, 2021');
    });
});
