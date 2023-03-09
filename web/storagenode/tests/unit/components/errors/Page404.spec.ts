// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import Vuex from 'vuex';
import { createLocalVue, shallowMount } from '@vue/test-utils';

import { appStateModule } from '@/app/store/modules/appState';

import Page404 from '@/app/components/errors/Page404.vue';

const localVue = createLocalVue();
localVue.use(Vuex);

const store = new Vuex.Store({ modules: { appStateModule } });

describe('Page404', (): void => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(Page404, {
            store,
            localVue,
        });

        expect(wrapper).toMatchSnapshot();
    });
});
