// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import SortingHeader from '@/components/project/buckets/SortingHeader.vue';

import { createLocalVue, shallowMount } from '@vue/test-utils';

const localVue = createLocalVue();

describe('SortingHeader.vue', () => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(SortingHeader, {
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
