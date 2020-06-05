// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import HeldHistoryMonthlyBreakdownTable from '@/app/components/payments/HeldHistoryMonthlyBreakdownTable.vue';

import { makePayoutModule, PAYOUT_MUTATIONS } from '@/app/store/modules/payout';
import { HeldHistory, HeldHistoryMonthlyBreakdownItem } from '@/app/types/payout';
import { PayoutHttpApi } from '@/storagenode/api/payout';
import { createLocalVue, shallowMount } from '@vue/test-utils';

const localVue = createLocalVue();
localVue.use(Vuex);

localVue.filter('centsToDollars', (cents: number): string => {
    return `$${(cents / 100).toFixed(2)}`;
});

const payoutApi = new PayoutHttpApi();
const payoutModule = makePayoutModule(payoutApi);

const store = new Vuex.Store({ modules: { payoutModule }});

describe('HeldHistoryMonthlyBreakdownTable', (): void => {
    it('renders correctly with actual values', async (): Promise<void> => {
        const wrapper = shallowMount(HeldHistoryMonthlyBreakdownTable, {
            store,
            localVue,
        });

        await store.commit(PAYOUT_MUTATIONS.SET_HELD_HISTORY, new HeldHistory([
            new HeldHistoryMonthlyBreakdownItem('1', 'name1', 1, 50000, 0, 0, 0),
            new HeldHistoryMonthlyBreakdownItem('2', 'name2', 5, 50000, 422280, 0, 0),
            new HeldHistoryMonthlyBreakdownItem('3', 'name3', 6, 50000, 7333880, 7852235, 0),
        ]));

        expect(wrapper).toMatchSnapshot();
    });
});
