// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { newPayoutModule, PAYOUT_MUTATIONS } from '@/app/store/modules/payout';
import { PayoutHttpApi } from '@/storagenode/api/payout';
import { EstimatedPayout, Paystub, PreviousMonthEstimatedPayout, TotalPayments } from '@/storagenode/payouts/payouts';
import { PayoutService } from '@/storagenode/payouts/service';

import TotalPayoutArea from '@/app/components/TotalPayoutArea.vue';

const localVue = createLocalVue();
localVue.use(Vuex);

localVue.filter('centsToDollars', (cents: number): string => {
    return `$${(cents / 100).toFixed(2)}`;
});

const payoutApi = new PayoutHttpApi();
const payoutService = new PayoutService(payoutApi);
const payoutModule = newPayoutModule(payoutService);

const store = new Vuex.Store({ modules: { payoutModule } });

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

        const totalPayments = new TotalPayments([paystub]);
        await store.commit(PAYOUT_MUTATIONS.SET_TOTAL, totalPayments);

        const currentMonthEstimation = new PreviousMonthEstimatedPayout();
        currentMonthEstimation.payout = 30000;
        currentMonthEstimation.held = 10000;
        const estimation = new EstimatedPayout();
        estimation.currentMonth = currentMonthEstimation;
        await store.commit(PAYOUT_MUTATIONS.SET_ESTIMATION, estimation);

        expect(wrapper).toMatchSnapshot();

        totalPayments.held = 400000;

        await store.commit(PAYOUT_MUTATIONS.SET_TOTAL, totalPayments);

        expect(wrapper).toMatchSnapshot();
    });
});
