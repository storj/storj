// Copyright (C) 2020 Storj Labs, Inc.
// See LICENSE for copying information.

import { shallowMount } from '@vue/test-utils';

import LoadingScreen from '@/app/components/LoadingScreen.vue';

describe('LoadingScreen', (): void => {
    it('renders correctly', (): void => {
        const wrapper = shallowMount(LoadingScreen);

        expect(wrapper).toMatchSnapshot();
    });
});
