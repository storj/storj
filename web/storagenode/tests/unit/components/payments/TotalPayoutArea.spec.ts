// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import TotalPayoutArea from '@/app/components/TotalPayoutArea.vue';

import { makePayoutModule, PAYOUT_MUTATIONS } from '@/app/store/modules/payout';
import { TotalPayoutInfo } from '@/app/types/payout';
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

describe('TotalPayoutInfo', (): void => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(TotalPayoutArea, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with actual values', async (): Promise<void> => {
        const wrapper = shallowMount(TotalPayoutArea, {
            store,
            localVue,
        });

        await store.commit(PAYOUT_MUTATIONS.SET_TOTAL, new TotalPayoutInfo(2100, 5000, 8000));

        expect(wrapper).toMatchSnapshot();
    });
});
