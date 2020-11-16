// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import HeldHistoryAllStatsTable from '@/app/components/payments/HeldHistoryAllStatsTable.vue';

import { newPayoutModule, PAYOUT_MUTATIONS } from '@/app/store/modules/payout';
import { PayoutHttpApi } from '@/storagenode/api/payout';
import { SatelliteHeldHistory } from '@/storagenode/payouts/payouts';
import { PayoutService } from '@/storagenode/payouts/service';
import { createLocalVue, shallowMount } from '@vue/test-utils';

const localVue = createLocalVue();
localVue.use(Vuex);

localVue.filter('centsToDollars', (cents: number): string => {
    return `$${(cents / 100).toFixed(2)}`;
});

const payoutApi = new PayoutHttpApi();
const payoutService = new PayoutService(payoutApi);
const payoutModule = newPayoutModule(payoutService);

const store = new Vuex.Store({ modules: { payoutModule }});

describe('HeldHistoryAllStatsTable', (): void => {
    it('renders correctly with actual values', async (): Promise<void> => {
        const _Date = Date;
        const mockedDate = new Date(1580522290000);
        const testJoinAt = new Date(Date.UTC(2019, 6, 30));
        global.Date = jest.fn(() => mockedDate);

        const wrapper = shallowMount(HeldHistoryAllStatsTable, {
            store,
            localVue,
        });

        const testHeldHistory = [
            new SatelliteHeldHistory('1', 'name1', 1, 50000, 0, 1, 1, testJoinAt),
            new SatelliteHeldHistory('2', 'name2', 5, 50000, 422280, 0, 0, testJoinAt),
            new SatelliteHeldHistory('3', 'name3', 6, 50000, 7333880, 7852235, 0, testJoinAt),
        ];

        await store.commit(PAYOUT_MUTATIONS.SET_HELD_HISTORY, testHeldHistory);

        expect(wrapper).toMatchSnapshot();

        global.Date = _Date;
    });
});
