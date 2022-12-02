// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import { shallowMount } from '@vue/test-utils';

import PayoutHistoryBlock from '@/app/components/payouts/PayoutHistoryBlock.vue';

describe('PayoutHistoryBlock', (): void => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(PayoutHistoryBlock);

        expect(wrapper).toMatchSnapshot();
    });
});
