// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import HeldHistoryMonthlyBreakdownTable from '@/app/components/payments/HeldHistoryMonthlyBreakdownTable.vue';

import { makePayoutModule, PAYOUT_MUTATIONS } from '@/app/store/modules/payout';
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
const payoutModule = makePayoutModule(payoutApi, payoutService);

const store = new Vuex.Store({ modules: { payoutModule }});

describe('HeldHistoryMonthlyBreakdownTable', (): void => {
    it('renders correctly with actual values', async (): Promise<void> => {
        const wrapper = shallowMount(HeldHistoryMonthlyBreakdownTable, {
            store,
            localVue,
        });

        const testJoinedAt = new Date(2020, 1, 20);

        await store.commit(PAYOUT_MUTATIONS.SET_HELD_HISTORY, [
            new SatelliteHeldHistory('1', 'name1', 1, 50000, 0, 0, 0, testJoinedAt),
            new SatelliteHeldHistory('2', 'name2', 5, 50000, 422280, 0, 0 , testJoinedAt),
            new SatelliteHeldHistory('3', 'name3', 6, 50000, 7333880, 7852235, 0, testJoinedAt),
        ]);

        expect(wrapper).toMatchSnapshot();
    });
});
