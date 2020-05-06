// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import AddStorjState from '@/components/onboardingTour/steps/paymentStates/AddStorjState.vue';

import { makePaymentsModule, PAYMENTS_MUTATIONS } from '@/store/modules/payments';
import { BillingHistoryItem, BillingHistoryItemStatus, BillingHistoryItemType } from '@/types/payments';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { PaymentsMock } from '../../../mock/api/payments';

const localVue = createLocalVue();
localVue.use(Vuex);

const paymentsApi = new PaymentsMock();
const paymentsModule = makePaymentsModule(paymentsApi);
const store = new Vuex.Store({ modules: { paymentsModule }});

describe('AddStorjState.vue', () => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(AddStorjState, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with pending transaction', (): void => {
        const billingTransactionItem = new BillingHistoryItem('itemId', 'test', 50, 50,
            BillingHistoryItemStatus.Pending, 'test', new Date(), new Date(), BillingHistoryItemType.Transaction);
        store.commit(PAYMENTS_MUTATIONS.SET_BILLING_HISTORY, [billingTransactionItem]);
        const wrapper = shallowMount(AddStorjState, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with completed transaction', (): void => {
        const billingTransactionItem = new BillingHistoryItem('itemId', 'test', 50, 50,
            BillingHistoryItemStatus.Completed, 'test', new Date(), new Date(), BillingHistoryItemType.Transaction);
        store.commit(PAYMENTS_MUTATIONS.SET_BILLING_HISTORY, [billingTransactionItem]);
        store.commit(PAYMENTS_MUTATIONS.SET_BALANCE, 5000);
        const wrapper = shallowMount(AddStorjState, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
