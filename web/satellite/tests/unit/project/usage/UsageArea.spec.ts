// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { createLocalVue, shallowMount } from '@vue/test-utils';

import UsageArea from '@/components/project/usage/UsageArea.vue';

const localVue = createLocalVue();

describe('UsageArea.vue', () => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(UsageArea, {
            localVue,
            propsData: {
                title: 'test Title',
                used: 500000000,
                limit: 1000000000,
                isDataFetching: false,
            },
        });

        expect(wrapper).toMatchSnapshot();
    });

    it('renders correctly if used > limit', (): void => {
        const wrapper = shallowMount(UsageArea, {
            localVue,
            propsData: {
                title: 'test Title',
                used: 1000000000,
                limit: 500000000,
                isDataFetching: false,
            },
        });

        expect(wrapper).toMatchSnapshot();
        expect(wrapper.find('.usage-area__remaining').text()).toMatch('0 Bytes Remaining');
    });
});
