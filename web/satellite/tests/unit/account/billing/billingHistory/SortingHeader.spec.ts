// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import SortingHeader from '@/components/account/billing/billingHistory/SortingHeader.vue';

import { createLocalVue, mount } from '@vue/test-utils';

const localVue = createLocalVue();

describe('SortingHeader', (): void => {
    it('renders correctly', (): void => {
        const wrapper = mount(SortingHeader, {
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
