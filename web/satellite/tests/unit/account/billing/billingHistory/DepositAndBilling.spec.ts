// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import sinon from 'sinon';
import Vuex from 'vuex';

import DepositAndBilling from '@/components/account/billing/billingHistory/DepositAndBilling.vue';

import { PaymentsHttpApi } from '@/api/payments';
import { makePaymentsModule, PAYMENTS_MUTATIONS } from '@/store/modules/payments';
import { BillingHistoryItem, BillingHistoryItemType } from '@/types/payments';
import { createLocalVue, shallowMount } from '@vue/test-utils';

const localVue = createLocalVue();
localVue.use(Vuex);

const paymentsApi = new PaymentsHttpApi();
const paymentsModule = makePaymentsModule(paymentsApi);
const itemInvoice = new BillingHistoryItem('testId', 'Invoice', 500, 500, 'test', 'test', new Date(1), new Date(1), BillingHistoryItemType.Invoice);
const itemCharge = new BillingHistoryItem('testId1', 'Charge', 500, 500, 'test', 'test', new Date(1), new Date(1), BillingHistoryItemType.Charge);
const itemTransaction = new BillingHistoryItem('testId2', 'Transaction', 500, 500, 'test', 'test', new Date(1), new Date(1), BillingHistoryItemType.Transaction);
const itemTransaction1 = new BillingHistoryItem('testId3', 'Transaction', 500, 500, 'test', 'test', new Date(1), new Date(1), BillingHistoryItemType.Transaction);
const clickSpy = sinon.spy();

const store = new Vuex.Store({ modules: { paymentsModule }});

describe('DepositAndBilling', (): void => {
    it('renders correctly without items', (): void => {
        const wrapper = shallowMount(DepositAndBilling, {
            localVue,
            store,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with items', (): void => {
        store.commit(PAYMENTS_MUTATIONS.SET_BILLING_HISTORY, [itemCharge, itemInvoice, itemTransaction, itemTransaction1]);

        const wrapper = shallowMount(DepositAndBilling, {
            localVue,
            store,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('click on view all works correctly', async (): Promise<void> => {
        const wrapper = shallowMount(DepositAndBilling, {
            localVue,
            store,
            methods: {
                onViewAllClick: clickSpy,
            },
        });

        await wrapper.find('.button').trigger('click');

        expect(clickSpy.callCount).toBe(1);
    });
});
