// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import BillingHistoryItemDate from '@/components/account/billing/billingHistory/BillingHistoryItemDate.vue';

import { BillingHistoryItemType } from '@/types/payments';
import { createLocalVue, mount } from '@vue/test-utils';

const localVue = createLocalVue();
localVue.filter('leadingZero', function (value: number): string {
    if (value <= 9) {
        return `0${value}`;
    }

    return `${value}`;
});

describe('BillingHistoryItemDate', (): void => {
    it('renders correctly if invoice', (): void => {
        const startDate = new Date(2019, 1, 1, 1, 1, 1, 1);
        const expirationDate = new Date(0, 1, 1, 1, 1, 1, 1);

        const wrapper = mount(BillingHistoryItemDate, {
            localVue,
            propsData: {
                expiration: expirationDate,
                start: startDate,
                type: BillingHistoryItemType.Invoice,
            },
        });

        spyOn(Date.prototype, 'toLocaleString').and.returnValue(startDate.toLocaleString());

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly if charge', (): void => {
        const startDate = new Date(2019, 5, 5, 5, 5, 5, 5);
        const expirationDate = new Date(0, 1, 1, 1, 1, 1, 1);

        const wrapper = mount(BillingHistoryItemDate, {
            localVue,
            propsData: {
                expiration: expirationDate,
                start: startDate,
                type: BillingHistoryItemType.Charge,
            },
        });

        spyOn(Date.prototype, 'toLocaleString').and.returnValue(startDate.toLocaleString());

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly if transaction not expired', (): void => {
        const startDate = new Date(2019, 5, 5, 5, 5, 5, 5);
        const expirationDate = new Date(2019, 5, 5, 6, 5, 5, 5);
        const testTimeNow = expirationDate.getTime();

        spyOn(Date.prototype, 'getTime').and.returnValue(testTimeNow);

        const wrapper = mount(BillingHistoryItemDate, {
            localVue,
            propsData: {
                expiration: expirationDate,
                start: startDate,
                type: BillingHistoryItemType.Transaction,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly if transaction expired', (): void => {
        const startDate = new Date(2019, 5, 6, 5, 5, 5, 5);
        const expirationDate = new Date(2019, 5, 6, 6, 5, 5, 5);

        const wrapper = mount(BillingHistoryItemDate, {
            localVue,
            propsData: {
                expiration: expirationDate,
                start: startDate,
                type: BillingHistoryItemType.Transaction,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });
});
