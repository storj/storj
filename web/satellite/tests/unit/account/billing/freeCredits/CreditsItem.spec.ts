// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import CreditsItem from '@/components/account/billing/freeCredits/CreditsItem.vue';

import { PaymentsHistoryItem, PaymentsHistoryItemType } from '@/types/payments';
import { createLocalVue, mount } from '@vue/test-utils';

const localVue = createLocalVue();
const coupon = new PaymentsHistoryItem('testId', 'desc', 275, 0, 'Active', '', new Date(1), new Date(1), PaymentsHistoryItemType.Coupon, 275);

localVue.filter('centsToDollars', (cents: number): string => {
    return `$${(cents / 100).toFixed(2)}`;
});

describe('CreditsItem', (): void => {
    it('renders correctly', (): void => {
        const wrapper = mount(CreditsItem, {
            localVue,
            propsData: {
                creditsItem: coupon,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });
});
