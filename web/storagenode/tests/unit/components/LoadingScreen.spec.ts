// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import LoadingScreen from '@/app/components/LoadingScreen.vue';

import { shallowMount } from '@vue/test-utils';

describe('LoadingScreen', (): void => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(LoadingScreen);

        expect(wrapper).toMatchSnapshot();
    });
});
