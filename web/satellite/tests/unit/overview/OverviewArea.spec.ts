// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import OverviewArea from '@/components/overview/OverviewArea.vue';

import { shallowMount } from '@vue/test-utils';

describe('OverviewArea.vue', () => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(OverviewArea);

        expect(wrapper).toMatchSnapshot();
    });
});
