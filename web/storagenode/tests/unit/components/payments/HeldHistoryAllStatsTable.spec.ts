// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import HeldHistoryAllStatsTable from '@/app/components/payments/HeldHistoryAllStatsTable.vue';

import { makePayoutModule, PAYOUT_MUTATIONS } from '@/app/store/modules/payout';
import { HeldHistory, HeldHistoryAllStatItem } from '@/app/types/payout';
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

describe('HeldHistoryAllStatsTable', (): void => {
    it('renders correctly with actual values', async (): Promise<void> => {
        const wrapper = shallowMount(HeldHistoryAllStatsTable, {
            store,
            localVue,
        });

        const testJoinAt = new Date(Date.UTC(2020, 0, 30));

        await store.commit(PAYOUT_MUTATIONS.SET_HELD_HISTORY, new HeldHistory(
            [],
            [
                new HeldHistoryAllStatItem('1', 'name1', 1, 50000, 20000, testJoinAt),
                new HeldHistoryAllStatItem('2', 'name2', 5, 40000, 30000, testJoinAt),
            ],
        ));

        expect(wrapper).toMatchSnapshot();
    });
});
