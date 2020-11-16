// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import HeldHistoryArea from '@/app/components/payments/HeldHistoryArea.vue';

import { newPayoutModule } from '@/app/store/modules/payout';
import { PayoutHttpApi } from '@/storagenode/api/payout';
import { PayoutService } from '@/storagenode/payouts/service';
import { createLocalVue, shallowMount } from '@vue/test-utils';

const localVue = createLocalVue();
localVue.use(Vuex);

const payoutApi = new PayoutHttpApi();
const payoutService = new PayoutService(payoutApi);
const payoutModule = newPayoutModule(payoutService);

const store = new Vuex.Store({ modules: { payoutModule }});

describe('HeldHistoryArea', (): void => {
    it('renders correctly',  async (): Promise<void> => {
        const wrapper = shallowMount(HeldHistoryArea, {
            store,
            localVue,
        });

        await localVue.nextTick();

        expect(wrapper).toMatchSnapshot();
    });

    it('changes state correctly',  async (): Promise<void> => {
        const wrapper = shallowMount(HeldHistoryArea, {
            store,
            localVue,
        });

        wrapper.findAll('.held-history-container__header__selection-area__item').at(1).trigger('click');

        await localVue.nextTick();

        expect(wrapper).toMatchSnapshot();

        wrapper.findAll('.held-history-container__header__selection-area__item').at(0).trigger('click');

        await localVue.nextTick();

        expect(wrapper).toMatchSnapshot();
    });
});
