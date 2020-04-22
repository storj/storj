// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import AddPaymentStep from '@/components/onboardingTour/steps/AddPaymentStep.vue';

import { PaymentsHttpApi } from '@/api/payments';
import { makePaymentsModule } from '@/store/modules/payments';
import { createLocalVue, shallowMount } from '@vue/test-utils';

const localVue = createLocalVue();
localVue.use(Vuex);
const paymentsApi = new PaymentsHttpApi();
const paymentsModule = makePaymentsModule(paymentsApi);

const store = new Vuex.Store({ modules: { paymentsModule }});

describe('AddPaymentStep.vue', () => {
    it('renders correctly', async (): Promise<void> => {
        const wrapper = shallowMount(AddPaymentStep, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();

        await wrapper.find('.payment-step__methods-container__title-area__options-area__token').trigger('click');

        expect(wrapper).toMatchSnapshot();
    });
});
