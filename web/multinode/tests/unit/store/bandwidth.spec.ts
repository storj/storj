// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue } from '@vue/test-utils';

import store, { bandwidthService } from '../mock/store';

import { RootState } from '@/app/store';
import { BandwidthRollup, BandwidthTraffic, Egress, Ingress } from '@/bandwidth';

const state = store.state as RootState;

const mockedDate1 = new Date(Date.UTC(2020, 1, 30));
const mockedDate2 = new Date(1573562290000); // Tue Nov 12 2019
const traffic = new BandwidthTraffic(
    [
        new BandwidthRollup(
            new Egress(1220000, 600000, 1245500000),
            new Ingress(4000000, 1111),
            80080080,
            mockedDate1,
        ),
        new BandwidthRollup(
            new Egress(122000000, 23500000, 15500000),
            new Ingress(40000, 11110000),
            80080080,
            mockedDate2,
        ),
    ],
    700000000,
    577700000000,
    5000000,
);

describe('mutations', () => {
    beforeEach(() => {
        createLocalVue().use(Vuex);
    });

    it('populates traffic', () => {
        store.commit('bandwidth/populate', traffic);

        expect(state.bandwidth.traffic.bandwidthSummary).toBe(traffic.bandwidthSummary);
    });
});

describe('actions', () => {
    beforeEach(() => {
        jest.resetAllMocks();
        store.commit('bandwidth/populate', new BandwidthTraffic());
    });

    it('throws error on failed bandwidth fetch', async() => {
        jest.spyOn(bandwidthService, 'fetch').mockImplementation(() => { throw new Error(); });

        try {
            await store.dispatch('bandwidth/fetch');
            expect(true).toBe(false);
        } catch (error) {
            expect(state.bandwidth.traffic.bandwidthSummary).toBe(0);
        }
    });

    it('success fetches payouts summary', async() => {
        jest.spyOn(bandwidthService, 'fetch').mockReturnValue(
            Promise.resolve(traffic),
        );

        await store.dispatch('bandwidth/fetch');

        expect(state.bandwidth.traffic.bandwidthDaily.length).toBe(traffic.bandwidthDaily.length);
    });
});
