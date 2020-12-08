// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import TotalPayoutArea from '@/app/components/TotalPayoutArea.vue';

import { newPayoutModule, PAYOUT_MUTATIONS } from '@/app/store/modules/payout';
import { PayoutHttpApi } from '@/storagenode/api/payout';
import { Paystub, TotalHeldAndPaid } from '@/storagenode/payouts/payouts';
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

describe('TotalPayoutArea', (): void => {
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
        const paystub = new Paystub();
        paystub.held = 600000;
        paystub.disposed = 100000;
        paystub.paid = 1000000;

        const totalHeldAndPaid = new TotalHeldAndPaid([paystub]);
        totalHeldAndPaid.setCurrentMonthEarnings(40000);

        await store.commit(PAYOUT_MUTATIONS.SET_TOTAL, totalHeldAndPaid);

        expect(wrapper).toMatchSnapshot();

        totalHeldAndPaid.held = 400000;

        await store.commit(PAYOUT_MUTATIONS.SET_TOTAL, totalHeldAndPaid);

        expect(wrapper).toMatchSnapshot();
    });
});
