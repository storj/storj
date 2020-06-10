// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import CreditsItem from '@/components/account/billing/freeCredits/CreditsItem.vue';

import { PaymentsHistoryItem, PaymentsHistoryItemType } from '@/types/payments';
import { createLocalVue, mount } from '@vue/test-utils';

const localVue = createLocalVue();
const couponActive = new PaymentsHistoryItem('testId', 'desc', 275, 0, 'Active', '', new Date(1), new Date(1), PaymentsHistoryItemType.Coupon, 275);
const couponExpired = new PaymentsHistoryItem('testId', 'desc', 275, 0, 'Expired', '', new Date(1), new Date(1), PaymentsHistoryItemType.Coupon, 0);
const couponUsed = new PaymentsHistoryItem('testId', 'desc', 500, 0, 'Used', '', new Date(1), new Date(1), PaymentsHistoryItemType.Coupon, 0);

localVue.filter('centsToDollars', (cents: number): string => {
    return `$${(cents / 100).toFixed(2)}`;
});

describe('CreditsItem', (): void => {
    it('renders correctly if not expired', (): void => {
        const wrapper = mount(CreditsItem, {
            localVue,
            propsData: {
                creditsItem: couponActive,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly if expired', (): void => {
        const wrapper = mount(CreditsItem, {
            localVue,
            propsData: {
                creditsItem: couponExpired,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly if used', (): void => {
        const wrapper = mount(CreditsItem, {
            localVue,
            propsData: {
                creditsItem: couponUsed,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });
});
