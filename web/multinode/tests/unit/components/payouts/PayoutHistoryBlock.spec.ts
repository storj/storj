// Copyright (C) 2021 Storj Labs, Inc.
// See LICENSE for copying information.

import PayoutHistoryBlock from '@/app/components/payouts/PayoutHistoryBlock.vue';

import { shallowMount } from '@vue/test-utils';

describe('PayoutHistoryBlock', (): void => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(PayoutHistoryBlock);

        expect(wrapper).toMatchSnapshot();
    });
});
