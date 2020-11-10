// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import PayoutHistoryTable from '@/app/components/payments/PayoutHistoryTable.vue';

import { newPayoutModule, PAYOUT_MUTATIONS } from '@/app/store/modules/payout';
import { PayoutHttpApi } from '@/storagenode/api/payout';
import { SatellitePayoutForPeriod } from '@/storagenode/payouts/payouts';
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

describe('PayoutHistoryTable', (): void => {
    it('renders correctly with actual values', async (): Promise<void> => {
        await store.commit(PAYOUT_MUTATIONS.SET_PAYOUT_HISTORY, [
            new SatellitePayoutForPeriod('1', 'name1', 1, 100000, 1200000, 140,
                500000, 600000, 200000, 800000, 'receipt1', false,
            ),
            new SatellitePayoutForPeriod('2', 'name2', 16, 100000, 1200000, 140,
                500000, 600000, 200000, 800000, 'receipt2', true,
            ),
        ]);

        const wrapper = shallowMount(PayoutHistoryTable, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
