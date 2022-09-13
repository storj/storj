// Copyright (C) 2019 Storj Labs, Inc.
// See LICENSE for copying information.

import { createLocalVue, shallowMount } from '@vue/test-utils';
import Vuex from 'vuex';

import { router } from '@/router';
import { appStateModule } from '@/store/modules/appState';

import GeneratePassphrase from '@/components/common/GeneratePassphrase.vue';

const localVue = createLocalVue();
localVue.use(Vuex);

const store = new Vuex.Store({ modules: {
    appStateModule,
} });

describe('GeneratePassphrase.vue', () => {
    it('renders correctly with default props', () => {
        const wrapper = shallowMount<GeneratePassphrase>(GeneratePassphrase, {
            localVue,
            router,
            store,
        });

        expect(wrapper.vm.passphrase).toBeTruthy();
        expect(wrapper).toMatchSnapshot();
    });
});
