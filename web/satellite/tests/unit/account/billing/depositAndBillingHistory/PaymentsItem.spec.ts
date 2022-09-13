// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { createLocalVue, mount } from '@vue/test-utils';

import { PaymentsHistoryItem, PaymentsHistoryItemType } from '@/types/payments';

import PaymentsItem from '@/components/account/billing/depositAndBillingHistory/PaymentsItem.vue';

const localVue = createLocalVue();
const itemInvoice = new PaymentsHistoryItem('testId', 'Invoice', 500, 500, 'test', 'test', new Date(1), new Date(1), PaymentsHistoryItemType.Invoice);
const itemCharge = new PaymentsHistoryItem('testId', 'Charge', 500, 500, 'test', 'test', new Date(1), new Date(1), PaymentsHistoryItemType.Charge);
const itemTransaction = new PaymentsHistoryItem('testId', 'Transaction', 500, 500, 'test', 'test', new Date(1), new Date(1), PaymentsHistoryItemType.Transaction);

describe('PaymentsItem', (): void => {
    it('renders correctly if invoice', (): void => {
        const wrapper = mount(PaymentsItem, {
            localVue,
            propsData: {
                billingItem: itemInvoice,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly if charge', (): void => {
        const wrapper = mount(PaymentsItem, {
            localVue,
            propsData: {
                billingItem: itemCharge,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly if transaction', (): void => {
        const wrapper = mount(PaymentsItem, {
            localVue,
            propsData: {
                billingItem: itemTransaction,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });
});
