// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import sinon from 'sinon';
import Vuex from 'vuex';

import SmallDepositHistory from '@/components/account/billing/depositAndBillingHistory/SmallDepositHistory.vue';

import { PaymentsHttpApi } from '@/api/payments';
import { makePaymentsModule, PAYMENTS_MUTATIONS } from '@/store/modules/payments';
import { PaymentsHistoryItem, PaymentsHistoryItemType } from '@/types/payments';
import { createLocalVue, shallowMount } from '@vue/test-utils';

const localVue = createLocalVue();
localVue.use(Vuex);

const paymentsApi = new PaymentsHttpApi();
const paymentsModule = makePaymentsModule(paymentsApi);
const itemInvoice = new PaymentsHistoryItem('testId', 'Invoice', 500, 500, 'test', 'test', new Date(1), new Date(1), PaymentsHistoryItemType.Invoice);
const itemCharge = new PaymentsHistoryItem('testId1', 'Charge', 500, 500, 'test', 'test', new Date(1), new Date(1), PaymentsHistoryItemType.Charge);
const itemTransaction = new PaymentsHistoryItem('testId2', 'Transaction', 500, 500, 'test', 'test', new Date(1), new Date(1), PaymentsHistoryItemType.Transaction);
const itemTransaction1 = new PaymentsHistoryItem('testId3', 'Transaction', 500, 500, 'test', 'test', new Date(1), new Date(1), PaymentsHistoryItemType.Transaction);
const itemTransaction2 = new PaymentsHistoryItem('testId4', 'Transaction', 500, 500, 'test', 'test', new Date(1), new Date(1), PaymentsHistoryItemType.Transaction);
const itemTransaction3 = new PaymentsHistoryItem('testId5', 'Transaction', 500, 500, 'test', 'test', new Date(1), new Date(1), PaymentsHistoryItemType.Transaction);
const clickSpy = sinon.spy();

const store = new Vuex.Store({ modules: { paymentsModule }});

describe('SmallDepositHistory', (): void => {
    it('renders correctly without items', (): void => {
        const wrapper = shallowMount(SmallDepositHistory, {
            localVue,
            store,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly with items', (): void => {
        store.commit(PAYMENTS_MUTATIONS.SET_PAYMENTS_HISTORY, [itemCharge, itemInvoice, itemTransaction, itemTransaction1, itemTransaction2, itemTransaction3]);

        const wrapper = shallowMount(SmallDepositHistory, {
            localVue,
            store,
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('click on view all works correctly', async (): Promise<void> => {
        const wrapper = shallowMount(SmallDepositHistory, {
            localVue,
            store,
        });

        wrapper.vm.onViewAllClick = clickSpy;

        await wrapper.find('.deposit-area__header__button').trigger('click');

        expect(clickSpy.callCount).toBe(1);
    });
});
