// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import BillingItem from '@/components/account/billing/billingHistory/BillingItem.vue';

import { BillingHistoryItem, BillingHistoryItemType } from '@/types/payments';
import { createLocalVue, mount } from '@vue/test-utils';

const localVue = createLocalVue();
const itemInvoice = new BillingHistoryItem('testId', 'Invoice', 500, 500, 'test', 'test', new Date(1), new Date(1), BillingHistoryItemType.Invoice);
const itemCharge = new BillingHistoryItem('testId', 'Charge', 500, 500, 'test', 'test', new Date(1), new Date(1), BillingHistoryItemType.Charge);
const itemTransaction = new BillingHistoryItem('testId', 'Transaction', 500, 500, 'test', 'test', new Date(1), new Date(1), BillingHistoryItemType.Transaction);

describe('BillingItem', (): void => {
    it('renders correctly if invoice', (): void => {
        const wrapper = mount(BillingItem, {
            localVue,
            propsData: {
                billingItem: itemInvoice,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly if charge', (): void => {
        const wrapper = mount(BillingItem, {
            localVue,
            propsData: {
                billingItem: itemCharge,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly if transaction', (): void => {
        const wrapper = mount(BillingItem, {
            localVue,
            propsData: {
                billingItem: itemTransaction,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });
});
