// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { createLocalVue, mount } from '@vue/test-utils';

import { PaymentsHistoryItemStatus, PaymentsHistoryItemType } from '@/types/payments';

import PaymentsHistoryItemDate from '@/components/account/billing/depositAndBillingHistory/PaymentsHistoryItemDate.vue';

const localVue = createLocalVue();

describe('PaymentsHistoryItemDate', (): void => {
    it('renders correctly if invoice', (): void => {
        const startDate = new Date(2019, 1, 1, 1, 1, 1, 1);
        const expirationDate = new Date(0, 1, 1, 1, 1, 1, 1);

        jest.useFakeTimers('modern');
        jest.setSystemTime(startDate);

        const wrapper = mount(PaymentsHistoryItemDate, {
            localVue,
            propsData: {
                expiration: expirationDate,
                start: startDate,
                type: PaymentsHistoryItemType.Invoice,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly if charge', (): void => {
        const startDate = new Date(2019, 5, 5, 5, 5, 5, 5);
        const expirationDate = new Date(0, 1, 1, 1, 1, 1, 1);

        jest.useFakeTimers('modern');
        jest.setSystemTime(startDate);

        const wrapper = mount(PaymentsHistoryItemDate, {
            localVue,
            propsData: {
                expiration: expirationDate,
                start: startDate,
                type: PaymentsHistoryItemType.Charge,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly if transaction is not expired', (): void => {
        const startDate = new Date(2019, 5, 5, 5, 5, 5, 5);
        const expirationDate = new Date(2019, 5, 5, 6, 5, 5, 5);
        const testTimeNow = expirationDate.getTime();

        jest.useFakeTimers('modern');
        jest.setSystemTime(testTimeNow);

        const wrapper = mount(PaymentsHistoryItemDate, {
            localVue,
            propsData: {
                expiration: expirationDate,
                start: startDate,
                type: PaymentsHistoryItemType.Transaction,
                status: PaymentsHistoryItemStatus.Pending,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly if transaction expired', (): void => {
        const startDate = new Date(2019, 5, 6, 5, 5, 5, 5);
        const expirationDate = new Date(2019, 5, 6, 6, 5, 5, 5);

        const wrapper = mount(PaymentsHistoryItemDate, {
            localVue,
            propsData: {
                expiration: expirationDate,
                start: startDate,
                type: PaymentsHistoryItemType.Transaction,
                status: PaymentsHistoryItemStatus.Completed,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });
});
