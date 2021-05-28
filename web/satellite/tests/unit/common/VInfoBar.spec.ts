// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';

import VInfoBar from '@/components/common/VInfoBar.vue';

import { router } from '@/router';
import { createLocalVue, mount, shallowMount } from '@vue/test-utils';

const localVue = createLocalVue();
localVue.use(Vuex);

describe('VInfoBar.vue', () => {
    it('renders correctly', () => {
        const wrapper = shallowMount(VInfoBar, {
            propsData: {
                firstValue: '5GB',
                secondValue: '5GB',
                firstDescription: 'test1',
                secondDescription: 'test2',
                link: 'testlink',
                linkLabel: 'label',
                path: '/',
            },
            router,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
