// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { createLocalVue, mount } from '@vue/test-utils';

import SortingHeader from '@/components/account/billing/depositAndBillingHistory/SortingHeader.vue';

const localVue = createLocalVue();

describe('SortingHeader', (): void => {
    it('renders correctly', (): void => {
        const wrapper = mount(SortingHeader, {
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
