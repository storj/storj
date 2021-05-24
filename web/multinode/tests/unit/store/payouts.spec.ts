// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import { PayoutsSummary } from '@/payouts';
import { createLocalVue } from '@vue/test-utils';

import store, { payoutsService } from '../mock/store';

const state = store.state as any;

const summary = new PayoutsSummary(5000000, 6000000, 9000000);
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
});

describe('actions', () => {
    beforeEach(() => {
        jest.resetAllMocks();
        store.commit('payouts/setSummary', new PayoutsSummary());
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
});

describe('getters', () => {
    it('getter monthsOnNetwork returns correct value',  () => {
        store.commit('payouts/setPayoutPeriod', period);

        expect(store.getters['payouts/periodString']).toBe('April, 2021');
    });
});
