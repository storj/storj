// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { createLocalVue, shallowMount } from '@vue/test-utils';

import NoBucketsArea from '@/components/project/buckets/NoBucketsArea.vue';

const localVue = createLocalVue();

describe('NoBucketsArea.vue', () => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(NoBucketsArea, {
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
