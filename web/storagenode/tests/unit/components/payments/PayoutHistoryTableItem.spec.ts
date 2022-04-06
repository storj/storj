// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import PayoutHistoryTableItem from '@/app/components/payments/PayoutHistoryTableItem.vue';

import { newPayoutModule } from '@/app/store/modules/payout';
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

describe('PayoutHistoryTableItem', (): void => {
    it('renders correctly with actual values (eth)', async (): Promise<void> => {
        const wrapper = shallowMount(PayoutHistoryTableItem, {
            store,
            localVue,
            propsData: {
                historyItem: new SatellitePayoutForPeriod('1', 'name1', 2, 100000, 1200000, 140,
                    500000, 600000, 200000, 800000, 'eth:receipt1', false, 75, 300000,
                ),
            },
        });

        expect(wrapper).toMatchSnapshot();

        await wrapper.find('.payout-history-item__header').trigger('click');

        expect(wrapper.vm.$data.isExpanded).toBe(true);
        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with actual values (zksync)', async (): Promise<void> => {
        const wrapper = shallowMount(PayoutHistoryTableItem, {
            store,
            localVue,
            propsData: {
                historyItem: new SatellitePayoutForPeriod('1', 'name1', 1, 100000, 1200000, 140,
                    500000, 600000, 200000, 800000, 'zksync:receipt1', false, 75, 300000,
                ),
            },
        });

        await wrapper.find('.payout-history-item__header').trigger('click');
        expect(wrapper).toMatchSnapshot();
    });
});
