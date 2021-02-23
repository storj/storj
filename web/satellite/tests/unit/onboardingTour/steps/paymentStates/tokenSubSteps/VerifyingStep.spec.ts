// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import VerifyingStep from '@/components/onboardingTour/steps/paymentStates/tokenSubSteps/VerifyingStep.vue';

import { makePaymentsModule, PAYMENTS_MUTATIONS } from '@/store/modules/payments';
import { PaymentsHistoryItem, PaymentsHistoryItemStatus, PaymentsHistoryItemType } from '@/types/payments';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { PaymentsMock } from '../../../../mock/api/payments';

const localVue = createLocalVue();
localVue.use(Vuex);

const paymentsApi = new PaymentsMock();
const paymentsModule = makePaymentsModule(paymentsApi);
const store = new Vuex.Store({ modules: { paymentsModule }});
const paymentsTransactionItem = new PaymentsHistoryItem('itemId', 'test', 50, 50,
    PaymentsHistoryItemStatus.Pending, 'test', new Date(), new Date(), PaymentsHistoryItemType.Transaction);
store.commit(PAYMENTS_MUTATIONS.SET_PAYMENTS_HISTORY, [paymentsTransactionItem]);

describe('VerifyingStep.vue', () => {
    it('renders correctly', async (): Promise<void> => {
        const wrapper = shallowMount(VerifyingStep, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();

        await wrapper.find('.verifying-step__back-button').trigger('click');

        expect(wrapper.emitted()).toEqual({ 'setDefaultState': [[]] });
    });
});
